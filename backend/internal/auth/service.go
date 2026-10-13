package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"github.com/google/uuid"
	"regexp"
	"strings"
	"time"
)

// DefaultWorkspaceID is the single shared workspace, including imported legacy rows.
const DefaultWorkspaceID = "00000000-0000-4000-8000-000000000001"

var ErrRegistration = errors.New("registration failed")
var ErrCredentials = errors.New("invalid credentials")
var ErrUnauthenticated = errors.New("authentication required")
var ErrUnavailable = errors.New("authentication storage unavailable")
var ErrPassword = errors.New("invalid password")

type User struct {
	ID          string `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"displayName"`
	Role        string `json:"role"`
	WorkspaceID string `json:"workspaceId"`
}
type Account struct {
	User         User
	PasswordHash string
}
type Session struct {
	Digest    string
	User      User
	ExpiresAt time.Time
	CSRFToken string
}
type Repository interface {
	CreateAccount(context.Context, Account) error
	AccountByUsername(context.Context, string) (Account, error)
	CreateSession(context.Context, Session) error
	SessionByDigest(context.Context, string) (Session, error)
	RevokeSession(context.Context, string) error
}
type Service struct {
	repo   Repository
	invite string
	now    func() time.Time
}

func NewService(r Repository, invite string) *Service { return &Service{r, invite, time.Now} }

var usernamePattern = regexp.MustCompile(`^[a-z0-9_-]{3,32}$`)

func normalizeUsername(n string) (string, bool) {
	n = strings.ToLower(n)
	return n, usernamePattern.MatchString(n)
}
func (s *Service) Register(ctx context.Context, n, p, invite string) (User, error) {
	n, ok := normalizeUsername(n)
	if !ok || s.invite == "" || subtle.ConstantTimeCompare([]byte(invite), []byte(s.invite)) != 1 {
		return User{}, ErrRegistration
	}
	hash, e := HashPassword(ctx, p)
	if e != nil {
		return User{}, ErrRegistration
	}
	u := User{ID: uuid.NewString(), Username: n, DisplayName: n, Role: "annotator", WorkspaceID: DefaultWorkspaceID}
	if e = s.repo.CreateAccount(ctx, Account{u, hash}); e != nil {
		return User{}, ErrRegistration
	}
	return u, nil
}
func (s *Service) Login(ctx context.Context, n, p string) (string, Session, error) {
	n, valid := normalizeUsername(n)
	if !valid || !validPassword(p) {
		return "", Session{}, ErrCredentials
	}
	a, e := s.repo.AccountByUsername(ctx, n)
	if e != nil { // Equalize missing-account password work without creating a user.
		_, _ = HashPassword(ctx, p)
		return "", Session{}, ErrCredentials
	}
	if !VerifyPassword(ctx, a.PasswordHash, p) {
		return "", Session{}, ErrCredentials
	}
	token, e := randomToken()
	if e != nil {
		return "", Session{}, e
	}
	csrf, e := randomToken()
	if e != nil {
		return "", Session{}, e
	}
	session := Session{Digest: digest(token), User: a.User, ExpiresAt: s.now().Add(12 * time.Hour), CSRFToken: csrf}
	if e = s.repo.CreateSession(ctx, session); e != nil {
		return "", Session{}, e
	}
	return token, session, nil
}
func (s *Service) Authenticate(ctx context.Context, token string) (Session, error) {
	raw, e := base64.RawURLEncoding.DecodeString(token)
	if e != nil || len(raw) != 32 {
		return Session{}, ErrUnauthenticated
	}
	session, e := s.repo.SessionByDigest(ctx, digest(token))
	if e != nil {
		if errors.Is(e, sql.ErrNoRows) || errors.Is(e, ErrUnauthenticated) {
			return Session{}, ErrUnauthenticated
		}
		return Session{}, ErrUnavailable
	}
	if !s.now().Before(session.ExpiresAt) || session.User.WorkspaceID != DefaultWorkspaceID {
		return Session{}, ErrUnauthenticated
	}
	return session, nil
}
func (s *Service) Logout(ctx context.Context, token string) error {
	return s.repo.RevokeSession(ctx, digest(token))
}
func randomToken() (string, error) {
	b := make([]byte, 32)
	if _, e := rand.Read(b); e != nil {
		return "", e
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
func digest(token string) string { b := sha256.Sum256([]byte(token)); return hex.EncodeToString(b[:]) }
func newUserID() string          { return uuid.NewString() }
