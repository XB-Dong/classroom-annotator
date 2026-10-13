package auth

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"github.com/gin-gonic/gin"
	"io"
	"net/http"
	"net/url"
	"time"
)

type HTTPOptions struct {
	TrustedOrigins []string
	CookieSecure   bool
}
type HTTP struct {
	service *Service
	options HTTPOptions
	limiter *rateLimiter
}

func NewHTTP(s *Service, o HTTPOptions) (*HTTP, error) {
	if s == nil || s.repo == nil || s.invite == "" || len(o.TrustedOrigins) == 0 {
		return nil, errors.New("authentication configuration required")
	}
	for _, origin := range o.TrustedOrigins {
		u, e := url.Parse(origin)
		if e != nil || u.Host == "" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" || (u.Scheme != "https" && u.Scheme != "http") {
			return nil, errors.New("invalid trusted origin")
		}
		loopback := u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1" || u.Hostname() == "::1"
		if u.Scheme == "http" && !loopback {
			return nil, errors.New("public authentication requires HTTPS")
		}
		if !o.CookieSecure && (u.Scheme != "http" || (u.Hostname() != "localhost" && u.Hostname() != "127.0.0.1" && u.Hostname() != "::1")) {
			return nil, errors.New("insecure cookies require loopback HTTP")
		}
	}
	o.TrustedOrigins = append([]string(nil), o.TrustedOrigins...)
	return &HTTP{service: s, options: o, limiter: newRateLimiter()}, nil
}

const CookieName = "cvte_session"

type actorKey struct{}
type sessionKey struct{}

func ActorFrom(ctx context.Context) (User, bool) { u, ok := ctx.Value(actorKey{}).(User); return u, ok }
func authError(c *gin.Context, status int, code, message string) {
	c.AbortWithStatusJSON(status, gin.H{"code": code, "message": message, "requestId": c.GetString("requestId")})
}
func (h *HTTP) origin(c *gin.Context) bool {
	for _, o := range h.options.TrustedOrigins {
		if c.GetHeader("Origin") == o {
			return true
		}
	}
	authError(c, 403, "ORIGIN_REJECTED", "Request origin is not trusted")
	return false
}
func (h *HTTP) Register(g *gin.RouterGroup) {
	a := g.Group("/auth")
	a.POST("/register", h.register)
	a.POST("/login", h.login)
	a.GET("/me", h.Protect(), h.me)
	a.POST("/logout", h.Protect(), h.logout)
}
func (h *HTTP) Protect() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer = &privateWriter{ResponseWriter: c.Writer}
		c.Header("Cache-Control", "no-store")
		token, e := c.Cookie(CookieName)
		if e != nil {
			authError(c, 401, "AUTH_REQUIRED", "Authentication required")
			return
		}
		session, e := h.service.Authenticate(c.Request.Context(), token)
		if e != nil {
			if errors.Is(e, ErrUnavailable) {
				authError(c, 503, "AUTH_UNAVAILABLE", "Authentication temporarily unavailable")
				return
			}
			authError(c, 401, "AUTH_REQUIRED", "Authentication required")
			return
		}
		if c.Request.Method != "GET" && c.Request.Method != "HEAD" && c.Request.Method != "OPTIONS" {
			if !h.origin(c) {
				return
			}
			if subtle.ConstantTimeCompare([]byte(c.GetHeader("X-CSRF-Token")), []byte(session.CSRFToken)) != 1 {
				authError(c, 403, "CSRF_REJECTED", "CSRF token is invalid")
				return
			}
		}
		ctx := context.WithValue(c.Request.Context(), actorKey{}, session.User)
		ctx = context.WithValue(ctx, sessionKey{}, session)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
		c.Header("Cache-Control", "no-store")
	}
}
func decode(c *gin.Context, v any) bool {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 4096)
	d := json.NewDecoder(c.Request.Body)
	d.DisallowUnknownFields()
	if e := d.Decode(v); e != nil {
		authError(c, 400, "INVALID_REQUEST", "Invalid JSON request")
		return false
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		authError(c, 400, "INVALID_REQUEST", "Invalid JSON request")
		return false
	}
	return true
}
func (h *HTTP) register(c *gin.Context) {
	if !h.origin(c) {
		return
	}
	var body struct {
		Username   string `json:"username"`
		Password   string `json:"password"`
		InviteCode string `json:"inviteCode"`
	}
	if !decode(c, &body) {
		return
	}
	if !h.limit(c, body.Username, "register") {
		return
	}
	u, e := h.service.Register(c.Request.Context(), body.Username, body.Password, body.InviteCode)
	if e != nil {
		authError(c, 400, "REGISTRATION_FAILED", "Registration failed")
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(201, gin.H{"user": u})
}
func (h *HTTP) login(c *gin.Context) {
	if !h.origin(c) {
		return
	}
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if !decode(c, &body) {
		return
	}
	if !h.limit(c, body.Username, "login") {
		return
	}
	token, s, e := h.service.Login(c.Request.Context(), body.Username, body.Password)
	if e != nil {
		authError(c, 401, "INVALID_CREDENTIALS", "Invalid username or password")
		return
	}
	h.cookie(c, token, 43200, s.ExpiresAt)
	c.Header("Cache-Control", "no-store")
	c.JSON(200, gin.H{"user": s.User, "csrfToken": s.CSRFToken})
}
func (h *HTTP) cookie(c *gin.Context, token string, age int, expires time.Time) {
	http.SetCookie(c.Writer, &http.Cookie{Name: CookieName, Value: token, Path: "/", HttpOnly: true, Secure: h.options.CookieSecure, SameSite: http.SameSiteLaxMode, MaxAge: age, Expires: expires})
}
func (h *HTTP) me(c *gin.Context) {
	s := c.Request.Context().Value(sessionKey{}).(Session)
	c.JSON(200, gin.H{"user": s.User, "csrfToken": s.CSRFToken})
}
func (h *HTTP) logout(c *gin.Context) {
	token, _ := c.Cookie(CookieName)
	if e := h.service.Logout(c.Request.Context(), token); e != nil {
		authError(c, 500, "AUTH_UNAVAILABLE", "Authentication unavailable")
		return
	}
	h.cookie(c, "", -1, time.Unix(1, 0))
	c.Status(204)
}
