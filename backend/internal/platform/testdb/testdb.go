// Package testdb creates isolated PostgreSQL schemas for integration tests.
package testdb

import (
	"context"
	"database/sql"
	"github.com/XB-Dong/classroom-annotator/backend/internal/migration"
	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
)

// URL requires an explicitly provided test database and never falls back to a live database.
func URL(t testing.TB) string { return Create(t, true) }
func Create(t testing.TB, migrate bool) string {
	t.Helper()
	raw := os.Getenv("P01_TEST_DATABASE_URL")
	if raw == "" {
		t.Fatal("P01_TEST_DATABASE_URL is required for PostgreSQL integration tests")
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || (u.Scheme != "postgres" && u.Scheme != "postgresql") || strings.TrimPrefix(u.Path, "/") != "p01_test_go" {
		t.Fatal("P01_TEST_DATABASE_URL must address the dedicated p01_test_go database")
	}
	schema := "p01_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	admin, err := sql.Open("pgx", raw)
	if err != nil {
		t.Fatal("test database open failed")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if _, err = admin.ExecContext(ctx, `CREATE SCHEMA `+schema); err != nil {
		admin.Close()
		t.Fatal("test schema creation failed")
	}
	t.Cleanup(func() {
		defer admin.Close()
		if !strings.HasPrefix(schema, "p01_test_") || len(schema) != 41 {
			t.Error("unsafe test schema cleanup refused")
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if _, err := admin.ExecContext(ctx, `DROP SCHEMA `+schema+` CASCADE`); err != nil {
			t.Error("test schema cleanup failed")
		}
	})
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	if migrate {
		db, err := sql.Open("pgx", u.String())
		if err != nil {
			t.Fatal("test database open failed")
		}
		defer db.Close()
		if err = migration.Up(ctx, db); err != nil {
			t.Fatal("test migration failed:", err)
		}
	}
	return u.String()
}
