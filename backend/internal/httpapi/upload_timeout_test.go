package httpapi

import (
	"bufio"
	"fmt"
	"github.com/XB-Dong/classroom-annotator/backend/internal/dataset"
	"github.com/XB-Dong/classroom-annotator/backend/internal/extraction"
	"github.com/XB-Dong/classroom-annotator/backend/internal/jobs"
	"github.com/XB-Dong/classroom-annotator/backend/internal/media"
	"github.com/XB-Dong/classroom-annotator/backend/internal/platform/config"
	"github.com/XB-Dong/classroom-annotator/backend/internal/platform/store"
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
				s, e := store.Open(t.TempDir())
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
				r := New(cfg, s, m, &media.Service{Root: s.Root, Limit: cfg.MaxUploadBytes, Runner: flowRunner{}}, &extraction.Service{}, dataset.New(cfg))
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
