package samples

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestCatalogVerifiesRelativeSourceAndHash(t *testing.T) {
	dir := t.TempDir()
	payload := []byte("synthetic video bytes")
	if err := os.WriteFile(filepath.Join(dir, "sample.webm"), payload, 0600); err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256(payload)
	item := Item{SampleID: "public-classroom-01", Version: "1", Name: "sample", Description: "description", Creator: "creator", SourcePageURL: "https://commons.wikimedia.org/wiki/File:Sample.webm", LicenseName: "CC0 1.0", LicenseURL: "https://creativecommons.org/publicdomain/zero/1.0/", SHA256: hex.EncodeToString(hash[:]), SizeBytes: int64(len(payload)), Width: 320, Height: 240, DurationSeconds: 2, SourceFile: "sample.webm"}
	catalogPath := filepath.Join(dir, "catalog.json")
	b, _ := json.Marshal(Catalog{CatalogVersion: "1.0.0", Items: []Item{item}})
	if err := os.WriteFile(catalogPath, b, 0600); err != nil {
		t.Fatal(err)
	}
	items, err := Load(catalogPath)
	if err != nil || len(items) != 1 {
		t.Fatalf("load: %v", err)
	}
	if err := VerifySource(dir, items[0]); err != nil {
		t.Fatal(err)
	}
	items[0].SHA256 = "a" + items[0].SHA256[1:]
	if err := VerifySource(dir, items[0]); err == nil {
		t.Fatal("accepted changed hash")
	}
	items[0] = item
	items[0].SourceFile = "../sample.webm"
	if err := VerifySource(dir, items[0]); err == nil {
		t.Fatal("accepted path escape")
	}
}

func TestCatalogRejectsSymlinkAndDuplicateVersion(t *testing.T) {
	dir := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside.webm")
	if err := os.WriteFile(outside, []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(dir, "link.webm")); err == nil {
		if err := VerifySource(dir, Item{SourceFile: "link.webm", SizeBytes: 1, SHA256: "2d711642b726b04401627ca9fbac32f5da7e5f2bba0f8f5a7fcb3f9c656d3"}); err == nil {
			t.Fatal("accepted symlink")
		}
	}
	item := Item{SampleID: "id", Version: "1", Name: "n", Creator: "a", SourcePageURL: "https://commons.wikimedia.org/wiki/File:A.webm", LicenseName: "CC0", LicenseURL: "https://creativecommons.org/publicdomain/zero/1.0/", SHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", SizeBytes: 1, Width: 1, Height: 1, DurationSeconds: 1, SourceFile: "x.webm"}
	b, _ := json.Marshal(Catalog{CatalogVersion: "1.0.0", Items: []Item{item, item}})
	p := filepath.Join(dir, "catalog.json")
	if err := os.WriteFile(p, b, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(p); err == nil {
		t.Fatal("accepted duplicate sample version")
	}
}
