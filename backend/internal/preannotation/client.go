package preannotation

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"mime/multipart"
	"net/http"
	"net/url"
	"time"
)

var ErrInvalidDetection = errors.New("invalid detection response")
var ErrUnavailable = errors.New("AI unavailable")

type Client struct{ URL, Token string }

func (c Client) Detect(ctx context.Context, image []byte, width, height int, requestID string) (DetectionResult, error) {
	if c.URL == "" || c.Token == "" || len(image) == 0 || len(image) > 15*1024*1024 {
		return DetectionResult{}, ErrUnavailable
	}
	u, e := url.Parse(c.URL)
	if e != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Scheme != "http" && u.Scheme != "https") {
		return DetectionResult{}, ErrUnavailable
	}
	if u.Path != "" && u.Path != "/" {
		return DetectionResult{}, ErrUnavailable
	}
	u.Path = "/v1/detections"
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, e := writer.CreateFormFile("image", "frame.jpg")
	if e != nil {
		return DetectionResult{}, ErrUnavailable
	}
	part.Write(image)
	writer.WriteField("confidenceThreshold", "0.25")
	writer.WriteField("nmsThreshold", "0.5")
	writer.Close()
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	req, e := http.NewRequestWithContext(ctx, "POST", u.String(), &body)
	if e != nil {
		return DetectionResult{}, ErrUnavailable
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("X-Request-ID", requestID)
	client := http.Client{Timeout: 30 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	response, e := client.Do(req)
	if e != nil {
		return DetectionResult{}, ErrUnavailable
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		return DetectionResult{}, ErrUnavailable
	}
	raw, e := io.ReadAll(io.LimitReader(response.Body, 2*1024*1024+1))
	if e != nil || len(raw) > 2*1024*1024 {
		return DetectionResult{}, ErrInvalidDetection
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var result DetectionResult
	if dec.Decode(&result) != nil {
		return DetectionResult{}, ErrInvalidDetection
	}
	var extra any
	if dec.Decode(&extra) != io.EOF {
		return DetectionResult{}, ErrInvalidDetection
	}
	if result.ContractVersion != "1.0.0" || result.ModelSHA256 != ModelSHA256 || result.PipelineVersion != PipelineVersion || result.Image.Width != width || result.Image.Height != height || len(result.Detections) > MaxCandidates || !finite(result.TimingMs) || result.TimingMs < 0 {
		return DetectionResult{}, ErrInvalidDetection
	}
	for _, d := range result.Detections {
		if d.ClassID != 7 || !finite(d.Confidence) || d.Confidence < .25 || d.Confidence > 1 || !ValidBBox(d.BBox, width, height) {
			return DetectionResult{}, ErrInvalidDetection
		}
	}
	if result.Detections == nil {
		result.Detections = []Detection{}
	}
	return result, nil
}
func finite(n float64) bool { return !math.IsNaN(n) && !math.IsInf(n, 0) }
func ValidBBox(b [4]float64, width, height int) bool {
	for _, n := range b {
		if !finite(n) {
			return false
		}
	}
	return width > 0 && height > 0 && b[0] >= 0 && b[1] >= 0 && b[2] > 0 && b[3] > 0 && b[0]+b[2] <= float64(width) && b[1]+b[3] <= float64(height)
}
