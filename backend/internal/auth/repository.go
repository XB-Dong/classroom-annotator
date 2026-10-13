package auth

import (
	"context"
	"database/sql"
	"encoding/hex"
	"errors"
	"time"
)

type PGRepository struct{ db *sql.DB }

func NewPGRepository(db *sql.DB) *PGRepository { return &PGRepository{db} }
func (r *PGRepository) bounded(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, 10*time.Second)
}
func (r *PGRepository) CreateAccount(ctx context.Context, a Account) error {
	ctx, cancel := r.bounded(ctx)
	defer cancel()
	tx, e := r.db.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	if _, e = tx.ExecContext(ctx, `INSERT INTO users(id,username,display_name,password_hash) VALUES($1,$2,$3,$4)`, a.User.ID, a.User.Username, a.User.DisplayName, a.PasswordHash); e != nil {
		return e
	}
	if _, e = tx.ExecContext(ctx, `INSERT INTO workspace_members(workspace_id,user_id,role) VALUES($1,$2,$3)`, a.User.WorkspaceID, a.User.ID, a.User.Role); e != nil {
		return e
	}
	return tx.Commit()
}
func (r *PGRepository) AccountByUsername(ctx context.Context, n string) (a Account, e error) {
	ctx, cancel := r.bounded(ctx)
	defer cancel()
	e = r.db.QueryRowContext(ctx, `SELECT u.id,u.username,u.display_name,m.role,m.workspace_id,u.password_hash FROM users u JOIN workspace_members m ON m.user_id=u.id WHERE u.username=$1 AND m.workspace_id=$2`, n, DefaultWorkspaceID).Scan(&a.User.ID, &a.User.Username, &a.User.DisplayName, &a.User.Role, &a.User.WorkspaceID, &a.PasswordHash)
	return
}
func (r *PGRepository) CreateSession(ctx context.Context, s Session) error {
	ctx, cancel := r.bounded(ctx)
	defer cancel()
	d, e := hex.DecodeString(s.Digest)
	if e != nil {
		return e
	}
	_, e = r.db.ExecContext(ctx, `INSERT INTO sessions(token_digest,user_id,workspace_id,csrf_token,expires_at) VALUES($1,$2,$3,$4,$5)`, d, s.User.ID, s.User.WorkspaceID, s.CSRFToken, s.ExpiresAt)
	return e
}
func (r *PGRepository) SessionByDigest(ctx context.Context, d string) (s Session, e error) {
	ctx, cancel := r.bounded(ctx)
	defer cancel()
	raw, e := hex.DecodeString(d)
	if e != nil {
		return s, e
	}
	s.Digest = d
	e = r.db.QueryRowContext(ctx, `SELECT u.id,u.username,u.display_name,m.role,m.workspace_id,s.expires_at,s.csrf_token FROM sessions s JOIN users u ON u.id=s.user_id JOIN workspace_members m ON m.workspace_id=s.workspace_id AND m.user_id=s.user_id WHERE s.token_digest=$1 AND s.expires_at>now()`, raw).Scan(&s.User.ID, &s.User.Username, &s.User.DisplayName, &s.User.Role, &s.User.WorkspaceID, &s.ExpiresAt, &s.CSRFToken)
	return
}
func (r *PGRepository) RevokeSession(ctx context.Context, d string) error {
	ctx, cancel := r.bounded(ctx)
	defer cancel()
	raw, e := hex.DecodeString(d)
	if e != nil {
		return e
	}
	_, e = r.db.ExecContext(ctx, `DELETE FROM sessions WHERE token_digest=$1`, raw)
	return e
}

// BootstrapAdmin creates a named admin only through the explicit operator command.
// Existing accounts are never promoted or overwritten.
func (s *Service) BootstrapAdmin(ctx context.Context, n, p string) (User, error) {
	n, ok := normalizeUsername(n)
	if !ok {
		return User{}, ErrRegistration
	}
	hash, e := HashPassword(ctx, p)
	if e != nil {
		return User{}, e
	}
	u := User{ID: newUserID(), Username: n, DisplayName: n, Role: "admin", WorkspaceID: DefaultWorkspaceID}
	if e = s.repo.CreateAccount(ctx, Account{u, hash}); e != nil {
		return User{}, ErrRegistration
	}
	return u, nil
}

var ErrForbidden = errors.New("resource is outside workspace")

func (r *PGRepository) AuthorizeResource(ctx context.Context, u User, kind, id string) error {
	ctx, cancel := r.bounded(ctx)
	defer cancel()
	query := ""
	switch kind {
	case "video":
		query = `SELECT workspace_id FROM videos WHERE id=$1`
	case "frame":
		query = `SELECT v.workspace_id FROM frames f JOIN videos v ON v.id=f.video_id WHERE f.id=$1`
	case "batch":
		query = `SELECT v.workspace_id FROM batches b JOIN videos v ON v.id=b.video_id WHERE b.id=$1`
	case "job":
		query = `SELECT coalesce(v.workspace_id,'00000000-0000-4000-8000-000000000001') FROM jobs j LEFT JOIN videos v ON v.id=j.video_id WHERE j.id=$1`
	default:
		return ErrForbidden
	}
	var workspace string
	if e := r.db.QueryRowContext(ctx, query, id).Scan(&workspace); e != nil {
		if !errors.Is(e, sql.ErrNoRows) {
			return ErrUnavailable
		}
		return ErrForbidden
	}
	if workspace != u.WorkspaceID {
		return ErrForbidden
	}
	return nil
}
