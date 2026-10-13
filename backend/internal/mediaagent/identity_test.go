package mediaagent

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/XB-Dong/classroom-annotator/backend/internal/mediaapi"
)

func TestCapabilitiesRejectsNonExecutableToolContent(t *testing.T) {
	p := filepath.Join(t.TempDir(), "ffprobe.exe")
	if e := os.WriteFile(p, []byte("this is not an executable"), 0700); e != nil {
		t.Fatal(e)
	}
	if toolAvailable(p) {
		t.Fatal("invalid executable advertised as available")
	}
}
func TestCanonicalFingerprintIgnoresJSONPropertyOrder(t *testing.T) {
	a := mediaapi.ArchiveRequest{OperationID: "archive-a", COCO: json.RawMessage(`{"images":[],"annotations":[]}`)}
	b := a
	b.COCO = json.RawMessage(`{ "annotations": [], "images": [] }`)
	if fingerprint(a) != fingerprint(b) {
		t.Fatal("equivalent request fingerprints differ")
	}
}
