package httpapi

import (
	"encoding/json"
	"testing"
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
