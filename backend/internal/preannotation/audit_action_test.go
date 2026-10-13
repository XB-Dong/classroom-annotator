package preannotation

import "testing"

func TestReviewAuditDescribesCommittedActions(t *testing.T) {
	for _, item := range []struct {
		name     string
		write    ReviewWrite
		expected string
	}{
		{"accepted", ReviewWrite{Decisions: []Decision{{Decision: "accepted"}}}, "accept"},
		{"rejected", ReviewWrite{Decisions: []Decision{{Decision: "rejected"}}}, "reject"},
		{"adjusted", ReviewWrite{Decisions: []Decision{{Decision: "adjusted"}}}, "adjust"},
		{"image_checked", ReviewWrite{IssueResolutions: []IssueResolution{{Resolution: "manual_checked"}}}, "resolve"},
		{"mixed_decisions", ReviewWrite{Decisions: []Decision{{Decision: "accepted"}, {Decision: "rejected"}}}, "save"},
		{"decisions_and_image", ReviewWrite{Decisions: []Decision{{Decision: "accepted"}}, IssueResolutions: []IssueResolution{{Resolution: "manual_checked"}}}, "save"},
		{"empty", ReviewWrite{}, "save"},
	} {
		t.Run(item.name, func(t *testing.T) {
			if action := reviewAction(item.write); action != item.expected {
				t.Fatalf("committed audit action %q, expected %q", action, item.expected)
			}
		})
	}
}
