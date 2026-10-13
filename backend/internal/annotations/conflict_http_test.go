package annotations

import (
	"encoding/json"
	"github.com/XB-Dong/classroom-annotator/backend/internal/domain"
	"github.com/gin-gonic/gin"
	"net/http/httptest"
	"testing"
)

func TestConflictResponseIncludesTraceAndCurrentRevision(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("requestId", "request-trace-123")
	writeConflict(c, domain.Snapshot{Revision: 7, UpdatedBy: "member-123"})
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if w.Code != 409 || body["code"] != "REVISION_CONFLICT" || body["requestId"] != "request-trace-123" || body["currentRevision"] != float64(7) || body["latestUpdatedBy"] != "member-123" {
		t.Fatalf("conflict lost trace or current metadata: %v status=%d", body, w.Code)
	}
}
