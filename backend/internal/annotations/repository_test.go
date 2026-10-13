package annotations

import (
	"context"
	"errors"
	"math"
	"reflect"
	"strings"
	"testing"

	"github.com/XB-Dong/classroom-annotator/backend/internal/domain"
	"github.com/XB-Dong/classroom-annotator/backend/internal/platform/store"
)

func TestGeometryRejectsNonfiniteAndAcceptsPositiveSubpixelBoxes(t *testing.T) {
	valid := Save{ExpectedRevision: 0, Status: "in_progress", Annotations: []domain.Annotation{{ID: "edge", Source: "manual", BBox: [4]float64{99.5, 79.5, 0.5, 0.5}}}}
	if err := Validate(valid, 100, 80); err != nil {
		t.Fatal("positive subpixel box rejected", err)
	}
	for _, n := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		for pos := 0; pos < 4; pos++ {
			bad := valid
			bad.Annotations = append([]domain.Annotation(nil), valid.Annotations...)
			bad.Annotations[0].BBox[pos] = n
			if err := Validate(bad, 100, 80); !errors.Is(err, ErrInvalid) {
				t.Fatal("nonfinite accepted", pos, n, err)
			}
		}
	}
	bad := valid
	bad.Annotations = append([]domain.Annotation(nil), valid.Annotations...)
	bad.Annotations[0].ID = strings.Repeat("a", MaxIDLength+1)
	if err := Validate(bad, 100, 80); !errors.Is(err, ErrInvalid) {
		t.Fatal("unbounded ID accepted", err)
	}
}

func TestSnapshotSurvivesDatabaseReopen(t *testing.T) {
	s, r := fixture(t)
	w := request(r, "PUT", "f1", firstSave)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	before := snapshot(t, r, "f1")
	if err := s.DB.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := store.Open(s.Root, s.Options)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.DB.Close()
	got, err := (Repository{Store: reopened}).Get(context.Background(), "f1")
	if err != nil || !reflect.DeepEqual(got, before) {
		t.Fatal("persisted snapshot differs", got, err)
	}
}
