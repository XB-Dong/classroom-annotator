package httpapi

import (
	"context"
	"errors"
	"github.com/XB-Dong/classroom-annotator/backend/internal/media"
	"github.com/gin-gonic/gin"
	"net"
	"net/http"
	"time"
)

type uploadDeadline struct {
	ctx        context.Context
	cancel     context.CancelFunc
	controller *http.ResponseController
	stopClose  func() bool
	closed     chan struct{}
	timedOut   bool
	c          *gin.Context
}

func startUploadDeadline(c *gin.Context, timeout time.Duration) *uploadDeadline {
	ctx, cancel := context.WithTimeout(c.Request.Context(), timeout)
	g := &uploadDeadline{ctx: ctx, cancel: cancel, controller: http.NewResponseController(c.Writer), closed: make(chan struct{}), c: c}
	deadline, _ := ctx.Deadline()
	_ = g.controller.SetReadDeadline(deadline)
	// Setting the socket deadline interrupts an in-flight body Read before Close waits for it.
	body := c.Request.Body
	g.stopClose = context.AfterFunc(ctx, func() { defer close(g.closed); _ = g.controller.SetReadDeadline(time.Now()); _ = body.Close() })
	return g
}
func (g *uploadDeadline) close() {
	if !g.stopClose() {
		<-g.closed
	}
	expired := g.ctx.Err() != nil
	g.cancel()
	if !expired && !g.timedOut {
		_ = g.controller.SetReadDeadline(time.Time{})
	}
}
func (g *uploadDeadline) report(err error) bool {
	if errors.Is(err, media.ErrCleanup) {
		return false
	}
	var netErr net.Error
	deadline, _ := g.ctx.Deadline()
	// A socket timeout can cancel the parent request before the context timer reports DeadlineExceeded.
	if !time.Now().Before(deadline) || g.ctx.Err() == context.DeadlineExceeded || (errors.As(err, &netErr) && netErr.Timeout()) {
		g.timedOut = true
		g.c.Header("Connection", "close")
		Error(g.c, 503, "UPLOAD_TIMEOUT", "Upload timed out")
		return true
	}
	return false
}
