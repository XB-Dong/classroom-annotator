package auth

import (
	"github.com/gin-gonic/gin"
	"net"
	"strings"
	"sync"
	"time"
)

type rateBucket struct {
	count   int
	expires time.Time
}
type rateLimiter struct {
	mu      sync.Mutex
	buckets map[string]rateBucket
}

func newRateLimiter() *rateLimiter { return &rateLimiter{buckets: make(map[string]rateBucket)} }
func (r *rateLimiter) allow(keys ...string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now()
	for k, b := range r.buckets {
		if !now.Before(b.expires) {
			delete(r.buckets, k)
		}
	}
	if len(r.buckets)+len(keys) > 4096 {
		return false
	}
	for _, key := range keys {
		b := r.buckets[key]
		if b.count >= 10 && now.Before(b.expires) {
			return false
		}
	}
	for _, key := range keys {
		b := r.buckets[key]
		if !now.Before(b.expires) {
			b = rateBucket{expires: now.Add(time.Minute)}
		}
		b.count++
		r.buckets[key] = b
	}
	return true
}
func (h *HTTP) limit(c *gin.Context, username, operation string) bool {
	ip, _, e := net.SplitHostPort(c.Request.RemoteAddr)
	if e != nil {
		ip = c.Request.RemoteAddr
	}
	username = strings.ToLower(username)
	if len(username) > 32 {
		username = "invalid"
	}
	if !h.limiter.allow(operation+":ip:"+ip, operation+":user:"+username) {
		c.Header("Retry-After", "60")
		authError(c, 429, "AUTH_RATE_LIMITED", "Try again later")
		return false
	}
	return true
}

// Enforce no-store at the moment headers reach the wire, including File/ServeContent.
type privateWriter struct{ gin.ResponseWriter }

func (w *privateWriter) WriteHeader(status int) {
	w.Header().Set("Cache-Control", "no-store")
	w.ResponseWriter.WriteHeader(status)
}
func (w *privateWriter) WriteHeaderNow() {
	w.Header().Set("Cache-Control", "no-store")
	w.ResponseWriter.WriteHeaderNow()
}
func (w *privateWriter) Write(b []byte) (int, error) {
	w.Header().Set("Cache-Control", "no-store")
	return w.ResponseWriter.Write(b)
}
func (w *privateWriter) WriteString(s string) (int, error) {
	w.Header().Set("Cache-Control", "no-store")
	return w.ResponseWriter.WriteString(s)
}
