package exports

import (
	"encoding/json"
	"testing"
)

func TestExportHTTPStatusMatrix(t *testing.T) {
	_, r, _ := fixture(t)
	for _, tc := range []struct {
		name, body string
		status     int
	}{
		{"missingSelector", `{}`, 400},
		{"emptyFrameList", `{"frameIds":[]}`, 400},
		{"mixedVideoAndFrames", `{"videoId":"v","frameIds":["f1"]}`, 400},
		{"batchWithoutVideo", `{"batchId":"b"}`, 400},
		{"mixedBatchAndFrames", `{"frameIds":["f1"],"batchId":"b"}`, 400},
		{"duplicateFrameIDs", `{"frameIds":["f1","f1"]}`, 400},
		{"emptyFrameID", `{"frameIds":[""]}`, 400},
		{"nullFrameList", `{"frameIds":null}`, 400},
		{"wrongFrameListType", `{"frameIds":"f1"}`, 400},
		{"wrongFrameElementType", `{"frameIds":[1]}`, 400},
		{"nullFrameElement", `{"frameIds":[null]}`, 400},
		{"wrongVideoType", `{"videoId":1}`, 400},
		{"emptyVideoID", `{"videoId":""}`, 400},
		{"nullVideoID", `{"videoId":null}`, 400},
		{"emptyBatchID", `{"videoId":"v","batchId":""}`, 400},
		{"wrongBatchType", `{"videoId":"v","batchId":1}`, 400},
		{"noCompletedFrames", `{"frameIds":["unfinished"]}`, 422},
		{"unknownVideo", `{"videoId":"missing"}`, 404},
		{"unknownFrame", `{"frameIds":["missing"]}`, 404},
		{"batchOwnership", `{"videoId":"v","batchId":"foreign"}`, 404},
		{"validFrameSelection", `{"frameIds":["f1"]}`, 200},
		{"validVideoSelection", `{"videoId":"v"}`, 200},
		{"validBatchSelection", `{"videoId":"v","batchId":"b"}`, 200},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := export(r, tc.body)
			t.Logf("POST /api/v1/exports %s -> %d", tc.body, w.Code)
			if w.Code != tc.status {
				t.Fatalf("status %d want %d: %s", w.Code, tc.status, w.Body.String())
			}
			if tc.status == 400 {
				var body struct {
					Code string `json:"code"`
				}
				if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
					t.Fatal(err)
				}
				if body.Code != "INVALID_REQUEST" {
					t.Fatal("malformed selector error code", body.Code)
				}
			}
			if tc.status != 200 && (w.Header().Get("Content-Type") == "application/zip" || w.Header().Get("Content-Disposition") != "") {
				t.Fatal("ZIP headers sent for rejected selection")
			}
		})
	}
}
