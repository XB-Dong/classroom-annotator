package store

import (
	"database/sql"
	_ "embed"
	"errors"
	"github.com/XB-Dong/classroom-annotator/backend/internal/domain"
	_ "modernc.org/sqlite"
	"os"
	"path/filepath"
)

//go:embed schema.sql
var schema string
var ErrState = errors.New("job state changed")

type Store struct {
	DB   *sql.DB
	Root string
}

func Open(root string) (*Store, error) {
	for _, dir := range []string{"", "staging", "videos", "frames"} {
		if e := os.MkdirAll(filepath.Join(root, dir), 0700); e != nil {
			return nil, e
		}
	}
	db, e := sql.Open("sqlite", filepath.Join(root, "app.sqlite"))
	if e != nil {
		return nil, e
	}
	db.SetMaxOpenConns(1)
	for _, stmt := range []string{"PRAGMA foreign_keys=ON", "PRAGMA busy_timeout=5000", "PRAGMA journal_mode=WAL", schema} {
		if _, e = db.Exec(stmt); e != nil {
			db.Close()
			return nil, e
		}
	}
	return &Store{DB: db, Root: root}, nil
}
func (s *Store) AddVideo(v domain.Video) error {
	_, e := s.DB.Exec(`INSERT INTO videos VALUES(?,?,?,?,?,?,?,?)`, v.ID, v.Name, v.Width, v.Height, v.DurationSeconds, v.Status, v.CreatedAt, v.StorageKey)
	return e
}
func (s *Store) Video(id string) (domain.Video, error) {
	var v domain.Video
	e := s.DB.QueryRow(`SELECT * FROM videos WHERE id=?`, id).Scan(&v.ID, &v.Name, &v.Width, &v.Height, &v.DurationSeconds, &v.Status, &v.CreatedAt, &v.StorageKey)
	return v, e
}
func (s *Store) Videos() ([]domain.Video, error) {
	out := []domain.Video{}
	r, e := s.DB.Query(`SELECT * FROM videos ORDER BY created_at DESC,id`)
	if e != nil {
		return nil, e
	}
	defer r.Close()
	for r.Next() {
		var v domain.Video
		if e = r.Scan(&v.ID, &v.Name, &v.Width, &v.Height, &v.DurationSeconds, &v.Status, &v.CreatedAt, &v.StorageKey); e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, r.Err()
}
func (s *Store) Recover() error {
	if _, e := s.DB.Exec(`UPDATE jobs SET status='failed',error='INTERRUPTED' WHERE status IN ('queued','running','cancelling')`); e != nil {
		return e
	}
	for _, kind := range []string{"staging", "frames", "videos"} {
		entries, e := os.ReadDir(filepath.Join(s.Root, kind))
		if e != nil {
			return e
		}
		for _, v := range entries {
			n := 0
			if kind == "frames" {
				e = s.DB.QueryRow(`SELECT count(*) FROM batches WHERE id=?`, v.Name()).Scan(&n)
			}
			if kind == "videos" {
				e = s.DB.QueryRow(`SELECT count(*) FROM videos WHERE storage_key=?`, filepath.Join("videos", v.Name())).Scan(&n)
			}
			if e != nil {
				return e
			}
			if n == 0 {
				if e = os.RemoveAll(filepath.Join(s.Root, kind, v.Name())); e != nil {
					return e
				}
			}
		}
	}
	return nil
}
