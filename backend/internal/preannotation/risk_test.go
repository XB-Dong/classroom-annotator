package preannotation

import (
	"github.com/XB-Dong/classroom-annotator/backend/internal/domain"
	"testing"
)

func TestRiskReasonsAreServerOwnedAndIncludeTileAndOverlap(t *testing.T) {
	ds := []Detection{{BBox: [4]float64{10, 10, 10, 10}, Confidence: .4, Clipped: true, TileBoundary: true}, {BBox: [4]float64{11, 11, 10, 10}, Confidence: .9}}
	reasons := RiskReasons(ds, []domain.Annotation{{BBox: [4]float64{10, 10, 10, 10}, Source: "manual"}})
	if len(reasons) != 2 {
		t.Fatalf("%v", reasons)
	}
	wanted := map[string]bool{"LOW_CONFIDENCE": true, "SMALL_HEAD": true, "CLIPPED_BOX": true, "TILE_BOUNDARY": true, "POSSIBLE_DUPLICATE": true, "OVERLAP_EXISTING": true}
	for _, r := range reasons[0] {
		delete(wanted, r)
	}
	if len(wanted) != 0 {
		t.Fatalf("missing=%v actual=%v", wanted, reasons)
	}
}
