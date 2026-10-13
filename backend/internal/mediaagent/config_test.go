package mediaagent

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRuntimeConfigRejectsNonLoopbackAndLegacyPort(t *testing.T) {
	root := t.TempDir()
	token := filepath.Join(t.TempDir(), "token")
	if e := os.WriteFile(token, []byte("0123456789abcdef0123456789abcdef"), 0600); e != nil {
		t.Fatal(e)
	}
	t.Setenv("MAC_MEDIA_ROOT", root)
	t.Setenv("MAC_TOKEN_FILE", token)
	for _, addr := range []string{"0.0.0.0:19092", "127.0.0.1:19090", "localhost:19092"} {
		t.Setenv("MAC_AGENT_ADDR", addr)
		if _, _, e := LoadConfig(); e == nil {
			t.Fatalf("unsafe bind accepted %s", addr)
		}
	}
	t.Setenv("MAC_AGENT_ADDR", "127.0.0.1:19092")
	cfg, addr, e := LoadConfig()
	if e != nil {
		t.Fatal(e)
	}
	if cfg.Root != root || addr != "127.0.0.1:19092" {
		t.Fatal("config mismatch")
	}
}
