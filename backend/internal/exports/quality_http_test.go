package exports

import (
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/XB-Dong/classroom-annotator/backend/internal/annotations"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
)

func qualityRequest(r *gin.Engine, body string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("POST", "/api/v1/exports/quality-check", strings.NewReader(body)))
	return w
}

func TestQualityCheckSnapshotIsReadOnlyAndDoesNotReadMedia(t *testing.T) {
	s, r, images := fixture(t)
	repo := annotations.Repository{Store: s}
	before, err := repo.Get(context.Background(), "f1")
	if err != nil {
		t.Fatal(err)
	}
	var eventsBefore int
	if err = s.DB.QueryRow(`SELECT count(*) FROM annotation_events`).Scan(&eventsBefore); err != nil {
		t.Fatal(err)
	}
	f, err := s.Frame("f1")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(s.Root, f.StorageKey)
	if err = os.Remove(path); err != nil {
		t.Fatal(err)
	}
	w := qualityRequest(r, `{"frameIds":["zero","unfinished","f1"]}`)
	var report QualityReport
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &report) != nil {
		t.Fatal(w.Code, w.Body.String())
	}
	if report.CheckedFrames != 2 || report.CheckedAnnotations != 1 || report.TotalFindings != 1 || report.Findings[0].Code != "TINY_BOX" || report.FrameRevisions[0].FrameID != "f1" {
		t.Fatal(report)
	}
	after, err := repo.Get(context.Background(), "f1")
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatal("quality check changed frame", err)
	}
	var eventsAfter int
	if err = s.DB.QueryRow(`SELECT count(*) FROM annotation_events`).Scan(&eventsAfter); err != nil || eventsAfter != eventsBefore {
		t.Fatal("quality check wrote audit", err)
	}
	entries, err := os.ReadDir(filepath.Join(s.Root, "staging"))
	if err != nil || len(entries) != 0 {
		t.Fatal("quality check created archive", entries, err)
	}
	if err = os.WriteFile(path, images["f1"], 0600); err != nil {
		t.Fatal(err)
	}
	inspectZIP(t, export(r, `{"frameIds":["f1","zero"]}`), images, 2, 1)
	for _, tc := range []struct {
		body string
		code int
	}{
		{`{"frameIds":["unfinished"]}`, 422}, {`{"frameIds":["missing"]}`, 404}, {`{"videoId":"v","batchId":"foreign"}`, 404},
	} {
		if got := qualityRequest(r, tc.body); got.Code != tc.code {
			t.Fatal(tc.body, got.Code, got.Body.String())
		}
	}
	zero := qualityRequest(r, `{"frameIds":["zero"]}`)
	if zero.Code != 200 || !strings.Contains(zero.Body.String(), `"findings":[]`) || !strings.Contains(zero.Body.String(), `"rulesVersion":"P2-QC-1"`) {
		t.Fatal(zero.Code, zero.Body.String())
	}
}

func TestQualityCheckRetainsCandidateIssueAndProvenanceHardGates(t *testing.T) {
	s, r, _ := fixture(t)
	_, err := s.DB.Exec(`
INSERT INTO users(id,username,display_name,password_hash) VALUES('qc-user','qc_user','QC user','unused');
INSERT INTO jobs(id,kind,video_id,status,created_at) VALUES('qc-job','preannotation','v','completed','2026-10-14T00:00:00Z');
INSERT INTO ai_requests(id,workspace_id,scope,request_key,fingerprint,job_id,actor_id) VALUES('qc-request','00000000-0000-4000-8000-000000000001','frame:f1','qc-key','synthetic','qc-job','qc-user');
INSERT INTO ai_runs(id,request_id,job_id,frame_id,base_revision,input_sha256,model_sha256,pipeline_version,risk_policy_version,options,status,actor_id) VALUES('qc-run','qc-request','qc-job','f1',1,'synthetic','synthetic','synthetic','synthetic','{}','completed','qc-user');
INSERT INTO ai_candidates(id,run_id,frame_id,x,y,width,height,confidence,risk_reasons) VALUES('qc-candidate','qc-run','f1',0,0,1,1,.5,'[]');
UPDATE frames SET active_ai_run_id='qc-run' WHERE id='f1';`)
	if err != nil {
		t.Fatal(err)
	}
	assertBlocked := func() {
		t.Helper()
		for _, response := range []*httptest.ResponseRecorder{qualityRequest(r, `{"frameIds":["f1"]}`), export(r, `{"frameIds":["f1"]}`)} {
			if response.Code != 400 || !strings.Contains(response.Body.String(), `"code":"INVALID_REQUEST"`) {
				t.Fatal(response.Code, response.Body.String())
			}
		}
	}
	assertBlocked()
	if _, err = s.DB.Exec(`UPDATE ai_candidates SET review_state='rejected' WHERE id='qc-candidate'; INSERT INTO frame_review_issues(id,run_id,frame_id,reason_code) VALUES('qc-issue','qc-run','f1','AI_FAILED')`); err != nil {
		t.Fatal(err)
	}
	assertBlocked()
	if _, err = s.DB.Exec(`UPDATE frame_review_issues SET resolved=true WHERE id='qc-issue'`); err != nil {
		t.Fatal(err)
	}
	w := qualityRequest(r, `{"frameIds":["f1"]}`)
	var report QualityReport
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &report) != nil || report.CheckedAnnotations != 1 || report.TotalFindings != 1 {
		t.Fatal(w.Code, w.Body.String())
	}
	if _, err = s.DB.Exec(`UPDATE annotations SET source='ai' WHERE frame_id='f1'`); err != nil {
		t.Fatal(err)
	}
	if w = qualityRequest(r, `{"frameIds":["f1"]}`); w.Code == 200 {
		t.Fatal("invalid provenance passed", w.Body.String())
	}
}

func TestQualityCheckRejectsInvalidSelectionWithoutDatabase(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	Register(r.Group("/api/v1"), nil, prometheus.NewRegistry())
	for _, body := range []string{`{}`, `{"frameIds":[]}`, `{"frameIds":["a","a"]}`, `{"videoId":"v","frameIds":["f"]}`, `{"videoId":"v","unknown":true}`} {
		w := qualityRequest(r, body)
		if w.Code != 400 || !strings.Contains(w.Body.String(), `"requestId"`) {
			t.Fatalf("%s: status %d: %s", body, w.Code, w.Body.String())
		}
	}
}

func TestQualityProviderCheckHonorsCancelledRequestBeforeDatabase(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := qualitySelectionProviders(ctx, nil, Selection{FrameIDs: []string{"one", "two"}}); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}
