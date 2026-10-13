package migration_test

import (
	"context"
	"database/sql"
	"github.com/XB-Dong/classroom-annotator/backend/internal/migration"
	"github.com/XB-Dong/classroom-annotator/backend/internal/platform/testdb"
	_ "modernc.org/sqlite"
	"os"
	"path/filepath"
	"testing"
)

func sourceFixture(t *testing.T) string {
	t.Helper()
	source := filepath.Join(t.TempDir(), "backup.sqlite")
	db, err := sql.Open("sqlite", source)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	schema, err := os.ReadFile("../platform/store/schema.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(string(schema)); err != nil {
		t.Fatal(err)
	}
	for _, statement := range []string{
		`INSERT INTO videos VALUES('v','synthetic',100,80,2.125,'ready','2026-01-02T03:04:05.000000006Z','videos/v')`,
		`INSERT INTO jobs VALUES('j','extraction','v','b','completed',1,'','2026-01-02T03:04:06Z')`,
		`INSERT INTO batches VALUES('b','v','j','2026-01-02T03:04:07Z')`,
		`INSERT INTO frames VALUES('f','v','b',2147483650,100,80,'completed',2147483651,'frames/b/f.jpg')`,
		`INSERT INTO frames VALUES('zero','v','b',2147483651,100,80,'completed',0,'frames/b/zero.jpg')`,
		`INSERT INTO annotations VALUES('a','f',0.125,1.5,2.25,3.75,'manual')`,
	} {
		if _, err = db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	return source
}
func TestImportPreservesFiveTableHashesAndRefusesSecondRun(t *testing.T) {
	source := sourceFixture(t)
	db, err := sql.Open("pgx", testdb.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	report, err := migration.Import(context.Background(), source, db)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Tables) != 5 {
		t.Fatal("five-table report missing")
	}
	for _, table := range report.Tables {
		if table.Source.Count != table.Target.Count || table.Source.SHA256 != table.Target.SHA256 {
			t.Fatal("canonical digest differs", table.Name)
		}
	}
	var revision, timestamp int64
	var created string
	if err = db.QueryRow(`SELECT revision,timestamp_ms FROM frames WHERE id='f'`).Scan(&revision, &timestamp); err != nil || revision != 2147483651 || timestamp != 2147483650 {
		t.Fatal(revision, timestamp, err)
	}
	if err = db.QueryRow(`SELECT created_at FROM videos WHERE id='v'`).Scan(&created); err != nil || created != "2026-01-02T03:04:05.000000006Z" {
		t.Fatal(created, err)
	}
	if _, err = migration.Import(context.Background(), source, db); err == nil {
		t.Fatal("nonempty target accepted")
	}
}
func TestImportFailureRollsBackAndDoesNotWriteSource(t *testing.T) {
	source := sourceFixture(t)
	before, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("pgx", testdb.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err = db.Exec(`ALTER TABLE annotations ADD CONSTRAINT synthetic_rejection CHECK(id<>'a')`); err != nil {
		t.Fatal(err)
	}
	if _, err = migration.Import(context.Background(), source, db); err == nil {
		t.Fatal("target rejection ignored")
	}
	for _, table := range []string{"videos", "jobs", "batches", "frames", "annotations"} {
		var n int
		if err = db.QueryRow(`SELECT count(*) FROM ` + table).Scan(&n); err != nil || n != 0 {
			t.Fatal("partial target", table, n, err)
		}
	}
	after, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("source changed")
	}
}
func TestImportRejectsInvalidSourceBeforeTarget(t *testing.T) {
	source := filepath.Join(t.TempDir(), "bad.sqlite")
	if err := os.WriteFile(source, []byte("invalid"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := migration.Import(context.Background(), source, nil); err == nil {
		t.Fatal("invalid SQLite backup accepted")
	}
}

func TestImportRejectsUnknownLegacySchemaVersion(t *testing.T) {
	source := sourceFixture(t)
	backup, err := sql.Open("sqlite", source)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := backup.Exec(`UPDATE schema_version SET version=2`); err != nil {
		backup.Close()
		t.Fatal(err)
	}
	if err := backup.Close(); err != nil {
		t.Fatal(err)
	}
	target, err := sql.Open("pgx", testdb.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	defer target.Close()
	if _, err := migration.Import(context.Background(), source, target); err == nil {
		t.Fatal("unknown SQLite schema version accepted")
	}
}

func TestImportRejectsPrepopulatedNewBusinessTable(t *testing.T) {
	source := sourceFixture(t)
	target, err := sql.Open("pgx", testdb.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	defer target.Close()
	fingerprint := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	if _, err := target.Exec(`INSERT INTO media_operations(operation_id,operation_type,provider,request_fingerprint,status,created_at,updated_at) VALUES($1,$2,$3,$4,$5,$6,$7)`, "existing", "extraction", "mac", fingerprint, "queued", "2026-01-01T00:00:00Z", "2026-01-01T00:00:00Z"); err != nil {
		t.Fatal(err)
	}
	if _, err := migration.Import(context.Background(), source, target); err == nil {
		t.Fatal("populated media_operations table accepted")
	}
	var count int
	if err := target.QueryRow(`SELECT count(*) FROM videos`).Scan(&count); err != nil || count != 0 {
		t.Fatal("legacy rows partially imported", count, err)
	}
	if err := target.QueryRow(`SELECT count(*) FROM media_operations`).Scan(&count); err != nil || count != 1 {
		t.Fatal("existing new-table row changed", count, err)
	}
}
