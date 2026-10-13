package mediaagent

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/XB-Dong/classroom-annotator/backend/internal/media"
	"github.com/XB-Dong/classroom-annotator/backend/internal/mediaapi"
)

type Config struct {
	AgentVersion                                          string
	Root, Token, SourceURL, SourceToken, FFmpeg, FFprobe  string
	Lease                                                 time.Duration
	QueueSize, MaxFrames                                  int
	MaxUploadBytes, MaxControlBytes, MaxArchiveImageBytes int64
	MaxOperation, ArchiveTTL                              time.Duration
	Runner                                                media.Runner
	Dataset                                               Dataset
}
type Dataset interface {
	Lessons(context.Context, int, int) (any, error)
	Download(context.Context, string) (io.ReadCloser, error)
}
type resource struct {
	Path, ContentType, SHA256 string
	SizeBytes                 int64
	Width, Height             int
	DurationSeconds           float64
	ExpiresAt                 time.Time
}
type sourceRecord struct {
	Source   mediaapi.Source
	LegacyID string
}
type operationRecord struct {
	mediaapi.Operation
	LeaseUntil, Deadline time.Time
	Staging              string
}
type manifestState struct {
	Sources    map[string]sourceRecord
	Resources  map[string]resource
	Operations map[string]*operationRecord
}
type task struct {
	id  string
	run func(context.Context) (any, error)
}
type Agent struct {
	cfg                               Config
	mu                                sync.Mutex
	state                             manifestState
	queue                             chan task
	rootctx                           context.Context
	stop                              context.CancelFunc
	wg                                sync.WaitGroup
	running                           map[string]context.CancelFunc
	done                              map[string]chan struct{}
	closed                            bool
	runnerInjected                    bool
	ffmpegAvailable, ffprobeAvailable bool
}

func New(c Config) (*Agent, error) {
	if c.Root == "" || c.Token == "" || !filepath.IsAbs(c.Root) {
		return nil, errors.New("INVALID_CONFIG")
	}
	if c.Lease == 0 {
		c.Lease = 120 * time.Second
	}
	if c.QueueSize == 0 {
		c.QueueSize = 8
	}
	if c.MaxFrames == 0 {
		c.MaxFrames = 1000
	}
	if c.MaxUploadBytes == 0 {
		c.MaxUploadBytes = 1 << 30
	}
	if c.MaxControlBytes == 0 {
		c.MaxControlBytes = 16 << 20
	}
	if c.MaxArchiveImageBytes == 0 {
		c.MaxArchiveImageBytes = 2 << 30
	}
	if c.MaxOperation == 0 {
		c.MaxOperation = 600 * time.Second
	}
	if c.ArchiveTTL == 0 {
		c.ArchiveTTL = time.Hour
	}
	if c.Lease <= 0 || c.QueueSize <= 0 || c.MaxFrames <= 0 || c.MaxUploadBytes <= 0 || c.MaxControlBytes <= 0 || c.MaxArchiveImageBytes <= 0 || c.MaxOperation <= 0 || c.MaxOperation > 600*time.Second || c.ArchiveTTL <= 0 {
		return nil, errors.New("INVALID_CONFIG")
	}
	if c.QueueSize > 8 || c.MaxFrames > 1000 || c.MaxUploadBytes > 1<<30 || c.MaxControlBytes > 16<<20 || c.MaxArchiveImageBytes > 2<<30 {
		return nil, errors.New("INVALID_CONFIG")
	}
	if c.AgentVersion == "" {
		c.AgentVersion = "0.2.0"
	}
	if err := os.MkdirAll(c.Root, 0700); err != nil {
		return nil, errors.New("NO_SPACE")
	}
	for _, d := range []string{"staging", "objects"} {
		if err := os.MkdirAll(filepath.Join(c.Root, d), 0700); err != nil {
			return nil, errors.New("NO_SPACE")
		}
	}
	a := &Agent{cfg: c, runnerInjected: c.Runner != nil, running: map[string]context.CancelFunc{}, done: map[string]chan struct{}{}, queue: make(chan task, c.QueueSize)}
	a.rootctx, a.stop = context.WithCancel(context.Background())
	a.state = manifestState{Sources: map[string]sourceRecord{}, Resources: map[string]resource{}, Operations: map[string]*operationRecord{}}
	if b, err := os.ReadFile(filepath.Join(c.Root, "manifest.json")); err == nil {
		if json.Unmarshal(b, &a.state) != nil || a.state.Sources == nil || a.state.Resources == nil || a.state.Operations == nil {
			return nil, errors.New("INVALID_MANIFEST")
		}
	} else if !os.IsNotExist(err) {
		return nil, errors.New("INVALID_MANIFEST")
	}
	if c.Runner == nil {
		a.cfg.Runner = media.Process{FFmpeg: c.FFmpeg, FFprobe: c.FFprobe}
	}
	a.ffmpegAvailable = toolAvailable(c.FFmpeg)
	a.ffprobeAvailable = toolAvailable(c.FFprobe)
	if err := a.recover(); err != nil {
		return nil, err
	}
	a.wg.Add(2)
	go a.worker()
	go a.sweeper()
	return a, nil
}
func (a *Agent) Close() error {
	a.mu.Lock()
	a.closed = true
	a.stop()
	for _, o := range a.state.Operations {
		if o.Status == "queued" {
			a.finishLocked(o, "failed", "PROCESS_FAILED", nil)
		}
	}
	a.mu.Unlock()
	a.wg.Wait()
	a.mu.Lock()
	for _, o := range a.state.Operations {
		if o.Status == "queued" {
			a.finishLocked(o, "failed", "PROCESS_FAILED", nil)
		}
	}
	a.mu.Unlock()
	return nil
}
func (a *Agent) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if r.URL.Path == "/health/live" && r.Method == "GET" {
		a.json(w, 200, map[string]string{"status": "ok"})
		return
	}
	expected := "Bearer " + a.cfg.Token
	if subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), []byte(expected)) != 1 {
		a.fail(w, 401, "UNAUTHORIZED")
		return
	}
	a.route(w, r)
}
func (a *Agent) json(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func (a *Agent) fail(w http.ResponseWriter, status int, code string) {
	a.json(w, status, mediaapi.Error{Code: code, Message: code, RequestID: opaqueID()})
}
func (a *Agent) decode(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, a.cfg.MaxControlBytes)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	err := d.Decode(v)
	if err == nil {
		var tail any
		if d.Decode(&tail) != io.EOF {
			err = errors.New("INVALID_REQUEST")
		}
	}
	if err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			a.fail(w, 413, "INVALID_REQUEST")
		} else {
			a.fail(w, 400, "INVALID_REQUEST")
		}
		return false
	}
	return true
}
