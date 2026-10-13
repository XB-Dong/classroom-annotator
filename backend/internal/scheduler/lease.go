// Package scheduler owns the single task scheduler for one PostgreSQL database.
package scheduler

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"sync"
	"sync/atomic"
	"time"
)

// The two-key advisory namespace is distinct from hashed one-key source locks.
// These domain constants identify the classroom task scheduler, independent of deployment.
const namespace int32 = 0x43565445
const schedulerKey int32 = 1
const queryTimeout = time.Second

var ErrNotOwner = errors.New("TASK_OWNER_UNAVAILABLE")

type Lease struct {
	conn  *sql.Conn
	owned atomic.Bool
	mu    sync.Mutex
}

// Acquire reserves one pool connection only when it wins the session advisory lock.
// A non-owner serves ordinary HTTP requests without retaining a connection.
func Acquire(ctx context.Context, db *sql.DB) (*Lease, error) {
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	conn, err := db.Conn(ctx)
	if err != nil {
		return nil, err
	}
	var owned bool
	if err = conn.QueryRowContext(ctx, `SELECT pg_try_advisory_lock($1,$2)`, namespace, schedulerKey).Scan(&owned); err != nil {
		// A timed-out result can be ambiguous: never return a possibly locked session.
		_ = conn.Raw(func(any) error { return driver.ErrBadConn })
		conn.Close()
		return nil, err
	}
	lease := &Lease{}
	if !owned {
		conn.Close()
		return lease, nil
	}
	lease.conn = conn
	lease.owned.Store(true)
	return lease, nil
}

func (l *Lease) Owned() bool { return l != nil && l.owned.Load() }

// Watch permanently downgrades this process when its held session is lost.
// It never reacquires; a new operator startup must acquire a new lease.
func (l *Lease) Watch(ctx context.Context, onLoss func()) {
	if !l.Owned() {
		return
	}
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			l.mu.Lock()
			if !l.Owned() {
				l.mu.Unlock()
				return
			}
			pingCtx, cancel := context.WithTimeout(ctx, queryTimeout)
			err := l.conn.PingContext(pingCtx)
			cancel()
			if err != nil {
				l.owned.Store(false)
			}
			l.mu.Unlock()
			if err != nil {
				if onLoss != nil {
					onLoss()
				}
				return
			}
		}
	}
}

// Recover uses the held session, so losing ownership cannot silently reconnect.
// AI recovery has its own cancellation semantics and must run separately first.
func (l *Lease) Recover(ctx context.Context) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if !l.Owned() {
		return ErrNotOwner
	}
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	_, err := l.conn.ExecContext(ctx, `UPDATE jobs SET status='failed',error='INTERRUPTED' WHERE kind <> 'preannotation' AND status IN ('queued','running','cancelling')`)
	return err
}

// Conn supplies the held session to startup-only local media reconciliation.
// Callers must use the cancellable ownership context and never close it.
func (l *Lease) Conn() *sql.Conn {
	l.mu.Lock()
	defer l.mu.Unlock()
	if !l.Owned() {
		return nil
	}
	return l.conn
}

func (l *Lease) Close() error {
	if l == nil {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.conn == nil {
		return nil
	}
	owned := l.owned.Swap(false)
	var err error
	if owned {
		ctx, cancel := context.WithTimeout(context.Background(), queryTimeout)
		_, err = l.conn.ExecContext(ctx, `SELECT pg_advisory_unlock($1,$2)`, namespace, schedulerKey)
		cancel()
		if err != nil {
			_ = l.conn.Raw(func(any) error { return driver.ErrBadConn })
		}
	}
	closeErr := l.conn.Close()
	l.conn = nil
	return errors.Join(err, closeErr)
}
