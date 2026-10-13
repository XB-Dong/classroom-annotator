package preannotation

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestArchitectureAIResponseRequiredFields(t *testing.T) {
	cases := []struct {
		name, field string
		null        bool
	}{
		{"missing_detections", "detections", false}, {"null_detections", "detections", true},
		{"missing_modelId", "modelId", false}, {"null_modelId", "modelId", true},
		{"missing_timingMs", "timingMs", false}, {"null_timingMs", "timingMs", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			response := map[string]any{"contractVersion": "1.0.0", "modelId": "fixture", "modelSHA256": ModelSHA256, "pipelineVersion": PipelineVersion, "image": map[string]int{"width": 100, "height": 80}, "detections": []any{}, "timingMs": 1}
			if tc.null {
				response[tc.field] = nil
			} else {
				delete(response, tc.field)
			}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(response)
			}))
			defer server.Close()
			got, err := (Client{URL: server.URL, Token: "synthetic-internal-token"}).Detect(context.Background(), []byte("synthetic-image"), 100, 80, "review-required-field")
			if err == nil {
				t.Errorf("invalid response accepted as success: field=%s null=%v detections=%d modelId=%q timingMs=%v", tc.field, tc.null, len(got.Detections), got.ModelID, got.TimingMs)
			}
		})
	}
}
func TestArchitectureAIResponseRejectsNullBBoxCoordinate(t *testing.T) {
	response := map[string]any{"contractVersion": "1.0.0", "modelId": "fixture", "modelSHA256": ModelSHA256, "pipelineVersion": PipelineVersion, "image": map[string]int{"width": 100, "height": 80}, "detections": []any{map[string]any{"bbox": []any{nil, 2, 10, 11}, "confidence": 0.8, "classId": 7}}, "timingMs": 1}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { json.NewEncoder(w).Encode(response) }))
	defer server.Close()
	got, err := (Client{URL: server.URL, Token: "synthetic-internal-token"}).Detect(context.Background(), []byte("synthetic-image"), 100, 80, "review-null-coordinate")
	if err == nil {
		t.Errorf("null numeric coordinate accepted as x=0: detections=%+v", got.Detections)
	}
}
func TestArchitectureAIResponseAllowsExplicitZeroDetections(t *testing.T) {
	response := map[string]any{"contractVersion": "1.0.0", "modelId": "fixture", "modelSHA256": ModelSHA256, "pipelineVersion": PipelineVersion, "image": map[string]int{"width": 100, "height": 80}, "detections": []any{}, "timingMs": 0}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { json.NewEncoder(w).Encode(response) }))
	defer server.Close()
	got, err := (Client{URL: server.URL, Token: "synthetic-internal-token"}).Detect(context.Background(), []byte("synthetic-image"), 100, 80, "review-zero-detections")
	if err != nil || len(got.Detections) != 0 {
		t.Fatalf("explicit valid zero-result rejected: got=%+v error=%v", got, err)
	}
}
