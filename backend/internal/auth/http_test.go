package auth

import (
	"context"
	"encoding/json"
	"github.com/gin-gonic/gin"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func authRouter(t *testing.T) (*gin.Engine, *Service) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	s := NewService(memory(), "invite")
	h, e := NewHTTP(s, HTTPOptions{TrustedOrigins: []string{"https://app.test"}, CookieSecure: true})
	if e != nil {
		t.Fatal(e)
	}
	r := gin.New()
	h.Register(r.Group("/api/v1"))
	g := r.Group("/private", h.Protect())
	media := func(c *gin.Context) {
		u, ok := ActorFrom(c.Request.Context())
		if !ok {
			t.Fatal("actor missing")
		}
		c.JSON(200, u)
	}
	g.GET("/media", media)
	g.HEAD("/media", media)
	g.PUT("/media", func(c *gin.Context) { c.Status(204) })
	return r, s
}
func request(r http.Handler, method, path, body, origin, csrf string, cookie *http.Cookie) *httptest.ResponseRecorder {
	q := httptest.NewRequest(method, path, strings.NewReader(body))
	q.Header.Set("Content-Type", "application/json")
	if origin != "" {
		q.Header.Set("Origin", origin)
	}
	if csrf != "" {
		q.Header.Set("X-CSRF-Token", csrf)
	}
	if cookie != nil {
		q.AddCookie(cookie)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, q)
	return w
}
func TestHTTPOriginRoleSpoofCookieAndCSRF(t *testing.T) {
	r, s := authRouter(t)
	ctx := context.Background()
	_, e := s.Register(ctx, "alice", "long-password-123", "invite")
	if e != nil {
		t.Fatal(e)
	}
	for _, origin := range []string{"", "https://evil.test"} {
		w := request(r, "POST", "/api/v1/auth/login", `{"username":"alice","password":"long-password-123"}`, origin, "", nil)
		if w.Code != 403 {
			t.Fatalf("origin accepted: %d", w.Code)
		}
	}
	w := request(r, "POST", "/api/v1/auth/register", `{"username":"bob","password":"long-password-123","inviteCode":"invite","role":"admin"}`, "https://app.test", "", nil)
	if w.Code != 400 {
		t.Fatalf("role spoof accepted: %d", w.Code)
	}
	w = request(r, "POST", "/api/v1/auth/login", `{"username":"alice","password":"long-password-123"}`, "https://app.test", "", nil)
	if w.Code != 200 {
		t.Fatalf("login %d: %s", w.Code, w.Body.String())
	}
	cookies := w.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatal("session cookie missing")
	}
	cookie := cookies[0]
	if !cookie.HttpOnly || !cookie.Secure || cookie.SameSite != http.SameSiteLaxMode || cookie.Path != "/" || cookie.MaxAge != 43200 {
		t.Fatalf("unsafe cookie: %+v", cookie)
	}
	var snapshot struct {
		User User   `json:"user"`
		CSRF string `json:"csrfToken"`
	}
	if e = json.Unmarshal(w.Body.Bytes(), &snapshot); e != nil || snapshot.CSRF == "" {
		t.Fatal("csrf missing")
	}
	if w = request(r, "PUT", "/private/media", "", "https://app.test", "wrong", cookie); w.Code != 403 {
		t.Fatal("bad csrf accepted")
	}
	if w = request(r, "PUT", "/private/media", "", "https://evil.test", snapshot.CSRF, cookie); w.Code != 403 {
		t.Fatal("bad origin accepted")
	}
	if w = request(r, "PUT", "/private/media", "", "https://app.test", snapshot.CSRF, cookie); w.Code != 204 {
		t.Fatalf("valid csrf denied: %d", w.Code)
	}
	if w = request(r, "GET", "/api/v1/auth/me", "", "", "", cookie); w.Code != 200 {
		t.Fatal("me failed")
	}
	if w = request(r, "POST", "/api/v1/auth/logout", "", "https://app.test", snapshot.CSRF, cookie); w.Code != 204 {
		t.Fatal("logout failed")
	}
	for _, method := range []string{"GET", "HEAD"} {
		if w = request(r, method, "/private/media", "", "", "", cookie); w.Code != 401 {
			t.Fatalf("revoked media accepted: %d", w.Code)
		}
	}
}
func TestAnonymousCannotReadMedia(t *testing.T) {
	r, _ := authRouter(t)
	for _, method := range []string{"GET", "HEAD"} {
		w := request(r, method, "/private/media", "", "", "", nil)
		if w.Code != 401 {
			t.Fatalf("anonymous %s accepted: %d", method, w.Code)
		}
	}
}
func TestAuthRateLimitIgnoresForwardedIP(t *testing.T) {
	r, _ := authRouter(t)
	for i := 0; i < 11; i++ {
		q := httptest.NewRequest("POST", "/api/v1/auth/register", strings.NewReader(`{"username":"alice","password":"long-password-123","inviteCode":"wrong"}`))
		q.Header.Set("Origin", "https://app.test")
		q.Header.Set("X-Forwarded-For", string(rune('a'+i)))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, q)
		if i == 10 && w.Code != 429 {
			t.Fatalf("register IP limit bypassed: %d", w.Code)
		}
	}
}
func TestProtectedMediaCannotOverrideNoStore(t *testing.T) {
	r, s := authRouter(t)
	_, e := s.Register(context.Background(), "alice", "long-password-123", "invite")
	if e != nil {
		t.Fatal(e)
	}
	token, _, e := s.Login(context.Background(), "alice", "long-password-123")
	if e != nil {
		t.Fatal(e)
	}
	h, _ := NewHTTP(s, HTTPOptions{TrustedOrigins: []string{"https://app.test"}, CookieSecure: true})
	r.GET("/cache", h.Protect(), func(c *gin.Context) { c.Header("Cache-Control", "public, max-age=31536000"); c.String(200, "media") })
	w := request(r, "GET", "/cache", "", "", "", &http.Cookie{Name: CookieName, Value: token})
	if w.Result().Header.Get("Cache-Control") != "no-store" {
		t.Fatalf("private bytes cached: %s", w.Result().Header.Get("Cache-Control"))
	}
}
func TestPublicHTTPOriginCannotEnableCredentialEndpoints(t *testing.T) {
	s := NewService(memory(), "invite")
	if _, e := NewHTTP(s, HTTPOptions{TrustedOrigins: []string{"http://public.test"}, CookieSecure: true}); e == nil {
		t.Fatal("public plaintext authentication enabled")
	}
	if _, e := NewHTTP(s, HTTPOptions{TrustedOrigins: []string{"http://127.0.0.1:8080"}, CookieSecure: false}); e != nil {
		t.Fatal("explicit loopback development rejected", e)
	}
}
