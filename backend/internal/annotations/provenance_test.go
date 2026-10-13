package annotations

import (
	"encoding/json"
	"github.com/XB-Dong/classroom-annotator/backend/internal/domain"
	"testing"
)

func TestAIAnnotationShapeRequiresImmutableReferences(t *testing.T) {
	a := domain.Annotation{ID: "a", BBox: [4]float64{1, 2, 12, 13}, Source: "ai", CandidateID: "c", RunID: "r"}
	if err := Validate(Save{Status: "in_progress", Annotations: []domain.Annotation{a}}, 100, 100); err != nil {
		t.Fatalf("trusted AI shape rejected: %v", err)
	}
	a.Source = "manual"
	if err := Validate(Save{Status: "in_progress", Annotations: []domain.Annotation{a}}, 100, 100); err == nil {
		t.Fatal("manual cast retained forged AI references")
	}
	a.Source = "ai"
	a.CandidateID = ""
	if err := Validate(Save{Status: "in_progress", Annotations: []domain.Annotation{a}}, 100, 100); err == nil {
		t.Fatal("AI source without candidate accepted")
	}
}

func TestManualWriteRejectsNullCoordinate(t *testing.T) {
	var w writeRequest
	if e := json.Unmarshal([]byte(`{"expectedRevision":0,"status":"in_progress","annotations":[{"id":"a","bbox":[null,2,12,13],"source":"manual"}]}`), &w); e != nil {
		t.Fatal(e)
	}
	if _, e := w.save(); e == nil {
		t.Fatal("null coordinate was converted to zero")
	}
}
