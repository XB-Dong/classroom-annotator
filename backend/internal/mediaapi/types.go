// Package mediaapi contains the versioned wire types shared by the media agent and bridge client.
package mediaapi

import "encoding/json"

const ContractVersion = "1.0.0"

type Capabilities struct {
	ContractVersion      string `json:"contractVersion"`
	AgentVersion         string `json:"agentVersion"`
	MediaRootWritable    bool   `json:"mediaRootWritable"`
	FFmpegAvailable      bool   `json:"ffmpegAvailable"`
	FFprobeAvailable     bool   `json:"ffprobeAvailable"`
	DatasetConfigured    bool   `json:"datasetConfigured"`
	SourceConfigured     bool   `json:"sourceConfigured"`
	MaxUploadBytes       int64  `json:"maxUploadBytes"`
	MaxFrames            int    `json:"maxFrames"`
	LeaseSeconds         int    `json:"leaseSeconds"`
	QueueSize            int    `json:"queueSize"`
	MaxOperationSeconds  int    `json:"maxOperationSeconds"`
	ArchiveTtlSeconds    int    `json:"archiveTtlSeconds"`
	MaxControlBytes      int64  `json:"maxControlBytes"`
	MaxArchiveImageBytes int64  `json:"maxArchiveImageBytes"`
}
type Source struct {
	SourceID        string  `json:"sourceId"`
	MediaKind       string  `json:"mediaKind"`
	Name            string  `json:"name"`
	Width           int     `json:"width"`
	Height          int     `json:"height"`
	DurationSeconds float64 `json:"durationSeconds"`
	SHA256          string  `json:"sha256"`
	SizeBytes       int64   `json:"sizeBytes"`
}
type Sources struct {
	ContractVersion string   `json:"contractVersion"`
	Items           []Source `json:"items"`
	NextCursor      *string  `json:"nextCursor"`
}
type RegistrationRequest struct {
	OperationID string `json:"operationId"`
	VideoID     string `json:"videoId"`
	SourceID    string `json:"sourceId"`
}
type Registration struct {
	OperationID     string  `json:"operationId"`
	VideoID         string  `json:"videoId"`
	MediaKey        string  `json:"mediaKey"`
	SHA256          string  `json:"sha256"`
	SizeBytes       int64   `json:"sizeBytes"`
	Width           int     `json:"width"`
	Height          int     `json:"height"`
	DurationSeconds float64 `json:"durationSeconds"`
}

// UploadIntent is the request identity known before the multipart body is read.
type UploadIntent struct {
	VideoID string `json:"videoId"`
	Name    string `json:"name"`
}
type ExtractionRequest struct {
	OperationID     string  `json:"operationId"`
	JobID           string  `json:"jobId"`
	BatchID         string  `json:"batchId"`
	VideoID         string  `json:"videoId"`
	MediaKey        string  `json:"mediaKey"`
	TimestampsMs    []int64 `json:"timestampsMs"`
	DeadlineSeconds int     `json:"deadlineSeconds"`
}
type Frame struct {
	FrameID     string `json:"frameId"`
	TimestampMs int64  `json:"timestampMs"`
	Width       int    `json:"width"`
	Height      int    `json:"height"`
	MediaKey    string `json:"mediaKey"`
	SHA256      string `json:"sha256"`
	SizeBytes   int64  `json:"sizeBytes"`
}
type ExtractionManifest struct {
	BatchID string  `json:"batchId"`
	Frames  []Frame `json:"frames"`
}
type ArchiveImage struct {
	FrameID   string `json:"frameId"`
	MediaKey  string `json:"mediaKey"`
	SHA256    string `json:"sha256"`
	SizeBytes int64  `json:"sizeBytes"`
	Width     int    `json:"width"`
	Height    int    `json:"height"`
}
type ArchiveRequest struct {
	OperationID string          `json:"operationId"`
	Images      []ArchiveImage  `json:"images"`
	COCO        json.RawMessage `json:"coco"`
}
type ArchiveManifest struct {
	MediaKey  string `json:"mediaKey"`
	SHA256    string `json:"sha256"`
	SizeBytes int64  `json:"sizeBytes"`
	ExpiresAt string `json:"expiresAt"`
}
type DatasetImportRequest struct {
	OperationID   string `json:"operationId"`
	JobID         string `json:"jobId"`
	VideoID       string `json:"videoId"`
	CatalogFileID string `json:"catalogFileId"`
	Name          string `json:"name"`
}
type Operation struct {
	ContractVersion     string          `json:"contractVersion"`
	OperationID         string          `json:"operationId"`
	OperationType       string          `json:"operationType"`
	JobID               *string         `json:"jobId"`
	ResourceID          *string         `json:"resourceId"`
	RequestFingerprint  string          `json:"requestFingerprint"`
	PublicationStatus   string          `json:"publicationStatus"`
	PublicationRevision *int64          `json:"publicationRevision"`
	Status              string          `json:"status"`
	Progress            float64         `json:"progress"`
	ErrorCode           *string         `json:"errorCode"`
	Manifest            json.RawMessage `json:"manifest,omitempty"`
}
type AckRequest struct {
	PublicationRevision int64 `json:"publicationRevision"`
}
type Error struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"requestId"`
}
