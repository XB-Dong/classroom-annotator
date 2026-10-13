package preannotation

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDecisionRejectsMalformedBBoxRatherThanTruncating(t *testing.T) {
	for _, raw := range []string{`{"candidateId":"c","decision":"adjusted","bbox":[1,2,12,13,99]}`, `{"candidateId":"c","decision":"adjusted","bbox":[1,2,12]}`, `{"candidateId":"c","decision":"accepted","riskReasons":[]}`} {
		var d Decision
		if json.Unmarshal([]byte(raw), &d) == nil {
			t.Fatalf("invalid decision decoded: %s", raw)
		}
	}
}

func TestDecisionRejectsNullCoordinate(t *testing.T) {
	var d Decision
	if json.Unmarshal([]byte(`{"candidateId":"c","decision":"adjusted","bbox":[null,2,12,13]}`), &d) == nil {
		t.Fatal("null coordinate was converted to zero")
	}
}
func TestClientReportsBusyForBoundedSchedulerRetry(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(429) }))
	defer server.Close()
	_, e := (Client{server.URL, "token"}).Detect(context.Background(), []byte("jpg"), 100, 80, "r")
	if e == nil || e.Error() != "AI busy" {
		t.Fatalf("busy response lost: %v", e)
	}
}
func TestReviewBoundsAndNullFractions(t *testing.T) {
	b := [4]float64{1, 2, 12, 13}
	w := ReviewWrite{ExpectedRevision: 4, Decisions: []Decision{{CandidateID: "c", Decision: "adjusted", BBox: &b}}}
	if e := ValidateReview(w, 100, 80); e != nil {
		t.Fatal(e)
	}
	w.Decisions = append(w.Decisions, w.Decisions[0])
	if ValidateReview(w, 100, 80) == nil {
		t.Fatal("duplicate decisions accepted")
	}
	if fraction(0, 0) != nil {
		t.Fatal("empty denominator pretended to be evaluated")
	}
	if f := fraction(2, 4); f == nil || *f != .5 {
		t.Fatal(f)
	}
}
