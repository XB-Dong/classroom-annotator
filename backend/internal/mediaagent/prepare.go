package mediaagent

import (
	"context"
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
		return a.probeVideo(ctx, stage, req.VideoID, req.OperationID)
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
	ctx, cancel := context.WithTimeout(r.Context(), a.cfg.MaxOperation)
	defer cancel()
	r.Body = http.MaxBytesReader(w, r.Body, a.cfg.MaxUploadBytes+a.cfg.MaxControlBytes)
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
	stage, dir, e := a.stage()
	if e != nil {
		a.respondError(w, e)
		return
	}
	defer func() {
		a.mu.Lock()
		record := a.state.Operations[id]
		owned := record != nil && record.Staging == stage
		a.mu.Unlock()
		if !owned {
			_ = a.cleanup(stage)
		}
	}()
	hash, size, e := copyFile(ctx, filepath.Join(dir, "video"), part, a.cfg.MaxUploadBytes)
	if e != nil {
		a.respondError(w, e)
		return
	}
	if _, e = reader.NextPart(); e != io.EOF {
		a.fail(w, 400, "INVALID_REQUEST")
		return
	}
	canonical := struct {
		VideoID, SHA256, Name string
		SizeBytes             int64
	}{videoID, hash, part.FileName(), size}
	uploadDeadline, _ := ctx.Deadline()
	remaining := time.Until(uploadDeadline)
	if remaining <= 0 {
		a.fail(w, 504, "TIMEOUT")
		return
	}
	_, e = a.submitStaged(id, "upload", "", videoID, canonical, stage, remaining, func(ctx context.Context) (any, error) { return a.probeVideo(ctx, stage, videoID, id) })
	if e != nil {
		a.respondError(w, e)
		return
	}
	a.waitPrepared(w, r, id)
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
