package annotations

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"sync"
	"testing"

	"github.com/XB-Dong/classroom-annotator/backend/internal/domain"
	"github.com/XB-Dong/classroom-annotator/backend/internal/platform/store"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
)

func fixture(t *testing.T) (*store.Store, *gin.Engine) {
	t.Helper()
	s, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.DB.Close() })
	v := domain.Video{ID: "v", Name: "synthetic", Width: 100, Height: 80, DurationSeconds: 1, Status: "ready", CreatedAt: domain.Now(), StorageKey: "videos/v"}
	if err = s.AddVideo(v); err != nil {
		t.Fatal(err)
	}
	j := domain.Job{ID: "j", Kind: "extraction", VideoID: "v", BatchID: "b", Status: "running", CreatedAt: domain.Now()}
	if err = s.AddJob(j); err != nil {
		t.Fatal(err)
	}
	frames := []domain.Frame{}
	for i, id := range []string{"f1", "f2"} {
		frames = append(frames, domain.Frame{ID: id, VideoID: "v", BatchID: "b", TimestampMs: int64(i), Width: 100, Height: 80, StorageKey: filepath.Join("frames", "b", id+".jpg")})
	}
	if err = s.PublishBatch(j, frames); err != nil {
		t.Fatal(err)
	}
	gin.SetMode(gin.TestMode)
	r := gin.New()
	Register(r.Group("/api/v1"), s, prometheus.NewRegistry())
	return s, r
}

func request(r *gin.Engine, method, id, body string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(method, "/api/v1/frames/"+id+"/annotations", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	return w
}

func snapshot(t *testing.T, r *gin.Engine, id string) domain.Snapshot {
	t.Helper()
	w := request(r, "GET", id, "")
	if w.Code != 200 {
		t.Fatalf("GET status %d: %s", w.Code, w.Body.String())
	}
	var got domain.Snapshot
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	return got
}

const firstSave = `{"expectedRevision":0,"status":"completed","annotations":[{"id":"a1","bbox":[0,0,10.5,20],"source":"manual"}]}`

func TestSaveRevisionGeometryAndZeroCompletion(t *testing.T) {
	s, r := fixture(t)
	initial := snapshot(t, r, "f1")
	if initial.Revision != 0 || initial.Status != "not_started" || initial.Annotations == nil || len(initial.Annotations) != 0 {
		t.Fatal(initial)
	}
	if w := request(r, "PUT", "f1", firstSave); w.Code != 200 {
		t.Fatalf("save %d: %s", w.Code, w.Body.String())
	}
	saved := snapshot(t, r, "f1")
	if saved.Revision != 1 || saved.Status != "completed" || len(saved.Annotations) != 1 || saved.Annotations[0].BBox[2] != 10.5 {
		t.Fatal(saved)
	}
	if w := request(r, "PUT", "f1", firstSave); w.Code != 409 {
		t.Fatalf("stale status %d", w.Code)
	}
	if got := snapshot(t, r, "f1"); !reflect.DeepEqual(got, saved) {
		t.Fatalf("stale mutation: %+v", got)
	}
	if w := request(r, "PUT", "f1", `{"expectedRevision":1,"status":"in_progress","annotations":[]}`); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	p, err := s.Progress("v", "b")
	if err != nil || p.Completed != 0 || p.InProgress != 1 {
		t.Fatal(p, err)
	}
	if w := request(r, "PUT", "f1", `{"expectedRevision":2,"status":"completed","annotations":[]}`); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	p, err = s.Progress("v", "b")
	if err != nil || p.Completed != 1 {
		t.Fatal(p, err)
	}
	if got := snapshot(t, r, "f1"); got.Revision != 3 || len(got.Annotations) != 0 || got.Status != "completed" {
		t.Fatal(got)
	}
}

func TestInvalidWritesAreAtomic(t *testing.T) {
	_, r := fixture(t)
	if w := request(r, "PUT", "f1", firstSave); w.Code != 200 {
		t.Fatal(w.Code)
	}
	before := snapshot(t, r, "f1")
	cases := []struct {
		name, body string
		code       int
	}{
		{"negative", `{"expectedRevision":1,"status":"completed","annotations":[{"id":"x","bbox":[-1,0,1,1],"source":"manual"}]}`, 422},
		{"zero", `{"expectedRevision":1,"status":"completed","annotations":[{"id":"x","bbox":[0,0,0,1],"source":"manual"}]}`, 422},
		{"outside", `{"expectedRevision":1,"status":"completed","annotations":[{"id":"x","bbox":[99,0,2,1],"source":"manual"}]}`, 422},
		{"duplicate", `{"expectedRevision":1,"status":"completed","annotations":[{"id":"x","bbox":[0,0,1,1],"source":"manual"},{"id":"x","bbox":[1,1,1,1],"source":"manual"}]}`, 422},
		{"source", `{"expectedRevision":1,"status":"completed","annotations":[{"id":"x","bbox":[0,0,1,1],"source":"ai"}]}`, 422},
		{"status", `{"expectedRevision":1,"status":"reviewed","annotations":[]}`, 422},
		{"missingSource", `{"expectedRevision":1,"status":"completed","annotations":[{"id":"x","bbox":[0,0,1,1]}]}`, 400},
		{"bboxLength", `{"expectedRevision":1,"status":"completed","annotations":[{"id":"x","bbox":[0,0,1,1,2],"source":"manual"}]}`, 422},
		{"missingRevision", `{"status":"completed","annotations":[]}`, 400},
		{"missingAnnotations", `{"expectedRevision":1,"status":"completed"}`, 400},
		{"overflow", `{"expectedRevision":1,"status":"completed","annotations":[{"id":"x","bbox":[0,0,1e999,1],"source":"manual"}]}`, 400},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := request(r, "PUT", "f1", tc.body)
			if w.Code != tc.code {
				t.Fatalf("status %d want %d: %s", w.Code, tc.code, w.Body.String())
			}
			if got := snapshot(t, r, "f1"); !reflect.DeepEqual(got, before) {
				t.Fatal("invalid write mutated snapshot", got)
			}
		})
	}
	if w := request(r, "PUT", "f2", firstSave); w.Code != 422 {
		t.Fatalf("cross-frame ID collision %d: %s", w.Code, w.Body.String())
	}
	if got := snapshot(t, r, "f2"); got.Revision != 0 || got.Status != "not_started" || len(got.Annotations) != 0 {
		t.Fatal("collision mutated frame", got)
	}
	if w := request(r, "GET", "absent", ""); w.Code != 404 {
		t.Fatal(w.Code)
	}
	if w := request(r, "PUT", "absent", firstSave); w.Code != 404 {
		t.Fatal(w.Code)
	}
}

func TestSQLFailureRollsBackReplacement(t *testing.T) {
	s, r := fixture(t)
	if w := request(r, "PUT", "f1", firstSave); w.Code != 200 {
		t.Fatal(w.Code)
	}
	before := snapshot(t, r, "f1")
	_, err := s.DB.Exec(`CREATE TRIGGER fail_annotation BEFORE INSERT ON annotations WHEN NEW.id='reject' BEGIN SELECT RAISE(ABORT,'synthetic failure'); END`)
	if err != nil {
		t.Fatal(err)
	}
	w := request(r, "PUT", "f1", `{"expectedRevision":1,"status":"in_progress","annotations":[{"id":"new","bbox":[1,1,2,2],"source":"manual"},{"id":"reject","bbox":[2,2,2,2],"source":"manual"}]}`)
	if w.Code != 500 {
		t.Fatal(w.Code, w.Body.String())
	}
	if got := snapshot(t, r, "f1"); !reflect.DeepEqual(got, before) {
		t.Fatal("partial transaction", got)
	}
}

func TestConcurrentSameRevisionHasOneWinner(t *testing.T) {
	_, r := fixture(t)
	var wg sync.WaitGroup
	codes := make(chan int, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); codes <- request(r, "PUT", "f1", firstSave).Code }()
	}
	wg.Wait()
	close(codes)
	counts := map[int]int{}
	for c := range codes {
		counts[c]++
	}
	if counts[200] != 1 || counts[409] != 1 {
		t.Fatal(counts)
	}
	if got := snapshot(t, r, "f1"); got.Revision != 1 || len(got.Annotations) != 1 {
		t.Fatal(got)
	}
}
