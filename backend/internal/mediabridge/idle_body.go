package mediabridge

import (
	"context"
	"io"
	"sync"
	"time"
)

// idleBody cancels a media request only when its body stops making progress.
type idleBody struct {
	io.ReadCloser
	cancel context.CancelFunc
	timer  *time.Timer
	idle   time.Duration
	mu     sync.Mutex
	closed bool
}

func newIdleBody(body io.ReadCloser, cancel context.CancelFunc, idle time.Duration) io.ReadCloser {
	b := &idleBody{ReadCloser: body, cancel: cancel, idle: idle}
	b.timer = time.AfterFunc(idle, cancel)
	return b
}

func (b *idleBody) Read(p []byte) (int, error) {
	n, err := b.ReadCloser.Read(p)
	if n > 0 {
		b.mu.Lock()
		if !b.closed {
			b.timer.Reset(b.idle)
		}
		b.mu.Unlock()
	}
	return n, err
}

func (b *idleBody) Close() error {
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return nil
	}
	b.closed = true
	b.timer.Stop()
	b.mu.Unlock()
	b.cancel()
	return b.ReadCloser.Close()
}
