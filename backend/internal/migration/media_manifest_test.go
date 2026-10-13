package migration_test

import (
	"database/sql"
	"strings"
	"testing"

	"github.com/XB-Dong/classroom-annotator/backend/internal/platform/testdb"
)

func TestMediaOperationManifestDigestIsDurableAndValidated(t *testing.T) {
	db, err := sql.Open("pgx", testdb.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	fingerprint, digest := strings.Repeat("a", 64), strings.Repeat("b", 64)
	_, err = db.Exec(`INSERT INTO media_operations(operation_id,operation_type,provider,request_fingerprint,status,created_at,updated_at,manifest_sha256) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, "op", "extraction", "mac", fingerprint, "completed", "2026-01-01T00:00:00Z", "2026-01-01T00:00:01Z", digest)
	if err != nil {
		t.Fatal(err)
	}
	var got string
	if err := db.QueryRow(`SELECT manifest_sha256 FROM media_operations WHERE operation_id='op'`).Scan(&got); err != nil || got != digest {
		t.Fatal("manifest digest was not preserved", err)
	}
	_, err = db.Exec(`UPDATE media_operations SET manifest_sha256='BAD' WHERE operation_id='op'`)
	if err == nil {
		t.Fatal("invalid manifest digest accepted")
	}
}
