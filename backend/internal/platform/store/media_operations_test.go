package store

import "testing"

func TestMediaOperationValidation(t *testing.T) {
	good := MediaOperation{OperationID: "op-1", OperationType: "extraction", JobID: "job-1", ResourceID: "batch-1", Provider: "mac", RequestFingerprint: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Status: "queued"}
	if err := validateMediaOperation(good); err != nil {
		t.Fatal(err)
	}
	bad := good
	bad.RequestFingerprint = "secret payload"
	if err := validateMediaOperation(bad); err == nil {
		t.Fatal("accepted unsafe fingerprint")
	}
	bad = good
	bad.JobID = ""
	if err := validateMediaOperation(bad); err == nil {
		t.Fatal("accepted extraction without job")
	}
	bad = good
	bad.OperationType = "upload"
	bad.JobID = ""
	bad.ResourceID = "video-1"
	if err := validateMediaOperation(bad); err != nil {
		t.Fatal(err)
	}
}
