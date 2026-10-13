package preannotation

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestPostgreSQLReviewAuditMatchesDecisionAndImageResolution(t *testing.T) {
	for _, item := range []struct{ decision, action string }{{"accepted", "accept"}, {"adjusted", "adjust"}, {"rejected", "reject"}, {"manual_checked", "resolve"}} {
		t.Run(item.decision, func(t *testing.T) {
			s, ctx := pgFixture(t, func(w http.ResponseWriter, r *http.Request) {
				result := pgResult()
				if item.decision == "manual_checked" {
					result.Detections = []Detection{}
				}
				json.NewEncoder(w).Encode(result)
			})
			startFrame(t, s, ctx, "audit-action", 0)
			snap, err := s.Get(ctx, "f1")
			if err != nil {
				t.Fatal(err)
			}
			write := ReviewWrite{ExpectedRevision: snap.Revision}
			if item.decision == "manual_checked" {
				write.IssueResolutions = []IssueResolution{{ID: snap.Issues[0].ID, Resolution: "manual_checked"}}
			} else {
				decision := Decision{CandidateID: snap.Candidates[0].ID, Decision: item.decision}
				if item.decision == "adjusted" {
					box := [4]float64{2, 2, 20, 20}
					decision.BBox = &box
				}
				write.Decisions = []Decision{decision}
			}
			out, err := s.Review(ctx, "f1", write)
			if err != nil {
				t.Fatal(err)
			}
			var action string
			if err = s.Store.DB.QueryRow(`SELECT action FROM annotation_events WHERE frame_id=$1 AND revision=$2`, "f1", out.Revision).Scan(&action); err != nil || action != item.action {
				t.Fatalf("audit=%q expected=%q error=%v", action, item.action, err)
			}
		})
	}
}
