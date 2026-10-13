package config

import (
	"path/filepath"
	"testing"
)

func TestLocalSourceRootsAndRetiredMacConfig(t *testing.T) {
	root := t.TempDir()
	values := map[string]string{"APP_DATA_DIR": t.TempDir(), "APP_DATABASE_URL": "postgres://localhost/test", "APP_LOCAL_SOURCE_DIRS": root, "APP_MAC_BRIDGE_URL": "ignored retired malformed value"}
	c, e := Load(func(k string) string { return values[k] })
	if e != nil {
		t.Fatal("retired Mac config still affects startup", e)
	}
	if len(c.LocalSourceDirs) != 1 || c.LocalSourceDirs[0] != filepath.Clean(root) {
		t.Fatal("roots not configured")
	}
	values["APP_LOCAL_SOURCE_DIRS"] = "../escape"
	if _, e = Load(func(k string) string { return values[k] }); e == nil {
		t.Fatal("relative source accepted")
	}
}
