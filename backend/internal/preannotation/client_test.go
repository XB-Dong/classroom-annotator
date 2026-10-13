package preannotation

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientSendsImageBytesAndRejectsUntrustedModel(t *testing.T) {
	result := DetectionResult{ContractVersion: "1.0.0", ModelID: ModelID, ModelSHA256: ModelSHA256, PipelineVersion: PipelineVersion, Detections: []Detection{{BBox: [4]float64{1, 2, 10, 11}, Confidence: .8, ClassID: 7}}}
	result.Image.Width = 100
	result.Image.Height = 80
	var seen bool
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer internal-token" || r.Header.Get("X-Request-ID") != "request-123" {
			t.Error("missing internal headers")
		}
		if e := r.ParseMultipartForm(1024); e != nil {
			t.Fatal(e)
		}
		f, _, e := r.FormFile("image")
		if e != nil {
			t.Fatal(e)
		}
		defer f.Close()
		var raw [3]byte
		f.Read(raw[:])
		if string(raw[:]) != "jpg" {
			t.Error("image bytes changed")
		}
		seen = true
		json.NewEncoder(w).Encode(result)
	}))
	defer s.Close()
	c := Client{s.URL, "internal-token"}
	got, e := c.Detect(context.Background(), []byte("jpg"), 100, 80, "request-123")
	if e != nil || !seen || len(got.Detections) != 1 {
		t.Fatalf("got=%+v err=%v seen=%v", got, e, seen)
	}
	result.ModelID = "another-model"
	if _, e = c.Detect(context.Background(), []byte("jpg"), 100, 80, "request-123"); e != ErrInvalidDetection {
		t.Fatalf("untrusted model identity error=%v", e)
	}
	result.ModelID = ModelID
	result.ModelSHA256 = "wrong"
	if _, e = c.Detect(context.Background(), []byte("jpg"), 100, 80, "request-123"); e != ErrInvalidDetection {
		t.Fatalf("untrusted model error=%v", e)
	}
}
func TestClientRejectsDimensionsClassesAndOutOfBounds(t *testing.T) {
	result := DetectionResult{ContractVersion: "1.0.0", ModelID: ModelID, ModelSHA256: ModelSHA256, PipelineVersion: PipelineVersion, Detections: []Detection{}}
	result.Image.Width = 100
	result.Image.Height = 80
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { json.NewEncoder(w).Encode(result) }))
	defer s.Close()
	c := Client{s.URL, "token"}
	for _, d := range []Detection{{BBox: [4]float64{0, 0, 20, 20}, Confidence: .7, ClassID: 16}, {BBox: [4]float64{90, 0, 20, 20}, Confidence: .7, ClassID: 7}, {BBox: [4]float64{0, 0, 0, 20}, Confidence: .7, ClassID: 7}} {
		result.Detections = []Detection{d}
		if _, e := c.Detect(context.Background(), []byte("jpg"), 100, 80, "request"); e != ErrInvalidDetection {
			t.Fatalf("d=%+v err=%v", d, e)
		}
	}
}

func TestClientRejectsIncorrectBBoxArrayLength(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"contractVersion":"1.0.0","modelSHA256":"a5ece478fc918787258afccd0b313e54319f8f39aeb1ceca11f27dbd269e78bc","pipelineVersion":"classroom-head-v1-fullframe-2x2-15","image":{"width":100,"height":80},"detections":[{"bbox":[1,2,10,11,999],"confidence":0.8,"classId":7}],"timingMs":1}`))
	}))
	defer s.Close()
	if _, e := (Client{s.URL, "token"}).Detect(context.Background(), []byte("jpg"), 100, 80, "request"); e != ErrInvalidDetection {
		t.Fatalf("malformed bbox error=%v", e)
	}
}
