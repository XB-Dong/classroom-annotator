package mediaagent

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"time"

	"github.com/XB-Dong/classroom-annotator/backend/internal/mediaapi"
)

func (a *Agent) probeReady() bool { return a.runnerInjected || a.ffprobeAvailable }
func (a *Agent) submitStaged(id, kind, job, resourceID string, request any, stage string, deadline time.Duration, run func(context.Context) (any, error)) (mediaapi.Operation, error) {
	op, e := a.submit(id, kind, job, resourceID, request, stage, deadline, run)
	a.mu.Lock()
	record := a.state.Operations[id]
	owned := record != nil && record.Staging == stage
	a.mu.Unlock()
	if !owned {
		if ce := a.cleanup(stage); ce != nil {
			return mediaapi.Operation{}, ce
		}
	}
	return op, e
}
func (a *Agent) waitPrepared(w http.ResponseWriter, r *http.Request, id string) {
	a.mu.Lock()
	ch := a.done[id]
	a.mu.Unlock()
	if ch != nil {
		select {
		case <-ch:
		case <-r.Context().Done():
			return
		}
	}
	a.mu.Lock()
	op := a.state.Operations[id].Operation
	a.mu.Unlock()
	if op.Status != "completed" {
		code := "PROCESS_FAILED"
		if op.ErrorCode != nil {
			code = *op.ErrorCode
		}
		a.respondError(w, errors.New(code))
		return
	}
	a.json(w, 200, json.RawMessage(op.Manifest))
}
func (a *Agent) registration(w http.ResponseWriter, r *http.Request) {
	var req mediaapi.RegistrationRequest
	if !a.decode(w, r, &req) {
		return
	}
	if !validID(req.OperationID) || !validID(req.VideoID) || !validID(req.SourceID) {
		a.fail(w, 400, "INVALID_REQUEST")
		return
	}
	if _, ok, e := a.existing(req.OperationID, "registration", req); ok {
		if e != nil {
			a.respondError(w, e)
		} else {
			a.waitPrepared(w, r, req.OperationID)
		}
		return
	}
	if !a.probeReady() {
		a.fail(w, 503, "PROCESS_FAILED")
		return
	}
	current, sourceErr := a.refreshSources(r.Context())
	if sourceErr != nil {
		a.respondError(w, sourceErr)
		return
	}
	authorized := false
	for _, s := range current {
		if s.SourceID == req.SourceID {
			authorized = true
			break
		}
	}
	if !authorized {
		a.fail(w, 404, "RESOURCE_NOT_FOUND")
		return
	}
	a.mu.Lock()
	source, ok := a.state.Sources[req.SourceID]
	a.mu.Unlock()
	if !ok || source.Source.MediaKind != "video" {
		a.fail(w, 404, "RESOURCE_NOT_FOUND")
		return
	}
	stage, dir, e := a.stage()
	if e != nil {
		a.respondError(w, e)
		return
	}
	_, e = a.submitStaged(req.OperationID, "registration", "", req.VideoID, req, stage, 0, func(ctx context.Context) (any, error) {
		resp, e := a.legacyRequest(ctx, "/v1/media/"+source.LegacyID)
		if e != nil {
			return nil, e
		}
		defer resp.Body.Close()
		hash, size, e := copyFile(ctx, filepath.Join(dir, "video"), resp.Body, a.cfg.MaxUploadBytes)
		if e != nil {
			return nil, e
		}
		if hash != source.Source.SHA256 || size != source.Source.SizeBytes {
			return nil, errors.New("RESOURCE_CHANGED")
		}
		return a.probeVideo(ctx, stage, req.VideoID, req.OperationID, hash, size)
	})
	if e != nil {
		a.respondError(w, e)
		return
	}
	a.waitPrepared(w, r, req.OperationID)
}
func (a *Agent) upload(w http.ResponseWriter, r *http.Request) {
	id, videoID := r.Header.Get("X-Operation-ID"), r.Header.Get("X-Video-ID")
	if !validID(id) || !validID(videoID) {
		a.fail(w, 400, "INVALID_REQUEST")
		return
	}
	if !a.probeReady() {
		a.fail(w, 503, "PROCESS_FAILED")
		return
	}
	select {
	case a.uploadReceivers <- struct{}{}:
		defer func() { <-a.uploadReceivers }()
	default:
		a.fail(w, 429, "QUEUE_FULL")
		return
	}
	a.mu.Lock()
	if a.closed {
		a.mu.Unlock()
		a.fail(w, 503, "PROCESS_FAILED")
		return
	}
	a.uploadWG.Add(1)
	a.mu.Unlock()
	defer a.uploadWG.Done()
	ctx, cancel := context.WithTimeout(a.rootctx, a.cfg.MaxOperation)
	defer cancel()
	stopRequest := context.AfterFunc(r.Context(), cancel)
	defer stopRequest()
	r.Body = http.MaxBytesReader(w, r.Body, a.cfg.MaxUploadBytes+a.cfg.MaxControlBytes)
	controller := http.NewResponseController(w)
	stopBody := context.AfterFunc(ctx, func() {
		// A net/http body may hold its mutex while blocked in a network Read.
		// Expire the connection read first so Close can then finish the cleanup.
		if err := controller.SetReadDeadline(time.Now().Add(-time.Second)); err != nil {
			// Some response wrappers expose Hijack but not read deadlines.
			// In-memory test writers expose neither; closing their body wakes Read.
			if conn, _, hijackErr := controller.Hijack(); hijackErr == nil {
				_ = conn.Close()
			}
		}
		_ = r.Body.Close()
	})
	defer stopBody()
	reader, e := r.MultipartReader()
	if e != nil {
		a.fail(w, 400, "INVALID_REQUEST")
		return
	}
	part, e := reader.NextPart()
	if e != nil || part.FormName() != "file" || part.FileName() == "" {
		a.fail(w, 400, "INVALID_REQUEST")
		return
	}
	intent := mediaapi.UploadIntent{VideoID: videoID, Name: part.FileName()}
	if _, exists, conflict := a.existing(id, "upload", intent); exists {
		if conflict != nil {
			a.respondError(w, conflict)
			return
		}
		hash, size, err := hashUpload(ctx, part, a.cfg.MaxUploadBytes)
		if err != nil {
			a.respondError(w, err)
			return
		}
		if _, err = reader.NextPart(); err != io.EOF {
			a.fail(w, 400, "INVALID_REQUEST")
			return
		}
		a.mu.Lock()
		record := a.state.Operations[id]
		matches := record != nil && record.ContentSHA256 != "" && record.ContentSHA256 == hash && record.ContentSizeBytes == size
		a.mu.Unlock()
		if !matches {
			a.fail(w, 409, "OPERATION_CONFLICT")
			return
		}
		a.waitPrepared(w, r, id)
		return
	}
	stage, dir, e := a.stage()
	if e != nil {
		a.respondError(w, e)
		return
	}
	if e = a.beginUpload(id, videoID, intent, stage, ctx, cancel); e != nil {
		_ = a.cleanup(stage)
		a.respondError(w, e)
		return
	}
	hash, size, e := copyFile(ctx, filepath.Join(dir, "video"), part, a.cfg.MaxUploadBytes)
	if e != nil {
		a.failUpload(id, ctx, e)
		a.respondError(w, e)
		return
	}
	if _, e = reader.NextPart(); e != io.EOF {
		a.failUpload(id, ctx, errors.New("INVALID_REQUEST"))
		a.fail(w, 400, "INVALID_REQUEST")
		return
	}
	e = a.queueUpload(id, hash, size, ctx, func(ctx context.Context) (any, error) { return a.probeVideo(ctx, stage, videoID, id, hash, size) })
	if e != nil {
		a.respondError(w, e)
		return
	}
	a.waitPrepared(w, r, id)
}
func hashUpload(ctx context.Context, r io.Reader, limit int64) (string, int64, error) {
	h := sha256.New()
	n, err := io.Copy(h, io.LimitReader(contextReader{ctx, r}, limit+1))
	if err != nil {
		return "", n, err
	}
	if n > limit {
		return "", n, errUploadLimit
	}
	if n == 0 {
		return "", n, errors.New("INVALID_REQUEST")
	}
	return hex.EncodeToString(h.Sum(nil)), n, nil
}
func (a *Agent) beginUpload(id, videoID string, intent mediaapi.UploadIntent, stage string, ctx context.Context, cancel context.CancelFunc) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closed {
		return errors.New("PROCESS_FAILED")
	}
	if previous := a.state.Operations[id]; previous != nil {
		return errors.New("OPERATION_CONFLICT")
	}
	deadline, _ := ctx.Deadline()
	now := time.Now()
	lease := now.Add(a.cfg.Lease)
	if lease.After(deadline) {
		lease = deadline
	}
	o := &operationRecord{Operation: mediaapi.Operation{ContractVersion: mediaapi.ContractVersion, OperationID: id, OperationType: "upload", ResourceID: ptr(videoID), RequestFingerprint: fingerprint(intent), PublicationStatus: "unpublished", Status: "running"}, LeaseUntil: lease, Deadline: deadline, Staging: stage}
	a.state.Operations[id] = o
	a.done[id] = make(chan struct{})
	a.running[id] = cancel
	if err := a.persistLocked(); err != nil {
		delete(a.state.Operations, id)
		delete(a.done, id)
		delete(a.running, id)
		return err
	}
	return nil
}
func (a *Agent) failUpload(id string, ctx context.Context, err error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	o := a.state.Operations[id]
	if o == nil || o.Status != "running" {
		return
	}
	delete(a.running, id)
	if ctx.Err() != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) || (o.ErrorCode != nil && *o.ErrorCode == "TIMEOUT") {
			a.finishLocked(o, "failed", "TIMEOUT", nil)
		} else if a.closed {
			a.finishLocked(o, "failed", "PROCESS_FAILED", nil)
		} else {
			a.finishLocked(o, "cancelled", "", nil)
		}
	} else {
		a.finishLocked(o, "failed", safeError(err), nil)
	}
}
func (a *Agent) queueUpload(id, hash string, size int64, receiverCtx context.Context, run func(context.Context) (any, error)) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	o := a.state.Operations[id]
	if o == nil || o.Status != "running" || receiverCtx.Err() != nil || time.Now().After(o.LeaseUntil) || time.Now().After(o.Deadline) {
		responseErr := errors.New("TIMEOUT")
		if o != nil && o.Status == "running" {
			if a.closed {
				a.finishLocked(o, "failed", "PROCESS_FAILED", nil)
				responseErr = errors.New("PROCESS_FAILED")
			} else if errors.Is(receiverCtx.Err(), context.Canceled) && (o.ErrorCode == nil || *o.ErrorCode != "TIMEOUT") {
				a.finishLocked(o, "cancelled", "", nil)
				responseErr = errors.New("OPERATION_CONFLICT")
			} else {
				a.finishLocked(o, "failed", "TIMEOUT", nil)
			}
		}
		return responseErr
	}
	delete(a.running, id)
	o.ContentSHA256, o.ContentSizeBytes = hash, size
	o.Status = "queued"
	if err := a.persistLocked(); err != nil {
		a.finishLocked(o, "failed", "NO_SPACE", nil)
		return err
	}
	select {
	case a.queue <- task{id, run}:
		return nil
	default:
		a.finishLocked(o, "failed", "QUEUE_FULL", nil)
		return errors.New("QUEUE_FULL")
	}
}
func (a *Agent) extraction(w http.ResponseWriter, r *http.Request) {
	var req mediaapi.ExtractionRequest
	if !a.decode(w, r, &req) {
		return
	}
	if !validID(req.OperationID) || !validID(req.JobID) || !validID(req.BatchID) || !validID(req.VideoID) || !validID(req.MediaKey) || len(req.TimestampsMs) == 0 || len(req.TimestampsMs) > a.cfg.MaxFrames || req.DeadlineSeconds <= 0 || time.Duration(req.DeadlineSeconds)*time.Second > a.cfg.MaxOperation {
		a.fail(w, 400, "INVALID_REQUEST")
		return
	}
	if op, ok, e := a.existing(req.OperationID, "extraction", req); ok {
		if e != nil {
			a.respondError(w, e)
		} else {
			a.json(w, 202, op)
		}
		return
	}
	if !a.probeReady() || (!a.runnerInjected && !a.ffmpegAvailable) {
		a.fail(w, 503, "PROCESS_FAILED")
		return
	}
	video, _, e := a.getResource(req.MediaKey)
	if e != nil {
		a.respondError(w, e)
		return
	}
	if video.ContentType != "video/mp4" {
		a.fail(w, 400, "INVALID_REQUEST")
		return
	}
	last := int64(-1)
	for _, ms := range req.TimestampsMs {
		if ms < 0 || ms <= last || float64(ms) >= video.DurationSeconds*1000 {
			a.fail(w, 400, "INVALID_REQUEST")
			return
		}
		last = ms
	}
	stage, _, e := a.stage()
	if e != nil {
		a.respondError(w, e)
		return
	}
	op, e := a.submitStaged(req.OperationID, "extraction", req.JobID, req.BatchID, req, stage, time.Duration(req.DeadlineSeconds)*time.Second, func(ctx context.Context) (any, error) { return a.extract(ctx, req, stage) })
	if e != nil {
		a.respondError(w, e)
		return
	}
	a.json(w, 202, op)
}
