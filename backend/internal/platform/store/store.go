package store

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path"
	"path/filepath"
	"time"

	"github.com/XB-Dong/classroom-annotator/backend/internal/domain"
	"github.com/XB-Dong/classroom-annotator/backend/migrations"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
)

var ErrState = errors.New("job state changed")
var ErrSchema = errors.New("database schema version mismatch")

type Options struct {
	DatabaseURL                                                    string
	MaxOpenConns, MaxIdleConns                                     int
	ConnectTimeout, QueryTimeout, ConnMaxLifetime, ConnMaxIdleTime time.Duration
}
type Store struct {
	DB      *sql.DB
	Root    string
	Options Options
}

// Open verifies an explicitly migrated PostgreSQL database. It never executes DDL.
func Open(root string, options ...Options) (*Store, error) {
	if len(options) != 1 || options[0].DatabaseURL == "" {
		return nil, errors.New("PostgreSQL configuration required")
	}
	o := options[0]
	if o.MaxOpenConns == 0 {
		o.MaxOpenConns = 5
	}
	if o.MaxIdleConns == 0 {
		o.MaxIdleConns = 2
	}
	if o.ConnectTimeout == 0 {
		o.ConnectTimeout = 5 * time.Second
	}
	if o.QueryTimeout == 0 {
		o.QueryTimeout = 10 * time.Second
	}
	if o.ConnMaxLifetime == 0 {
		o.ConnMaxLifetime = 30 * time.Minute
	}
	if o.ConnMaxIdleTime == 0 {
		o.ConnMaxIdleTime = 5 * time.Minute
	}
	if o.MaxOpenConns < 1 || o.MaxOpenConns > 5 || o.MaxIdleConns < 0 || o.MaxIdleConns > o.MaxOpenConns || o.ConnectTimeout <= 0 || o.QueryTimeout <= 0 {
		return nil, errors.New("invalid PostgreSQL options")
	}
	cfg, err := pgx.ParseConfig(o.DatabaseURL)
	if err != nil {
		return nil, errors.New("invalid PostgreSQL configuration")
	}
	cfg.ConnectTimeout = o.ConnectTimeout
	db := stdlib.OpenDB(*cfg)
	db.SetMaxOpenConns(o.MaxOpenConns)
	db.SetMaxIdleConns(o.MaxIdleConns)
	db.SetConnMaxLifetime(o.ConnMaxLifetime)
	db.SetConnMaxIdleTime(o.ConnMaxIdleTime)
	s := &Store{DB: db, Root: root, Options: o}
	ctx, cancel := context.WithTimeout(context.Background(), o.ConnectTimeout)
	defer cancel()
	if err = s.Ready(ctx); err != nil {
		db.Close()
		return nil, err
	}
	for _, dir := range []string{"", "staging", "videos", "frames"} {
		if err = os.MkdirAll(filepath.Join(root, dir), 0700); err != nil {
			db.Close()
			return nil, err
		}
	}
	return s, nil
}
func (s *Store) Context(parent context.Context) (context.Context, context.CancelFunc) {
	timeout := s.Options.QueryTimeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return context.WithTimeout(parent, timeout)
}
func (s *Store) Ready(ctx context.Context) error {
	if err := s.DB.PingContext(ctx); err != nil {
		return errors.New("database unavailable")
	}
	var version int64
	if err := s.DB.QueryRowContext(ctx, `SELECT version_id FROM goose_db_version WHERE is_applied ORDER BY id DESC LIMIT 1`).Scan(&version); err != nil || version != migrations.Version {
		return ErrSchema
	}
	return nil
}

const videoColumns = `id,name,width,height,duration_seconds,status,created_at,storage_key,source_kind,coalesce(source_id,''),storage_provider,media_sha256,media_size_bytes`
const insertVideo = `INSERT INTO videos(id,name,width,height,duration_seconds,status,created_at,storage_key,source_kind,source_id,storage_provider,media_sha256,media_size_bytes) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,nullif($10,''),$11,$12,$13)`

func videoArgs(v domain.Video) []any {
	if v.SourceKind == "" {
		v.SourceKind = "legacy"
	}
	if v.StorageProvider == "" {
		v.StorageProvider = "local"
	}
	return []any{v.ID, v.Name, v.Width, v.Height, v.DurationSeconds, v.Status, v.CreatedAt, v.StorageKey, v.SourceKind, v.SourceID, v.StorageProvider, v.MediaSHA256, v.MediaSizeBytes}
}
func scanVideo(r interface{ Scan(...any) error }) (v domain.Video, err error) {
	err = r.Scan(&v.ID, &v.Name, &v.Width, &v.Height, &v.DurationSeconds, &v.Status, &v.CreatedAt, &v.StorageKey, &v.SourceKind, &v.SourceID, &v.StorageProvider, &v.MediaSHA256, &v.MediaSizeBytes)
	return
}
func (s *Store) AddVideo(v domain.Video) error {
	return s.AddVideoWithContext(context.Background(), v)
}
func (s *Store) AddVideoWithContext(parent context.Context, v domain.Video) error {
	ctx, cancel := s.Context(parent)
	defer cancel()
	_, err := s.DB.ExecContext(ctx, insertVideo, videoArgs(v)...)
	return err
}
func (s *Store) Video(id string) (domain.Video, error) {
	ctx, cancel := s.Context(context.Background())
	defer cancel()
	return scanVideo(s.DB.QueryRowContext(ctx, `SELECT `+videoColumns+` FROM videos WHERE id=$1`, id))
}
func (s *Store) Videos() ([]domain.Video, error) {
	return s.videosWhere("", nil)
}
func (s *Store) VideosInWorkspace(workspace string) ([]domain.Video, error) {
	return s.videosWhere(" WHERE workspace_id=$1", []any{workspace})
}
func (s *Store) videosWhere(where string, args []any) ([]domain.Video, error) {
	ctx, cancel := s.Context(context.Background())
	defer cancel()
	rows, err := s.DB.QueryContext(ctx, `SELECT `+videoColumns+` FROM videos`+where+` ORDER BY created_at DESC,id`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.Video{}
	for rows.Next() {
		v, err := scanVideo(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// Recover updates interrupted jobs independently of physical media cleanup.
func (s *Store) Recover() error {
	ctx, cancel := s.Context(context.Background())
	defer cancel()
	_, err := s.DB.ExecContext(ctx, `UPDATE jobs SET status='failed',error='INTERRUPTED' WHERE status IN ('queued','running','cancelling')`)
	return err
}

// RecoverLocalMedia reconciles local files only. A ready sample cache may precede its video record.
func (s *Store) RecoverLocalMedia() error {
	return s.recoverLocalMedia(context.Background(), nil)
}

// RecoverLocalMediaWithContext requires the held scheduler session in production.
func (s *Store) RecoverLocalMediaWithContext(parent context.Context, conn *sql.Conn) error {
	if err := parent.Err(); err != nil {
		return err
	}
	if conn == nil {
		return errors.New("scheduler connection required")
	}
	return s.recoverLocalMedia(parent, conn)
}

func (s *Store) recoverLocalMedia(parent context.Context, conn *sql.Conn) error {
	ctx, cancel := s.Context(parent)
	defer cancel()
	var query interface {
		QueryRowContext(context.Context, string, ...any) *sql.Row
	} = s.DB
	if conn != nil {
		query = conn
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	for _, kind := range []string{"staging", "frames", "videos"} {
		entries, err := os.ReadDir(filepath.Join(s.Root, kind))
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if err := ctx.Err(); err != nil {
				return err
			}
			n := 0
			if kind == "frames" {
				err = query.QueryRowContext(ctx, `SELECT count(*) FROM frames WHERE batch_id=$1 AND storage_provider='local'`, entry.Name()).Scan(&n)
			}
			if kind == "videos" {
				legacyKey := filepath.Join("videos", entry.Name())
				canonicalKey := path.Join("videos", entry.Name())
				err = query.QueryRowContext(ctx, `SELECT (SELECT count(*) FROM videos WHERE storage_provider='local' AND (storage_key=$1 OR storage_key=$2 OR starts_with(storage_key,$3) OR starts_with(storage_key,$4))) + (SELECT count(*) FROM public_samples WHERE cache_status='ready' AND (storage_key=$1 OR storage_key=$2 OR starts_with(storage_key,$3) OR starts_with(storage_key,$4)))`, legacyKey, canonicalKey, legacyKey+`\`, canonicalKey+"/").Scan(&n)
			}
			if err != nil {
				return err
			}
			if n == 0 {
				if err := ctx.Err(); err != nil {
					return err
				}
				// Staging has no metadata query; verify the held session before every removal.
				if conn != nil {
					if err := conn.PingContext(ctx); err != nil {
						return err
					}
					if err := ctx.Err(); err != nil {
						return err
					}
				}
				if err := os.RemoveAll(filepath.Join(s.Root, kind, entry.Name())); err != nil {
					return err
				}
			}
		}
	}
	return nil
}
