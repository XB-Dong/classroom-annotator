package httpapi

import (
	"bufio"
	"encoding/json"
	"fmt"
	"github.com/XB-Dong/classroom-annotator/backend/internal/dataset"
	"github.com/XB-Dong/classroom-annotator/backend/internal/extraction"
	"github.com/XB-Dong/classroom-annotator/backend/internal/jobs"
	"github.com/XB-Dong/classroom-annotator/backend/internal/media"
	"github.com/XB-Dong/classroom-annotator/backend/internal/mediaapi"
	"github.com/XB-Dong/classroom-annotator/backend/internal/mediabridge"
	"github.com/XB-Dong/classroom-annotator/backend/internal/platform/config"
	"github.com/XB-Dong/classroom-annotator/backend/internal/platform/store"
	"github.com/XB-Dong/classroom-annotator/backend/internal/platform/testdb"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestUploadDeadlineCoversMultipartHeaderAndStalledFile(t *testing.T) {
	for _, budget := range []time.Duration{150 * time.Millisecond, time.Second} {
		for _, body := range []string{"--x\r\nContent-Disposition:", "--x\r\nContent-Disposition: form-data; name=\"file\"; filename=\"slow.mp4\"\r\nContent-Type: video/mp4\r\n\r\npartial-video"} {
			t.Run(fmt.Sprintf("%dms-%d", budget.Milliseconds(), len(body)), func(t *testing.T) {
				s, e := store.Open(t.TempDir(), store.Options{DatabaseURL: testdb.URL(t)})
				if e != nil {
					t.Fatal(e)
				}
				defer s.DB.Close()
				cfg := config.Config{MaxUploadBytes: 1024, JobTimeout: budget}
				if budget == time.Second {
					cfg, e = config.Load(func(k string) string {
						switch k {
						case "APP_DATA_DIR":
							return s.Root
						case "APP_DATABASE_URL":
							return s.Options.DatabaseURL
						case "APP_JOB_TIMEOUT_SECONDS":
							return "1"
						case "APP_MAX_UPLOAD_BYTES":
							return "1024"
						}
						return ""
					})
					if e != nil {
						t.Fatal(e)
					}
				}
				m := jobs.New(s, 1, time.Second)
				defer m.Close()
				remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					switch {
					case r.URL.Path == "/bridge/v1/capabilities":
						_ = json.NewEncoder(w).Encode(mediaapi.Capabilities{ContractVersion: mediaapi.ContractVersion, MediaRootWritable: true, FFprobeAvailable: true, FFmpegAvailable: true, MaxUploadBytes: 1 << 30, MaxFrames: 1000, MaxControlBytes: 16 << 20, MaxOperationSeconds: 600, LeaseSeconds: 120, QueueSize: 8})
					case r.URL.Path == "/bridge/v1/uploads":
						_, _ = io.Copy(io.Discard, r.Body)
					case strings.HasSuffix(r.URL.Path, "/cancel"):
						parts := strings.Split(r.URL.Path, "/")
						_ = json.NewEncoder(w).Encode(mediaapi.Operation{ContractVersion: mediaapi.ContractVersion, OperationID: parts[len(parts)-2], Status: "cancelled"})
					default:
						w.WriteHeader(http.StatusNotFound)
					}
				}))
				defer remote.Close()
				bridge, e := mediabridge.New(mediabridge.Config{BaseURL: remote.URL, Token: "test-agent-token-123", AllowedOrigins: []string{remote.URL}, Timeout: budget})
				if e != nil {
					t.Fatal(e)
				}
				sources := &SourceService{Store: s, Jobs: m, Bridge: bridge, MaxUploadBytes: cfg.MaxUploadBytes, MaxFrames: 10}
				r := NewWithSources(cfg, s, m, &media.Service{Root: s.Root, Limit: cfg.MaxUploadBytes, Runner: flowRunner{}}, &extraction.Service{}, dataset.New(cfg), sources)
				server := httptest.NewServer(r)
				defer server.Close()
				conn, e := net.Dial("tcp", strings.TrimPrefix(server.URL, "http://"))
				if e != nil {
					t.Fatal(e)
				}
				defer conn.Close()
				conn.SetDeadline(time.Now().Add(budget + 700*time.Millisecond))
				start := time.Now()
				fmt.Fprintf(conn, "POST /api/v1/videos HTTP/1.1\r\nHost: fixture\r\nContent-Type: multipart/form-data; boundary=x\r\nContent-Length: 900\r\n\r\n%s", body)
				resp, e := http.ReadResponse(bufio.NewReader(conn), nil)
				if e != nil {
					t.Fatal("handler remained blocked beyond upload deadline", time.Since(start), e)
				}
				defer resp.Body.Close()
				if resp.StatusCode != 503 {
					t.Fatal("timeout not reported safely", resp.StatusCode)
				}
				t.Logf("configured timeout=%s, observed response=%s", budget, time.Since(start))
				if time.Since(start) > budget+350*time.Millisecond {
					t.Fatal("timeout returned late", time.Since(start))
				}
				for _, dir := range []string{"staging", "videos"} {
					entries, e := os.ReadDir(filepath.Join(s.Root, dir))
					if e != nil || len(entries) != 0 {
						t.Fatal("upload left media", dir, entries, e)
					}
				}
				videos, e := s.Videos()
				if e != nil || len(videos) > 0 {
					t.Fatal(videos, e)
				}
			})
		}
	}
}
