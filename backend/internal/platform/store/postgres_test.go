package store

import (
	"database/sql"
	"errors"
	"testing"

	"github.com/XB-Dong/classroom-annotator/backend/internal/platform/testdb"
	"github.com/XB-Dong/classroom-annotator/backend/migrations"
)

func TestProductionStoreRejectsMissingDatabaseConfiguration(t *testing.T) {
	s, err := Open(t.TempDir())
	if err == nil {
		s.DB.Close()
		t.Fatal("production silently initialized SQLite without PostgreSQL configuration")
	}
}

func TestProductionStoreRejectsOutdatedSchema(t *testing.T) {
	dsn := testdb.URL(t)
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE goose_db_version SET is_applied=false WHERE version_id=$1`, migrations.Version); err != nil {
		db.Close()
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	s, err := Open(t.TempDir(), Options{DatabaseURL: dsn})
	if err == nil {
		s.DB.Close()
		t.Fatal("outdated PostgreSQL schema accepted")
	}
	if !errors.Is(err, ErrSchema) {
		t.Fatal("schema mismatch was not identified", err)
	}
}
