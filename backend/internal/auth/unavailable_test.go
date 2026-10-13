package auth

import (
	"context"
	"errors"
	"github.com/gin-gonic/gin"
	"net/http"
	"strings"
	"testing"
)

type unavailableSessions struct{ Repository }

func (unavailableSessions) SessionByDigest(context.Context, string) (Session, error) {
	return Session{}, errors.New("private database connection detail")
}
func TestStorageOutageDoesNotPretendSessionExpired(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h, e := NewHTTP(NewService(unavailableSessions{memory()}, "invite"), HTTPOptions{TrustedOrigins: []string{"https://app.test"}, CookieSecure: true})
	if e != nil {
		t.Fatal(e)
	}
	r := gin.New()
	h.Register(r.Group("/api/v1"))
	response := request(r, "GET", "/api/v1/auth/me", "", "", "", &http.Cookie{Name: CookieName, Value: strings.Repeat("A", 43)})
	if response.Code != 503 {
		t.Fatalf("storage outage status=%d body=%s", response.Code, response.Body.String())
	}
	if strings.Contains(response.Body.String(), "private database") {
		t.Fatal("private database detail leaked")
	}
}
