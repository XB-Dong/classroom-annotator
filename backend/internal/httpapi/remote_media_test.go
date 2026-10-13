package httpapi

import (
	"net/http"
	"testing"
)

func TestRemoteMediaLengthRejectsChangedRangeTotal(t *testing.T) {
	response := &http.Response{StatusCode: http.StatusPartialContent, Header: http.Header{"Content-Length": []string{"10"}, "Content-Range": []string{"bytes 0-9/20"}}}
	if !remoteMediaLength(response, 20) {
		t.Fatal("valid single range rejected")
	}
	response.Header.Set("Content-Range", "bytes 0-9/21")
	if remoteMediaLength(response, 20) {
		t.Fatal("changed total accepted")
	}
	response.Header.Set("Content-Range", "bytes 0-8/20")
	if remoteMediaLength(response, 20) {
		t.Fatal("mismatched range length accepted")
	}
}
