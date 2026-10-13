package migration_test

import (
	"database/sql"
	"strings"
	"testing"

	"github.com/XB-Dong/classroom-annotator/backend/internal/platform/testdb"
)

func TestPublicSampleMetadataIsStoredForListingAndCredits(t *testing.T) {
	db, err := sql.Open("pgx", testdb.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	hash := strings.Repeat("a", 64)
	_, err = db.Exec(`INSERT INTO public_samples(sample_id,version,author,license,source_page,media_sha256,media_size_bytes,width,height,duration_seconds,cache_status,name,description,license_url,storage_key) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)`, "sample", "1", "creator", "CC BY-SA 4.0", "https://example.org/source", hash, 1024, 100, 80, 2.5, "ready", "Classroom sample", "Public classroom footage", "https://creativecommons.org/licenses/by-sa/4.0/", "videos/public-samples/sample-v1.mp4")
	if err != nil {
		t.Fatal(err)
	}
	var name, description, licenseURL, storageKey string
	if err := db.QueryRow(`SELECT name,description,license_url,storage_key FROM public_samples WHERE sample_id='sample'`).Scan(&name, &description, &licenseURL, &storageKey); err != nil {
		t.Fatal(err)
	}
	if name != "Classroom sample" || description != "Public classroom footage" || licenseURL == "" || storageKey != "videos/public-samples/sample-v1.mp4" {
		t.Fatal("public sample metadata changed")
	}
}
