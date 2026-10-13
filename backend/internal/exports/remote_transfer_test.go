package exports

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/XB-Dong/classroom-annotator/backend/internal/mediabridge"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
)

func TestRemoteZIPTransferCountsOnlyCompleteBody(t *testing.T) {
	for _, tc := range []struct {
		name      string
		body      string
		wantLabel string
	}{{"truncated", "ab", "transfer_failed"}, {"complete", "abc", "success"}} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Length", "3")
				w.Header().Set("Content-Type", "application/zip")
				w.WriteHeader(200)
				_, _ = io.WriteString(w, tc.body)
			}))
			defer server.Close()
			bridge, err := mediabridge.New(mediabridge.Config{BaseURL: server.URL, Token: strings.Repeat("x", 24), DownloadTimeout: time.Second})
			if err != nil {
				t.Fatal(err)
			}
			response, err := bridge.OpenMedia(context.Background(), http.MethodGet, "archive-1", "")
			if err != nil {
				t.Fatal(err)
			}
			defer response.Body.Close()
			registry := prometheus.NewRegistry()
			counter := prometheus.NewCounterVec(prometheus.CounterOpts{Name: "review_remote_zip_total", Help: "Remote ZIP transfer outcome"}, []string{"result"})
			registry.MustRegister(counter)
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
			streamRemoteZIP(c, response, 3, counter)
			metrics, err := registry.Gather()
			if err != nil {
				t.Fatal(err)
			}
			found := false
			for _, family := range metrics {
				for _, metric := range family.Metric {
					label := metric.Label[0].GetValue()
					if label == tc.wantLabel {
						found = true
					}
					if label == tc.wantLabel && metric.Counter.GetValue() != 1 {
						t.Fatalf("%s count %v", label, metric.Counter.GetValue())
					}
					if label != tc.wantLabel && metric.Counter.GetValue() > 0 {
						t.Fatalf("unexpected %s count %v", label, metric.Counter.GetValue())
					}
				}
			}
			if !found {
				t.Fatalf("missing %s metric", tc.wantLabel)
			}
			if tc.name == "complete" && recorder.Body.String() != "abc" {
				t.Fatalf("complete bytes %q", recorder.Body.String())
			}
		})
	}
}
