// Package mediabridge is the cloud-side, private HTTP client for the Mac media agent.
package mediabridge

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/XB-Dong/classroom-annotator/backend/internal/dataset"
	"github.com/XB-Dong/classroom-annotator/backend/internal/mediaapi"
)

const maxControlBytes int64 = 16 << 20

var opaqueKey = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)
var sha256Key = regexp.MustCompile(`^[0-9a-f]{64}$`)
var singleRange = regexp.MustCompile(`^bytes=(?:[0-9]+-[0-9]*|-[0-9]+)$`)

var ErrUnavailable = errors.New("MEDIA_SOURCE_UNAVAILABLE")
var ErrContract = errors.New("BRIDGE_CONTRACT_MISMATCH")
var ErrInvalid = errors.New("INVALID_REQUEST")
var ErrTooLarge = errors.New("UPLOAD_TOO_LARGE")

// Fingerprint uses the same canonical Go JSON encoding as the agent's request digest.
func Fingerprint(value any) string {
	b, _ := json.Marshal(value)
	var normalized any
	d := json.NewDecoder(bytes.NewReader(b))
	d.UseNumber()
	if d.Decode(&normalized) == nil {
		b, _ = json.Marshal(normalized)
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

type RemoteError struct {
	Code   string
	Status int
}

func (e *RemoteError) Error() string { return e.Code }

type Config struct {
	BaseURL          string
	Token            string
	AllowedOrigins   []string
	Timeout          time.Duration
	ControlTimeout   time.Duration
	DownloadTimeout  time.Duration
	MediaIdleTimeout time.Duration
	HTTPClient       *http.Client
}

type Limits struct {
	MaxUploadBytes       int64
	MaxFrames            int
	MaxControlBytes      int64
	MaxArchiveImageBytes int64
}

type Client struct {
	base             *url.URL
	token            string
	http             *http.Client
	download         *http.Client
	controlTimeout   time.Duration
	mediaIdleTimeout time.Duration
}

func privateOrigin(raw string, allowed []string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "http" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Path != "" || u.RawPath != "" {
		return nil, ErrInvalid
	}
	ip := net.ParseIP(u.Hostname())
	if ip == nil || !(ip.IsLoopback() || ip.IsPrivate()) || u.Port() == "" {
		return nil, ErrInvalid
	}
	port, portErr := strconv.Atoi(u.Port())
	if portErr != nil || port < 1 || port > 65535 {
		return nil, ErrInvalid
	}
	if len(allowed) != 0 {
		found := false
		for _, origin := range allowed {
			if origin == u.String() {
				found = true
			}
		}
		if !found {
			return nil, ErrInvalid
		}
	}
	return u, nil
}

func New(cfg Config) (*Client, error) {
	base, err := privateOrigin(cfg.BaseURL, cfg.AllowedOrigins)
	if err != nil || len(cfg.Token) < 16 || strings.ContainsAny(cfg.Token, " \t\r\n") {
		return nil, ErrInvalid
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 600 * time.Second
	}
	if cfg.DownloadTimeout <= 0 {
		cfg.DownloadTimeout = 1800 * time.Second
	}
	if cfg.ControlTimeout <= 0 {
		cfg.ControlTimeout = 30 * time.Second
	}
	if cfg.MediaIdleTimeout <= 0 {
		cfg.MediaIdleTimeout = 60 * time.Second
	}
	client := cfg.HTTPClient
	var downloadTransport http.RoundTripper
	if client == nil {
		transport := &http.Transport{Proxy: nil, DialContext: (&net.Dialer{Timeout: 5 * time.Second, KeepAlive: 30 * time.Second}).DialContext, DisableCompression: true}
		client = &http.Client{Transport: transport, Timeout: cfg.Timeout, CheckRedirect: rejectRedirect}
		mediaTransport := transport.Clone()
		mediaTransport.ResponseHeaderTimeout = 60 * time.Second
		downloadTransport = mediaTransport
	} else {
		downloadTransport = client.Transport
	}
	download := &http.Client{Transport: downloadTransport, Timeout: cfg.DownloadTimeout, CheckRedirect: rejectRedirect}
	return &Client{base: base, token: cfg.Token, http: client, download: download, controlTimeout: cfg.ControlTimeout, mediaIdleTimeout: cfg.MediaIdleTimeout}, nil
}

func (c *Client) ControlTimeout() time.Duration { return c.controlTimeout }

func rejectRedirect(*http.Request, []*http.Request) error { return ErrContract }

func (c *Client) request(ctx context.Context, method, path string, body io.Reader) (*http.Request, error) {
	u := *c.base
	parsed, err := url.Parse(path)
	if err != nil || parsed.IsAbs() || parsed.Host != "" || !strings.HasPrefix(parsed.Path, "/bridge/v1/") || parsed.Fragment != "" {
		return nil, ErrInvalid
	}
	u.Path = parsed.Path
	u.RawPath = ""
	u.RawQuery = parsed.RawQuery
	req, err := http.NewRequestWithContext(ctx, method, u.String(), body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")
	return req, nil
}

func (c *Client) json(ctx context.Context, method, path string, in, out any, expected int) error {
	ctx, cancel := context.WithTimeout(ctx, c.controlTimeout)
	defer cancel()
	return c.jsonWithin(ctx, method, path, in, out, expected)
}

func (c *Client) jsonWithin(ctx context.Context, method, path string, in, out any, expected int) error {
	var body io.Reader
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil || int64(len(b)) > maxControlBytes {
			return ErrInvalid
		}
		body = bytes.NewReader(b)
	}
	req, err := c.request(ctx, method, path, body)
	if err != nil {
		return err
	}
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return ErrUnavailable
	}
	defer resp.Body.Close()
	if resp.StatusCode != expected {
		return decodeError(resp)
	}
	if out == nil {
		return nil
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, maxControlBytes+1))
	if err != nil {
		return ErrUnavailable
	}
	if int64(len(b)) > maxControlBytes || json.Unmarshal(b, out) != nil {
		return ErrContract
	}
	return nil
}

func decodeError(resp *http.Response) error {
	var body mediaapi.Error
	_ = json.NewDecoder(io.LimitReader(resp.Body, 4096)).Decode(&body)
	if !safeCode(body.Code) {
		body.Code = "PROCESS_FAILED"
	}
	return &RemoteError{Code: body.Code, Status: resp.StatusCode}
}
func safeCode(code string) bool {
	switch code {
	case "INVALID_REQUEST", "UNAUTHORIZED", "RESOURCE_NOT_FOUND", "RESOURCE_CHANGED", "OPERATION_CONFLICT", "QUEUE_FULL", "NO_SPACE", "PROCESS_FAILED", "TIMEOUT", "CLEANUP_FAILED", "DATASET_NOT_CONFIGURED":
		return true
	default:
		return false
	}
}

func (c *Client) Capabilities(ctx context.Context, want Limits) (mediaapi.Capabilities, error) {
	var got mediaapi.Capabilities
	if err := c.json(ctx, http.MethodGet, "/bridge/v1/capabilities", nil, &got, 200); err != nil {
		return got, err
	}
	if got.ContractVersion != mediaapi.ContractVersion || !got.MediaRootWritable || got.MaxUploadBytes <= 0 || got.MaxFrames <= 0 || got.MaxControlBytes <= 0 || got.MaxControlBytes > maxControlBytes || got.MaxOperationSeconds <= 0 || got.MaxOperationSeconds > 600 || got.LeaseSeconds <= 0 || got.QueueSize <= 0 {
		return got, ErrContract
	}
	if (want.MaxUploadBytes > 0 && got.MaxUploadBytes < want.MaxUploadBytes) || (want.MaxFrames > 0 && got.MaxFrames < want.MaxFrames) || (want.MaxControlBytes > 0 && got.MaxControlBytes < want.MaxControlBytes) || (want.MaxArchiveImageBytes > 0 && got.MaxArchiveImageBytes < want.MaxArchiveImageBytes) {
		return got, ErrContract
	}
	return got, nil
}

func (c *Client) Sources(ctx context.Context, cursor string) (mediaapi.Sources, error) {
	var result mediaapi.Sources
	if cursor != "" && !opaqueKey.MatchString(cursor) {
		return result, ErrInvalid
	}
	path := "/bridge/v1/sources"
	if cursor != "" {
		path += "?cursor=" + url.QueryEscape(cursor)
	}
	if err := c.json(ctx, http.MethodGet, path, nil, &result, 200); err != nil {
		return result, err
	}
	if result.ContractVersion != mediaapi.ContractVersion || result.Items == nil || len(result.Items) > 20 {
		return result, ErrContract
	}
	for _, item := range result.Items {
		if !opaqueKey.MatchString(item.SourceID) || (item.MediaKind != "video" && item.MediaKind != "image") || item.Name == "" || len(item.Name) > 255 || item.SizeBytes <= 0 || !sha256Key.MatchString(item.SHA256) {
			return result, ErrContract
		}
	}
	return result, nil
}

func (c *Client) Registration(ctx context.Context, req mediaapi.RegistrationRequest) (mediaapi.Registration, error) {
	var result mediaapi.Registration
	err := c.jsonWithin(ctx, http.MethodPost, "/bridge/v1/registrations", req, &result, 200)
	if err == nil && !validRegistration(result, req.OperationID, req.VideoID) {
		err = ErrContract
	}
	return result, err
}
func (c *Client) Extraction(ctx context.Context, req mediaapi.ExtractionRequest) (mediaapi.Operation, error) {
	return c.postOperation(ctx, "/bridge/v1/extractions", req)
}
func (c *Client) DatasetImport(ctx context.Context, req mediaapi.DatasetImportRequest) (mediaapi.Operation, error) {
	return c.postOperation(ctx, "/bridge/v1/dataset/imports", req)
}
func (c *Client) Archive(ctx context.Context, req mediaapi.ArchiveRequest) (mediaapi.Operation, error) {
	return c.postOperation(ctx, "/bridge/v1/archives", req)
}
func (c *Client) postOperation(ctx context.Context, path string, req any) (mediaapi.Operation, error) {
	var result mediaapi.Operation
	err := c.json(ctx, http.MethodPost, path, req, &result, 202)
	if err != nil {
		return result, err
	}
	if result.ContractVersion != mediaapi.ContractVersion || !opaqueKey.MatchString(result.OperationID) {
		return result, ErrContract
	}
	return result, nil
}
func (c *Client) Operation(ctx context.Context, id string) (mediaapi.Operation, error) {
	var result mediaapi.Operation
	if !opaqueKey.MatchString(id) {
		return result, ErrInvalid
	}
	err := c.json(ctx, http.MethodGet, "/bridge/v1/operations/"+id, nil, &result, 200)
	if err == nil && (result.ContractVersion != mediaapi.ContractVersion || result.OperationID != id) {
		err = ErrContract
	}
	return result, err
}
func (c *Client) control(ctx context.Context, id, action string, body any) (mediaapi.Operation, error) {
	var result mediaapi.Operation
	if !opaqueKey.MatchString(id) {
		return result, ErrInvalid
	}
	err := c.json(ctx, http.MethodPost, "/bridge/v1/operations/"+id+"/"+action, body, &result, 200)
	if err == nil && (result.ContractVersion != mediaapi.ContractVersion || result.OperationID != id) {
		err = ErrContract
	}
	return result, err
}
func (c *Client) Renew(ctx context.Context, id string) (mediaapi.Operation, error) {
	return c.control(ctx, id, "renew", struct{}{})
}
func (c *Client) Cancel(ctx context.Context, id string) (mediaapi.Operation, error) {
	return c.control(ctx, id, "cancel", struct{}{})
}
func (c *Client) Ack(ctx context.Context, id string, publicationRevision int64) (mediaapi.Operation, error) {
	if publicationRevision <= 0 {
		return mediaapi.Operation{}, ErrInvalid
	}
	return c.control(ctx, id, "ack", mediaapi.AckRequest{PublicationRevision: publicationRevision})
}

type uploadSourceTracker struct {
	io.ReadCloser
	consumed atomic.Bool
}

func (s *uploadSourceTracker) Read(p []byte) (int, error) {
	n, err := s.ReadCloser.Read(p)
	if err == io.EOF {
		s.consumed.Store(true)
	}
	return n, err
}

func (c *Client) Upload(ctx context.Context, operationID, videoID, filename string, source io.ReadCloser, limit int64) (mediaapi.Registration, error) {
	var result mediaapi.Registration
	if !opaqueKey.MatchString(operationID) || !opaqueKey.MatchString(videoID) || source == nil || limit <= 0 || limit > 1<<30 {
		return result, ErrInvalid
	}
	pipeR, pipeW := io.Pipe()
	tracked := &uploadSourceTracker{ReadCloser: source}
	interrupt := sync.OnceFunc(func() {
		if !tracked.consumed.Load() {
			_ = source.Close()
		}
	})
	stopInterrupt := context.AfterFunc(ctx, interrupt)
	defer stopInterrupt()
	writer := multipart.NewWriter(pipeW)
	copyDone := make(chan error, 1)
	go func() {
		defer close(copyDone)
		part, err := writer.CreateFormFile("file", filename)
		if err == nil {
			var n int64
			n, err = io.Copy(part, io.LimitReader(tracked, limit+1))
			if err == nil && n == 0 {
				err = ErrInvalid
			}
			if err == nil && n > limit {
				err = ErrTooLarge
			}
		}
		if err == nil {
			err = writer.Close()
		}
		_ = pipeW.CloseWithError(err)
		copyDone <- err
	}()
	req, err := c.request(ctx, http.MethodPost, "/bridge/v1/uploads", pipeR)
	if err != nil {
		_ = pipeR.Close()
		interrupt()
		<-copyDone
		return result, err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Expect", "100-continue")
	req.Header.Set("X-Operation-ID", operationID)
	req.Header.Set("X-Video-ID", videoID)
	resp, err := c.http.Do(req)
	_ = pipeR.Close()
	interrupt()
	copyErr := <-copyDone
	if err != nil {
		if copyErr != nil {
			return result, copyErr
		}
		return result, ErrUnavailable
	}
	defer resp.Body.Close()
	if copyErr != nil {
		return result, copyErr
	}
	if resp.StatusCode != 200 {
		return result, decodeError(resp)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, maxControlBytes+1))
	if err != nil || int64(len(b)) > maxControlBytes || json.Unmarshal(b, &result) != nil || !validRegistration(result, operationID, videoID) {
		return result, ErrContract
	}
	return result, nil
}

func validRegistration(result mediaapi.Registration, operationID, videoID string) bool {
	return result.OperationID == operationID && result.VideoID == videoID && opaqueKey.MatchString(result.MediaKey) && len(result.SHA256) == 64 && result.SizeBytes > 0 && result.Width > 0 && result.Height > 0 && result.DurationSeconds > 0
}

// OpenMedia returns a streaming response. The caller must close Body and forward only selected safe headers.
func (c *Client) OpenMedia(ctx context.Context, method, key, rangeHeader string) (*http.Response, error) {
	if method != http.MethodGet && method != http.MethodHead {
		return nil, ErrInvalid
	}
	if !opaqueKey.MatchString(key) || (rangeHeader != "" && !singleRange.MatchString(rangeHeader)) {
		return nil, ErrInvalid
	}
	mediaCtx, mediaCancel := context.WithCancel(ctx)
	req, err := c.request(mediaCtx, method, "/bridge/v1/media/"+key, nil)
	if err != nil {
		mediaCancel()
		return nil, err
	}
	if rangeHeader != "" {
		req.Header.Set("Range", rangeHeader)
	}
	resp, err := c.download.Do(req)
	if err != nil {
		mediaCancel()
		return nil, ErrUnavailable
	}
	if resp.StatusCode != 200 && resp.StatusCode != 206 && resp.StatusCode != 416 {
		defer resp.Body.Close()
		defer mediaCancel()
		return nil, decodeError(resp)
	}
	if resp.StatusCode != 416 && resp.Header.Get("Content-Length") == "" {
		resp.Body.Close()
		mediaCancel()
		return nil, fmt.Errorf("%w: missing length", ErrContract)
	}
	resp.Body = newIdleBody(resp.Body, mediaCancel, c.mediaIdleTimeout)
	return resp, nil
}

type DatasetVideo struct {
	ID            dataset.SafeID `json:"id"`
	LessonID      dataset.SafeID `json:"lesson_id"`
	CatalogFileID dataset.SafeID `json:"catalog_file_id"`
	Channel       int            `json:"channel"`
	PartIndex     int            `json:"part_index"`
}
type DatasetLesson struct {
	ID               dataset.SafeID `json:"id"`
	CourseName       string         `json:"course_name"`
	OrganizationName string         `json:"organization_name"`
	Teacher          string         `json:"teacher"`
	Date             string         `json:"date"`
	SectionIndex     int            `json:"section_index"`
	Name             *string        `json:"name,omitempty"`
	Room             *string        `json:"room,omitempty"`
	StartTime        *string        `json:"start_time,omitempty"`
	EndTime          *string        `json:"end_time,omitempty"`
	Videos           []DatasetVideo `json:"videos"`
}
type DatasetPage struct {
	Type  string          `json:"type"`
	Items []DatasetLesson `json:"items"`
	Total int             `json:"total"`
}

func (c *Client) Lessons(ctx context.Context, page, size int) (DatasetPage, error) {
	if page < 1 || size < 1 || size > 100 {
		return DatasetPage{}, ErrInvalid
	}
	var result DatasetPage
	path := fmt.Sprintf("/bridge/v1/dataset/lessons?page=%d&page_size=%d", page, size)
	err := c.json(ctx, http.MethodGet, path, nil, &result, 200)
	if err != nil {
		return DatasetPage{}, err
	}
	if result.Type == "" || result.Items == nil || result.Total < 0 {
		return DatasetPage{}, ErrContract
	}
	return result, nil
}
