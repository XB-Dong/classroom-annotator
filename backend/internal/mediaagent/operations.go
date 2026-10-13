package mediaagent

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/XB-Dong/classroom-annotator/backend/internal/mediaapi"
)

func opaqueID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic("random source unavailable")
	}
	return hex.EncodeToString(b[:])
}
func validID(s string) bool {
	return len(s) > 0 && len(s) <= 128 && strings.Trim(s, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-_") == ""
}
func fingerprint(v any) string {
	b, _ := json.Marshal(v)
	var normalized any
	decoder := json.NewDecoder(bytes.NewReader(b))
	decoder.UseNumber()
	if decoder.Decode(&normalized) == nil {
		b, _ = json.Marshal(normalized)
	}
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}
func ptr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
func (a *Agent) persistLocked() error {
	b, err := json.Marshal(a.state)
	if err != nil {
		return errors.New("PROCESS_FAILED")
	}
	f, err := os.CreateTemp(a.cfg.Root, "manifest-*.tmp")
	if err != nil {
		return errors.New("NO_SPACE")
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if _, err = f.Write(b); err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err == nil {
		err = os.Rename(tmp, filepath.Join(a.cfg.Root, "manifest.json"))
	}
	if err != nil {
		return errors.New("NO_SPACE")
	}
	return nil
}
func (a *Agent) recover() error {
	for key, r := range a.state.Resources {
		if !validID(key) || !safeRelative(r.Path) {
			return errors.New("INVALID_MANIFEST")
		}
	}
	for id, o := range a.state.Operations {
		if !validID(id) || o.OperationID != id {
			return errors.New("INVALID_MANIFEST")
		}
		if o.Staging != "" && !safeRelative(o.Staging) {
			return errors.New("INVALID_MANIFEST")
		}
		interrupted := o.Status == "running" || o.Status == "queued"
		if interrupted {
			o.Status = "failed"
			o.ErrorCode = ptr("PROCESS_FAILED")
		}
		if o.Staging != "" {
			if err := a.cleanup(o.Staging); err != nil {
				o.Status = "failed"
				o.ErrorCode = ptr("CLEANUP_FAILED")
			} else {
				o.Staging = ""
			}
		}
	}
	return a.persistLocked()
}
func safeRelative(s string) bool {
	if s == "" || filepath.IsAbs(s) || strings.Contains(s, "\\") {
		return false
	}
	clean := filepath.ToSlash(filepath.Clean(s))
	return clean == s && !strings.HasPrefix(s, "../") && s != ".."
}
func (a *Agent) cleanup(rel string) error {
	if rel == "" {
		return nil
	}
	if !safeRelative(rel) || !strings.HasPrefix(rel, "staging/") {
		return errors.New("CLEANUP_FAILED")
	}
	p, err := a.safePath(rel)
	if err != nil {
		return errors.New("CLEANUP_FAILED")
	}
	if err = os.RemoveAll(p); err != nil {
		return errors.New("CLEANUP_FAILED")
	}
	return nil
}
func (a *Agent) safePath(rel string) (string, error) {
	if !safeRelative(rel) {
		return "", errors.New("RESOURCE_NOT_FOUND")
	}
	p := filepath.Join(a.cfg.Root, filepath.FromSlash(rel))
	root, err := filepath.EvalSymlinks(a.cfg.Root)
	if err != nil {
		return "", errors.New("RESOURCE_NOT_FOUND")
	}
	current := a.cfg.Root
	for _, part := range strings.Split(rel, "/") {
		current = filepath.Join(current, part)
		info, e := os.Lstat(current)
		if os.IsNotExist(e) {
			break
		}
		if e != nil || info.Mode()&os.ModeSymlink != 0 {
			return "", errors.New("RESOURCE_NOT_FOUND")
		}
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", errors.New("RESOURCE_NOT_FOUND")
	}
	relative, err := filepath.Rel(root, abs)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", errors.New("RESOURCE_NOT_FOUND")
	}
	return abs, nil
}
func (a *Agent) submit(id, kind, job, resourceID string, request any, staging string, deadline time.Duration, run func(context.Context) (any, error)) (mediaapi.Operation, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	fp := fingerprint(request)
	if previous, ok := a.state.Operations[id]; ok {
		if previous.RequestFingerprint != fp || previous.OperationType != kind {
			return mediaapi.Operation{}, errors.New("OPERATION_CONFLICT")
		}
		return previous.Operation, nil
	}
	if a.closed {
		return mediaapi.Operation{}, errors.New("PROCESS_FAILED")
	}
	if len(a.queue) >= cap(a.queue) {
		return mediaapi.Operation{}, errors.New("QUEUE_FULL")
	}
	if deadline <= 0 || deadline > a.cfg.MaxOperation {
		deadline = a.cfg.MaxOperation
	}
	now := time.Now()
	op := &operationRecord{Operation: mediaapi.Operation{ContractVersion: mediaapi.ContractVersion, OperationID: id, OperationType: kind, JobID: ptr(job), ResourceID: ptr(resourceID), RequestFingerprint: fp, PublicationStatus: "unpublished", Status: "queued"}, LeaseUntil: now.Add(a.cfg.Lease), Deadline: now.Add(deadline), Staging: staging}
	a.state.Operations[id] = op
	a.done[id] = make(chan struct{})
	if err := a.persistLocked(); err != nil {
		delete(a.state.Operations, id)
		delete(a.done, id)
		return mediaapi.Operation{}, err
	}
	a.queue <- task{id, run}
	return op.Operation, nil
}
func (a *Agent) existing(id, kind string, request any) (mediaapi.Operation, bool, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	o, ok := a.state.Operations[id]
	if !ok {
		return mediaapi.Operation{}, false, nil
	}
	if o.OperationType != kind || o.RequestFingerprint != fingerprint(request) {
		return mediaapi.Operation{}, true, errors.New("OPERATION_CONFLICT")
	}
	return o.Operation, true, nil
}
func (a *Agent) worker() {
	defer a.wg.Done()
	for {
		select {
		case <-a.rootctx.Done():
			return
		case t := <-a.queue:
			a.execute(t)
		}
	}
}
func (a *Agent) execute(t task) {
	a.mu.Lock()
	o := a.state.Operations[t.id]
	if o.Status != "queued" {
		a.mu.Unlock()
		return
	}
	now := time.Now()
	if now.After(o.LeaseUntil) || now.After(o.Deadline) {
		a.finishLocked(o, "failed", "TIMEOUT", nil)
		a.mu.Unlock()
		return
	}
	ctx, cancel := context.WithDeadline(a.rootctx, o.Deadline)
	a.running[t.id] = cancel
	o.Status = "running"
	if err := a.persistLocked(); err != nil {
		cancel()
		delete(a.running, t.id)
		a.finishLocked(o, "failed", "NO_SPACE", nil)
		a.mu.Unlock()
		return
	}
	a.mu.Unlock()
	result, err := t.run(ctx)
	a.mu.Lock()
	defer a.mu.Unlock()
	contextErr := ctx.Err()
	cancel()
	delete(a.running, t.id)
	if contextErr != nil {
		if errors.Is(contextErr, context.DeadlineExceeded) || (o.ErrorCode != nil && *o.ErrorCode == "TIMEOUT") {
			a.finishLocked(o, "failed", "TIMEOUT", nil)
		} else if a.closed {
			a.finishLocked(o, "failed", "PROCESS_FAILED", nil)
		} else {
			a.finishLocked(o, "cancelled", "", nil)
		}
		return
	}
	if err != nil {
		a.finishLocked(o, "failed", safeError(err), nil)
		return
	}
	a.finishLocked(o, "completed", "", result)
}
func safeError(err error) string {
	if errors.Is(err, errUploadLimit) {
		return "INVALID_REQUEST"
	}
	switch err.Error() {
	case "INVALID_REQUEST", "RESOURCE_NOT_FOUND", "RESOURCE_CHANGED", "OPERATION_CONFLICT", "QUEUE_FULL", "NO_SPACE", "TIMEOUT", "CLEANUP_FAILED":
		return err.Error()
	default:
		return "PROCESS_FAILED"
	}
}
func (a *Agent) finishLocked(o *operationRecord, status, code string, result any) {
	var addedKeys []string
	if status == "completed" {
		if prepared, ok := result.(preparedOutput); ok {
			dir := "objects/" + opaqueID()
			from, e := a.safePath(o.Staging)
			to, e2 := a.safePath(dir)
			if e != nil || e2 != nil || os.Rename(from, to) != nil {
				status = "failed"
				code = "NO_SPACE"
			} else {
				o.Staging = ""
				for key, r := range prepared.Resources {
					r.Path = dir + "/" + r.Path
					a.state.Resources[key] = r
					addedKeys = append(addedKeys, key)
				}
				result = prepared.Manifest
			}
		}
	}
	if err := a.cleanup(o.Staging); err != nil {
		status = "failed"
		code = "CLEANUP_FAILED"
	} else {
		o.Staging = ""
	}
	o.Status = status
	o.ErrorCode = ptr(code)
	if status == "completed" {
		o.Progress = 1
		o.Manifest, _ = json.Marshal(result)
	}
	if err := a.persistLocked(); err != nil {
		o.Status = "failed"
		o.ErrorCode = ptr("NO_SPACE")
		o.Manifest = nil
		o.Progress = 0
		for _, key := range addedKeys {
			delete(a.state.Resources, key)
		}
	}
	if ch := a.done[o.OperationID]; ch != nil {
		close(ch)
		delete(a.done, o.OperationID)
	}
}
func (a *Agent) sweeper() {
	defer a.wg.Done()
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-a.rootctx.Done():
			return
		case now := <-ticker.C:
			a.mu.Lock()
			for id, o := range a.state.Operations {
				if o.Status == "failed" && o.Staging != "" {
					if a.cleanup(o.Staging) == nil {
						o.Staging = ""
						_ = a.persistLocked()
					}
				}
				if (o.Status == "queued" || o.Status == "running") && (now.After(o.LeaseUntil) || now.After(o.Deadline)) {
					if stop := a.running[id]; stop != nil {
						o.ErrorCode = ptr("TIMEOUT")
						stop()
					} else {
						a.finishLocked(o, "failed", "TIMEOUT", nil)
					}
				}
			}
			for key, r := range a.state.Resources {
				if r.ContentType == "application/zip" && !r.ExpiresAt.IsZero() && now.After(r.ExpiresAt) {
					if p, e := a.safePath(r.Path); e == nil {
						if e = os.Remove(p); e == nil || os.IsNotExist(e) {
							delete(a.state.Resources, key)
							_ = a.persistLocked()
						}
					}
				}
			}
			a.mu.Unlock()
		}
	}
}
