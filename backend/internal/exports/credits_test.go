package exports

import (
	"testing"

	"github.com/XB-Dong/classroom-annotator/backend/internal/platform/store"
)

func TestPublicCreditsSortBySampleVersionVideo(t *testing.T) {
	rows := []store.PublicSample{{SampleID: "b", Version: "1", VideoID: "z", Creator: "person", LicenseName: "CC0"}, {SampleID: "a", Version: "2", VideoID: "x", Creator: "person", LicenseName: "CC BY-SA"}, {SampleID: "a", Version: "1", VideoID: "y", Creator: "person", LicenseName: "CC0"}}
	credits := creditsForRows(rows)
	if len(credits) != 3 || credits[0].Version != "1" || credits[0].SampleID != "a" || credits[1].Version != "2" || credits[2].SampleID != "b" {
		t.Fatalf("unstable order: %+v", credits)
	}
	for _, entry := range credits {
		if entry.Changes != publicSampleChanges {
			t.Errorf("wrong changes: %q", entry.Changes)
		}
	}
}
