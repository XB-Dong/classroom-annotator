package auth

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

type memoryRepository struct {
	users    map[string]Account
	sessions map[string]Session
}

func memory() *memoryRepository { return &memoryRepository{map[string]Account{}, map[string]Session{}} }
func (r *memoryRepository) CreateAccount(_ context.Context, a Account) error {
	if _, ok := r.users[a.User.Username]; ok {
		return ErrRegistration
	}
	r.users[a.User.Username] = a
	return nil
}
func (r *memoryRepository) AccountByUsername(_ context.Context, n string) (Account, error) {
	a, ok := r.users[n]
	if !ok {
		return Account{}, ErrCredentials
	}
	return a, nil
}
func (r *memoryRepository) CreateSession(_ context.Context, s Session) error {
	r.sessions[s.Digest] = s
	return nil
}
func (r *memoryRepository) SessionByDigest(_ context.Context, d string) (Session, error) {
	s, ok := r.sessions[d]
	if !ok {
		return Session{}, ErrUnauthenticated
	}
	return s, nil
}
func (r *memoryRepository) RevokeSession(_ context.Context, d string) error {
	delete(r.sessions, d)
	return nil
}

func TestRegistrationRequiresInviteAndNeverPromotesFirstUser(t *testing.T) {
	r := memory()
	s := NewService(r, "invite")
	ctx := context.Background()
	if _, e := s.Register(ctx, "Alice", "long-password-123", "wrong"); !errors.Is(e, ErrRegistration) {
		t.Fatalf("bad invite accepted: %v", e)
	}
	u, e := s.Register(ctx, "Alice", "long-password-123", "invite")
	if e != nil {
		t.Fatal(e)
	}
	if u.Username != "alice" || u.Role != "annotator" || u.WorkspaceID != DefaultWorkspaceID {
		t.Fatalf("unexpected identity: %+v", u)
	}
	if len(r.sessions) != 0 {
		t.Fatal("registration logged user in")
	}
	if _, e = s.Register(ctx, "ALICE", "long-password-123", "invite"); !errors.Is(e, ErrRegistration) {
		t.Fatalf("duplicate accepted: %v", e)
	}
}
func TestPasswordsValidateUTF8BytesAndUniqueSalt(t *testing.T) {
	for _, p := range []string{"short", strings.Repeat("界", 43)} {
		if _, e := HashPassword(context.Background(), p); !errors.Is(e, ErrPassword) {
			t.Fatalf("invalid bytes accepted: %v", e)
		}
	}
	a, e := HashPassword(context.Background(), "long-password-123")
	if e != nil {
		t.Fatal(e)
	}
	b, e := HashPassword(context.Background(), "long-password-123")
	if e != nil {
		t.Fatal(e)
	}
	if a == b {
		t.Fatal("salt reused")
	}
	if !strings.HasPrefix(a, "$argon2id$v=19$m=19456,t=2,p=1$") {
		t.Fatal("wrong Argon2 parameters")
	}
	if !VerifyPassword(context.Background(), a, "long-password-123") || VerifyPassword(context.Background(), a, "wrong-password-123") {
		t.Fatal("password verification incorrect")
	}
}
func TestSessionsAreHashedExpireAbsolutelyAndRevoke(t *testing.T) {
	r := memory()
	s := NewService(r, "invite")
	now := time.Date(2026, 10, 14, 0, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return now }
	ctx := context.Background()
	if _, e := s.Register(ctx, "alice", "long-password-123", "invite"); e != nil {
		t.Fatal(e)
	}
	if _, _, e := s.Login(ctx, "alice", "wrong-password-123"); !errors.Is(e, ErrCredentials) {
		t.Fatalf("bad credentials accepted: %v", e)
	}
	token, session, e := s.Login(ctx, "ALICE", "long-password-123")
	if e != nil {
		t.Fatal(e)
	}
	if token == session.Digest || len(token) != 43 || session.ExpiresAt.Sub(now) != 12*time.Hour {
		t.Fatal("unsafe session")
	}
	if _, ok := r.sessions[token]; ok {
		t.Fatal("raw token persisted")
	}
	if _, e = s.Authenticate(ctx, token); e != nil {
		t.Fatal(e)
	}
	now = now.Add(12 * time.Hour)
	if _, e = s.Authenticate(ctx, token); !errors.Is(e, ErrUnauthenticated) {
		t.Fatalf("expired session accepted: %v", e)
	}
	now = now.Add(-time.Hour)
	if e = s.Logout(ctx, token); e != nil {
		t.Fatal(e)
	}
	if _, e = s.Authenticate(ctx, token); !errors.Is(e, ErrUnauthenticated) {
		t.Fatal("logged out session accepted")
	}
}
func TestAdminRequiresExplicitBootstrapAndCannotPromoteExistingAccount(t *testing.T) {
	r := memory()
	s := NewService(r, "invite")
	ctx := context.Background()
	u, e := s.BootstrapAdmin(ctx, "operator", "long-password-123")
	if e != nil || u.Role != "admin" {
		t.Fatalf("bootstrap failed: %+v %v", u, e)
	}
	if _, e = s.Register(ctx, "alice", "long-password-123", "invite"); e != nil {
		t.Fatal(e)
	}
	if _, e = s.BootstrapAdmin(ctx, "alice", "long-password-123"); e == nil {
		t.Fatal("existing public account promoted")
	}
	if r.users["alice"].User.Role != "annotator" {
		t.Fatal("existing role changed")
	}
}
