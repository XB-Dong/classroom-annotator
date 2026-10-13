package config

import (
	"path/filepath"
	"testing"
)

func TestDatabaseURLRequiredAndPoolLimitsValidated(t *testing.T) {
	values := map[string]string{"APP_DATA_DIR": filepath.Clean(t.TempDir())}
	get := func(k string) string { return values[k] }
	if _, err := Load(get); err == nil {
		t.Fatal("missing APP_DATABASE_URL accepted")
	}
	values["APP_DATABASE_URL"] = "postgres://user:password@localhost/test?sslmode=disable"
	if _, err := Load(get); err != nil {
		t.Fatal(err)
	}
	for _, pair := range [][2]string{{"APP_DB_MAX_OPEN_CONNS", "0"}, {"APP_DB_MAX_IDLE_CONNS", "6"}, {"APP_DB_CONNECT_TIMEOUT_SECONDS", "0"}, {"APP_DB_QUERY_TIMEOUT_SECONDS", "0"}, {"APP_DB_CONN_MAX_LIFETIME_SECONDS", "0"}, {"APP_DB_CONN_MAX_IDLE_SECONDS", "0"}} {
		values[pair[0]] = pair[1]
		if _, err := Load(get); err == nil {
			t.Fatalf("invalid %s accepted", pair[0])
		}
		delete(values, pair[0])
	}
	values["APP_DATABASE_URL"] = "file:app.sqlite"
	if _, err := Load(get); err == nil {
		t.Fatal("SQLite URL accepted")
	}
}
