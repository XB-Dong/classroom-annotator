package help

import (
	"context"
	"testing"
)

type fakeProvider struct{ raw RawAnswer }

func (p fakeProvider) Answer(context.Context, Document, string) (RawAnswer, error) { return p.raw, nil }
func TestServiceValidatesReferencesAndKeepsServerOwnedMetadata(t *testing.T) {
	d, _ := EmbeddedDocument()
	s := NewService(d, fakeProvider{RawAnswer{"逐项检查。", []string{"risk"}}})
	a, e := s.Ask(context.Background(), "actor", "如何复核？")
	if e != nil || a.DocumentSHA256 != d.SHA256 || len(a.References) != 1 || a.References[0].Anchor != "#risk" {
		t.Fatalf("%+v %v", a, e)
	}
	for _, refs := range [][]string{nil, {"unknown"}, {"https://evil.test"}} {
		s.Provider = fakeProvider{RawAnswer{"说明", refs}}
		if _, e = s.Ask(context.Background(), "actor", "问题"); e != ErrInvalidResponse {
			t.Fatalf("references %v error %v", refs, e)
		}
	}
}
func TestServiceBoundsQuestionsAndPerUserQuota(t *testing.T) {
	d, _ := EmbeddedDocument()
	s := NewService(d, fakeProvider{RawAnswer{"说明", []string{"risk"}}})
	for i := 0; i < 10; i++ {
		if _, e := s.Ask(context.Background(), "actor", "问题"); e != nil {
			t.Fatal(e)
		}
	}
	if _, e := s.Ask(context.Background(), "actor", "问题"); e != ErrLimited {
		t.Fatalf("error=%v", e)
	}
	if _, e := s.Ask(context.Background(), "other", ""); e != ErrInvalidQuestion {
		t.Fatalf("error=%v", e)
	}
	if _, e := s.Ask(context.Background(), "other", "问题"); e != nil {
		t.Fatal(e)
	}
}
