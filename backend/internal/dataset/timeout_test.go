package dataset

import (
	"github.com/XB-Dong/classroom-annotator/backend/internal/platform/config"
	"testing"
	"time"
)

func TestControlAndMediaDownloadTimeoutsAreIndependent(t *testing.T) {
	a := New(config.Config{HTTPTimeout: 3 * time.Second, MediaDownloadTimeout: 10 * time.Second})
	if a.api.Timeout != 3*time.Second || a.media.Timeout != 10*time.Second {
		t.Fatalf("control=%v media=%v", a.api.Timeout, a.media.Timeout)
	}
}
