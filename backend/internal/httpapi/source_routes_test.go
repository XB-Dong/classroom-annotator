package httpapi

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/XB-Dong/classroom-annotator/backend/internal/platform/store"
	"github.com/XB-Dong/classroom-annotator/backend/internal/platform/testdb"
)

func TestAvailabilityWireIncludesNullableReason(t *testing.T) {
	for _, tc := range []struct {
		input providerAvailability
		want  string
	}{
		{providerAvailability{Provider: "local", availability: availability{Available: true}}, `{"provider":"local","available":true,"reasonCode":null}`},
		{providerAvailability{Provider: "mac", availability: availability{Available: false, ReasonCode: reason("MEDIA_SOURCE_UNAVAILABLE")}}, `{"provider":"mac","available":false,"reasonCode":"MEDIA_SOURCE_UNAVAILABLE"}`},
	} {
		wire, err := json.Marshal(tc.input)
		if err != nil || string(wire) != tc.want {
			t.Fatalf("availability: %s %v", wire, err)
		}
	}
}

func TestRestartWithoutMacBridgeKeepsCleanupPending(t *testing.T) {
	s, err := store.Open(t.TempDir(), store.Options{DatabaseURL: testdb.URL(t)})
	if err != nil {
		t.Fatal(err)
	}
	defer s.DB.Close()
	ctx := context.Background()
	op := store.MediaOperation{OperationID: "pending-upload", OperationType: "upload", ResourceID: "unpublished-video", Provider: "mac", RequestFingerprint: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Status: "queued"}
	if err = s.PersistOperation(ctx, op); err != nil {
		t.Fatal(err)
	}
	sources := &SourceService{Store: s}
	if err = sources.ReconcileRemote(ctx); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetOperation(ctx, op.OperationID)
	if err != nil || got.Status != "failed" || got.ErrorCode != "REMOTE_CLEANUP_PENDING" || got.PublicationStatus != "unpublished" {
		t.Fatalf("restart ledger: %+v %v", got, err)
	}
}
