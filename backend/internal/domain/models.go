package domain

import (
	"github.com/google/uuid"
	"time"
)

func ID() string  { return uuid.NewString() }
func Now() string { return time.Now().UTC().Format(time.RFC3339Nano) }

type Video struct {
	ID              string  `json:"id"`
	Name            string  `json:"name"`
	Width           int     `json:"width"`
	Height          int     `json:"height"`
	DurationSeconds float64 `json:"durationSeconds"`
	Status          string  `json:"status"`
	CreatedAt       string  `json:"createdAt"`
	StorageKey      string  `json:"-"`
	SourceKind      string  `json:"sourceKind"`
	SourceID        string  `json:"-"`
	StorageProvider string  `json:"storageProvider"`
	MediaSHA256     string  `json:"-"`
	MediaSizeBytes  int64   `json:"-"`
}
type Job struct {
	ID        string  `json:"id"`
	Kind      string  `json:"kind"`
	VideoID   string  `json:"videoId,omitempty"`
	BatchID   string  `json:"batchId,omitempty"`
	Status    string  `json:"status"`
	Progress  float64 `json:"progress"`
	Error     string  `json:"error,omitempty"`
	CreatedAt string  `json:"createdAt"`
}
type Batch struct {
	ID         string `json:"id"`
	VideoID    string `json:"videoId"`
	JobID      string `json:"jobId"`
	CreatedAt  string `json:"createdAt"`
	FrameCount int    `json:"frameCount"`
}
type Frame struct {
	ID              string `json:"id"`
	VideoID         string `json:"videoId"`
	BatchID         string `json:"batchId"`
	TimestampMs     int64  `json:"timestampMs"`
	Width           int    `json:"width"`
	Height          int    `json:"height"`
	ImageURL        string `json:"imageUrl"`
	Status          string `json:"status"`
	Revision        int64  `json:"revision"`
	StorageKey      string `json:"-"`
	StorageProvider string `json:"storageProvider"`
	MediaSHA256     string `json:"-"`
	MediaSizeBytes  int64  `json:"-"`
}
type Annotation struct {
	ID          string     `json:"id"`
	BBox        [4]float64 `json:"bbox"`
	Source      string     `json:"source"`
	CandidateID string     `json:"candidateId,omitempty"`
	RunID       string     `json:"runId,omitempty"`
}
type Snapshot struct {
	FrameID     string       `json:"frameId"`
	Revision    int64        `json:"revision"`
	Status      string       `json:"status"`
	Annotations []Annotation `json:"annotations"`
	UpdatedBy   string       `json:"updatedBy,omitempty"`
	UpdatedAt   string       `json:"updatedAt,omitempty"`
}
type Progress struct {
	Total      int `json:"total"`
	NotStarted int `json:"notStarted"`
	InProgress int `json:"inProgress"`
	Completed  int `json:"completed"`
}
