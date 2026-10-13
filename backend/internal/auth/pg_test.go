package auth

import (
	"context"
	"database/sql"
	"github.com/XB-Dong/classroom-annotator/backend/internal/migration"
	"github.com/XB-Dong/classroom-annotator/backend/internal/platform/testdb"
	"testing"
)

func TestPostgreSQLAccountAndSessionPersistence(t *testing.T) {
	db, e := sql.Open("pgx", testdb.URL(t))
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	ctx := context.Background()
	repo := NewPGRepository(db)
	s := NewService(repo, "invite")
	u, e := s.Register(ctx, "alice", "long-password-123", "invite")
	if e != nil {
		t.Fatal(e)
	}
	if u.Role != "annotator" {
		t.Fatal("public signup admin")
	}
	token, a, e := s.Login(ctx, "alice", "long-password-123")
	if e != nil {
		t.Fatal(e)
	}
	restarted := NewService(NewPGRepository(db), "invite")
	b, e := restarted.Authenticate(ctx, token)
	if e != nil || a.CSRFToken != b.CSRFToken {
		t.Fatal("session lost on restart")
	}
	if e = s.Logout(ctx, token); e != nil {
		t.Fatal(e)
	}
	if _, e = restarted.Authenticate(ctx, token); e == nil {
		t.Fatal("revocation not durable")
	}
}
func TestPostgreSQLMigrationFiveBackfillsLegacyWithoutRewritingFrames(t *testing.T) {
	db, e := sql.Open("pgx", testdb.Create(t, false))
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	ctx := context.Background()
	p, e := migration.Provider(db)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = p.UpTo(ctx, 4); e != nil {
		t.Fatal(e)
	}
	for _, q := range []string{`INSERT INTO videos(id,name,width,height,duration_seconds,status,created_at,storage_key) VALUES('v','Legacy',100,80,2,'ready','old','videos/v.mp4')`, `INSERT INTO jobs(id,kind,video_id,batch_id,status,created_at) VALUES('j','extraction','v','b','completed','old')`, `INSERT INTO batches(id,video_id,job_id,created_at) VALUES('b','v','j','old')`, `INSERT INTO frames(id,video_id,batch_id,timestamp_ms,width,height,status,revision,storage_key) VALUES('f','v','b',0,100,80,'completed',17,'frames/b/f.jpg')`, `INSERT INTO annotations(id,frame_id,x,y,width,height,source) VALUES('a','f',1,2,3,4,'manual')`} {
		if _, e = db.ExecContext(ctx, q); e != nil {
			t.Fatal(e)
		}
	}
	if _, e = p.UpTo(ctx, 5); e != nil {
		t.Fatal(e)
	}
	var workspace, key, status string
	var revision int64
	var actor sql.NullString
	if e = db.QueryRowContext(ctx, `SELECT v.workspace_id,f.storage_key,f.status,f.revision,f.updated_by FROM frames f JOIN videos v ON v.id=f.video_id WHERE f.id='f'`).Scan(&workspace, &key, &status, &revision, &actor); e != nil {
		t.Fatal(e)
	}
	if workspace != DefaultWorkspaceID || key != "frames/b/f.jpg" || status != "completed" || revision != 17 || actor.Valid {
		t.Fatal("legacy frame rewritten")
	}
	var count int
	if e = db.QueryRowContext(ctx, `SELECT count(*) FROM annotations WHERE id='a' AND x=1 AND y=2 AND width=3 AND height=4`).Scan(&count); e != nil || count != 1 {
		t.Fatal("legacy annotation changed")
	}
}
