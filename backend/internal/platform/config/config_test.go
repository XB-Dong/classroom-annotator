package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOptionalAIConfigurationRejectsUnsafeOriginsAndCheckoutSecrets(t *testing.T) {
	root := t.TempDir()
	checkout := filepath.Join(root, "checkout")
	if err := os.MkdirAll(checkout, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(checkout, ".git"), []byte("gitdir: external"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, item := range []struct {
		url, token string
		rejected   bool
	}{
		{"", "", false},
		{"http://ai:8090", filepath.Join(root, "ai-token"), false},
		{"http://user:secret@ai:8090", "", true},
		{"http://ai:8090/v1/detections", "", true},
		{"http://ai:8090?secret=x", "", true},
		{"http://ai:8090", "relative-secret", true},
		{"http://ai:8090", filepath.Join(checkout, "token"), true},
	} {
		values := map[string]string{"APP_DATA_DIR": filepath.Join(root, "data"), "APP_DATABASE_URL": "postgres://localhost/test", "APP_AI_URL": item.url, "APP_AI_TOKEN_FILE": item.token}
		_, err := Load(func(k string) string { return values[k] })
		if (err != nil) != item.rejected {
			t.Fatalf("AI configuration rejection=%v, expected=%v", err != nil, item.rejected)
		}
	}
}

func TestConfigRequiresExternalDataAndPositiveLimits(t *testing.T) {
	get := func(string) string { return "" }
	if _, err := Load(get); err == nil {
		t.Fatal("missing DATA_DIR accepted")
	}
	root := t.TempDir()
	values := map[string]string{"DATA_DIR": root, "MAX_UPLOAD_BYTES": "0", "APP_DATABASE_URL": "postgres://localhost/test"}
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
func TestDataDirectoryCannotBeInsideCheckout(t *testing.T) {
	root := t.TempDir()
	if e := os.WriteFile(filepath.Join(root, ".git"), []byte("gitdir: external"), 0600); e != nil {
		t.Fatal(e)
	}
	if _, e := Load(func(k string) string {
		if k == "APP_DATA_DIR" {
			return filepath.Join(root, "runtime", "data")
		}
		return ""
	}); e == nil {
		t.Fatal("repository data directory accepted")
	}
}
func TestRejectInvalidAddressAndOverflowingUploadLimit(t *testing.T) {
	for _, values := range []map[string]string{{"APP_ADDR": "bad"}, {"APP_ADDR": ":99999"}, {"APP_MAX_UPLOAD_BYTES": "9223372036854775807"}} {
		values["APP_DATA_DIR"] = t.TempDir()
		if _, e := Load(func(k string) string { return values[k] }); e == nil {
			t.Fatalf("invalid config accepted: %v", values)
		}
	}
}
