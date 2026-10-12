package config

import (
	"path/filepath"
	"testing"
)

func TestConfigRequiresExternalDataAndPositiveLimits(t *testing.T) {
	get := func(string) string { return "" }
	if _, err := Load(get); err == nil {
		t.Fatal("missing DATA_DIR accepted")
	}
	root := t.TempDir()
	values := map[string]string{"DATA_DIR": root, "MAX_UPLOAD_BYTES": "0"}
	if _, err := Load(func(k string) string { return values[k] }); err == nil {
		t.Fatal("zero limit accepted")
	}
	values["MAX_UPLOAD_BYTES"] = "1024"
	c, err := Load(func(k string) string { return values[k] })
	if err != nil {
		t.Fatal(err)
	}
	if c.MaxUploadBytes != 1024 || c.DataDir != filepath.Clean(root) || c.QueueSize < 1 {
		t.Fatalf("bad config: %+v", c)
	}
}
