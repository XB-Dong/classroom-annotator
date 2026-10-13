package preannotation

import (
	"bytes"
	"encoding/json"
	"github.com/XB-Dong/classroom-annotator/backend/internal/domain"
	"io"
)

const ModelSHA256 = "a5ece478fc918787258afccd0b313e54319f8f39aeb1ceca11f27dbd269e78bc"
const PipelineVersion = "classroom-head-v1-fullframe-2x2-15"
const RiskPolicyVersion = "classroom-risk-v1"
const MaxCandidates = 1000

type Detection struct {
	BBox         [4]float64 `json:"bbox"`
	Confidence   float64    `json:"confidence"`
	ClassID      int        `json:"classId"`
	Clipped      bool       `json:"clipped"`
	TileBoundary bool       `json:"tileBoundary"`
}

func (d *Detection) UnmarshalJSON(raw []byte) error {
	var wire struct {
		BBox         *[]float64 `json:"bbox"`
		Confidence   *float64   `json:"confidence"`
		ClassID      *int       `json:"classId"`
		Clipped      bool       `json:"clipped"`
		TileBoundary bool       `json:"tileBoundary"`
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if dec.Decode(&wire) != nil {
		return ErrInvalidDetection
	}
	var extra any
	if dec.Decode(&extra) != io.EOF || wire.BBox == nil || len(*wire.BBox) != 4 || wire.Confidence == nil || wire.ClassID == nil {
		return ErrInvalidDetection
	}
	*d = Detection{BBox: [4]float64(*wire.BBox), Confidence: *wire.Confidence, ClassID: *wire.ClassID, Clipped: wire.Clipped, TileBoundary: wire.TileBoundary}
	return nil
}

type DetectionResult struct {
	ContractVersion string `json:"contractVersion"`
	ModelID         string `json:"modelId"`
	ModelSHA256     string `json:"modelSHA256"`
	PipelineVersion string `json:"pipelineVersion"`
	Mode            string `json:"mode"`
	Image           struct {
		Width  int `json:"width"`
		Height int `json:"height"`
	} `json:"image"`
	Detections []Detection `json:"detections"`
	TimingMs   float64     `json:"timingMs"`
}
type Candidate struct {
	ID           string     `json:"id"`
	RunID        string     `json:"runId"`
	BBox         [4]float64 `json:"bbox"`
	Confidence   float64    `json:"confidence"`
	ReviewState  string     `json:"reviewState"`
	RiskReasons  []string   `json:"riskReasons"`
	AnnotationID *string    `json:"annotationId,omitempty"`
}
type Run struct {
	ID              string `json:"id"`
	Status          string `json:"status"`
	ModelID         string `json:"modelId"`
	ModelSHA256     string `json:"modelSHA256"`
	PipelineVersion string `json:"pipelineVersion"`
	BaseRevision    int64  `json:"baseRevision"`
}
type Issue struct {
	ID         string `json:"id"`
	ReasonCode string `json:"reasonCode"`
	Resolved   bool   `json:"resolved"`
}
type Snapshot struct {
	FrameID     string              `json:"frameId"`
	Revision    int64               `json:"revision"`
	Status      string              `json:"status"`
	Annotations []domain.Annotation `json:"annotations"`
	Candidates  []Candidate         `json:"candidates"`
	CurrentRun  *Run                `json:"currentRun"`
	Issues      []Issue             `json:"issues"`
}
type Decision struct {
	CandidateID string      `json:"candidateId"`
	Decision    string      `json:"decision"`
	BBox        *[4]float64 `json:"bbox,omitempty"`
	ReasonCode  string      `json:"reasonCode,omitempty"`
}
type IssueResolution struct {
	ID         string `json:"id"`
	Resolution string `json:"resolution"`
}
type ReviewWrite struct {
	ExpectedRevision int64             `json:"expectedRevision"`
	Decisions        []Decision        `json:"decisions"`
	IssueResolutions []IssueResolution `json:"issueResolutions"`
}
