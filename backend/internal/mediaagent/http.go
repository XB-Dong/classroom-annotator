package mediaagent

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/XB-Dong/classroom-annotator/backend/internal/mediaapi"
)

func toolAvailable(path string) bool {
	if path == "" {
		return false
	}
	resolved, e := exec.LookPath(path)
	if e != nil {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, resolved, "-version")
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	return cmd.Run() == nil
}
func (a *Agent) capabilities() mediaapi.Capabilities {
	writable := false
	if f, e := os.CreateTemp(a.cfg.Root, ".write-check-*"); e == nil {
		f.Close()
		writable = os.Remove(f.Name()) == nil
	}
	return mediaapi.Capabilities{ContractVersion: mediaapi.ContractVersion, AgentVersion: a.cfg.AgentVersion, MediaRootWritable: writable, FFmpegAvailable: a.ffmpegAvailable, FFprobeAvailable: a.ffprobeAvailable, MaxUploadBytes: a.cfg.MaxUploadBytes, MaxFrames: a.cfg.MaxFrames, LeaseSeconds: int(a.cfg.Lease / time.Second), QueueSize: a.cfg.QueueSize, MaxOperationSeconds: int(a.cfg.MaxOperation / time.Second), ArchiveTtlSeconds: int(a.cfg.ArchiveTTL / time.Second), MaxControlBytes: a.cfg.MaxControlBytes, MaxArchiveImageBytes: a.cfg.MaxArchiveImageBytes}
}
func (a *Agent) route(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == "GET" && r.URL.Path == "/bridge/v1/capabilities":
		a.json(w, 200, a.capabilities())
	case r.Method == "GET" && r.URL.Path == "/bridge/v1/sources":
		a.sources(w, r)
	case strings.HasPrefix(r.URL.Path, "/bridge/v1/operations/"):
		a.operation(w, r)
	case (r.Method == "GET" || r.Method == "HEAD") && strings.HasPrefix(r.URL.Path, "/bridge/v1/media/"):
		a.serveMedia(w, r)
	case r.Method == "POST" && r.URL.Path == "/bridge/v1/registrations":
		a.registration(w, r)
	case r.Method == "POST" && r.URL.Path == "/bridge/v1/uploads":
		a.upload(w, r)
	case r.Method == "POST" && r.URL.Path == "/bridge/v1/extractions":
		a.extraction(w, r)
	case r.Method == "POST" && r.URL.Path == "/bridge/v1/archives":
		a.archive(w, r)
	case r.Method == "GET" && r.URL.Path == "/bridge/v1/dataset/lessons":
		a.lessons(w, r)
	case r.Method == "POST" && r.URL.Path == "/bridge/v1/dataset/imports":
		a.datasetImport(w, r)
	default:
		a.fail(w, 404, "RESOURCE_NOT_FOUND")
	}
}
func (a *Agent) operation(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/bridge/v1/operations/"), "/")
	if len(parts) > 2 || !validID(parts[0]) {
		a.fail(w, 404, "RESOURCE_NOT_FOUND")
		return
	}
	action := ""
	if len(parts) == 2 {
		action = parts[1]
	}
	var revision int64
	if r.Method == "POST" {
		if action == "ack" {
			var req mediaapi.AckRequest
			if !a.decode(w, r, &req) {
				return
			}
			revision = req.PublicationRevision
			if revision <= 0 {
				a.fail(w, 400, "INVALID_REQUEST")
				return
			}
		} else {
			var body struct{}
			if !a.decode(w, r, &body) {
				return
			}
		}
	} else if r.Method != "GET" || action != "" {
		a.fail(w, 404, "RESOURCE_NOT_FOUND")
		return
	}
	a.mu.Lock()
	o, ok := a.state.Operations[parts[0]]
	if !ok {
		a.mu.Unlock()
		a.fail(w, 404, "RESOURCE_NOT_FOUND")
		return
	}
	switch action {
	case "":
	case "ack":
		if o.Status != "completed" || (o.PublicationRevision != nil && *o.PublicationRevision != revision) {
			a.mu.Unlock()
			a.fail(w, 409, "OPERATION_CONFLICT")
			return
		}
		previous := o.PublicationRevision
		o.PublicationRevision = &revision
		o.PublicationStatus = "committed"
		if e := a.persistLocked(); e != nil {
			o.PublicationRevision = previous
			if previous == nil {
				o.PublicationStatus = "unpublished"
			}
			a.mu.Unlock()
			a.fail(w, 507, "NO_SPACE")
			return
		}
	case "renew":
		if o.Status == "queued" || o.Status == "running" {
			now := time.Now()
			if now.After(o.LeaseUntil) || now.After(o.Deadline) {
				a.mu.Unlock()
				a.fail(w, 409, "OPERATION_CONFLICT")
				return
			}
			o.LeaseUntil = now.Add(a.cfg.Lease)
			if o.LeaseUntil.After(o.Deadline) {
				o.LeaseUntil = o.Deadline
			}
			if e := a.persistLocked(); e != nil {
				a.mu.Unlock()
				a.fail(w, 507, "NO_SPACE")
				return
			}
		}
	case "cancel":
		if o.Status == "queued" {
			a.finishLocked(o, "cancelled", "", nil)
		} else if o.Status == "running" {
			stop := a.running[o.OperationID]
			ch := a.done[o.OperationID]
			if stop != nil {
				stop()
			}
			a.mu.Unlock()
			select {
			case <-ch:
			case <-r.Context().Done():
				return
			}
			a.mu.Lock()
			o = a.state.Operations[parts[0]]
		}
	default:
		a.mu.Unlock()
		a.fail(w, 404, "RESOURCE_NOT_FOUND")
		return
	}
	response := o.Operation
	a.mu.Unlock()
	a.json(w, 200, response)
}
func (a *Agent) respondError(w http.ResponseWriter, err error) {
	if errors.Is(err, errUploadLimit) {
		a.fail(w, 413, "INVALID_REQUEST")
		return
	}
	code := safeError(err)
	status := 500
	switch code {
	case "INVALID_REQUEST":
		status = 400
	case "RESOURCE_NOT_FOUND":
		status = 404
	case "RESOURCE_CHANGED", "OPERATION_CONFLICT":
		status = 409
	case "QUEUE_FULL":
		status = 429
	case "NO_SPACE":
		status = 507
	case "TIMEOUT":
		status = 504
	}
	a.fail(w, status, code)
}
func (a *Agent) serveMedia(w http.ResponseWriter, r *http.Request) {
	key := strings.TrimPrefix(r.URL.Path, "/bridge/v1/media/")
	res, p, e := a.getResource(key)
	if e != nil {
		a.respondError(w, e)
		return
	}
	f, e := os.Open(p)
	if e != nil {
		a.fail(w, 404, "RESOURCE_NOT_FOUND")
		return
	}
	defer f.Close()
	info, e := f.Stat()
	if e != nil || !info.Mode().IsRegular() || info.Size() != res.SizeBytes {
		a.fail(w, 409, "RESOURCE_CHANGED")
		return
	}
	if strings.Contains(r.Header.Get("Range"), ",") {
		w.Header().Set("Content-Range", "bytes */"+strconv.FormatInt(res.SizeBytes, 10))
		a.fail(w, 416, "INVALID_REQUEST")
		return
	}
	w.Header().Set("Content-Type", res.ContentType)
	w.Header().Set("ETag", `"`+res.SHA256+`"`)
	w.Header().Set("Accept-Ranges", "bytes")
	http.ServeContent(w, r, key, time.Time{}, f)
}
