package mediaagent

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"image/jpeg"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/XB-Dong/classroom-annotator/backend/internal/mediaapi"
)

func (a *Agent) archive(w http.ResponseWriter, r *http.Request) {
	var req mediaapi.ArchiveRequest
	if !a.decode(w, r, &req) {
		return
	}
	if !validID(req.OperationID) || len(req.Images) == 0 || len(req.Images) > 10000 || !json.Valid(req.COCO) || len(req.COCO) == 0 {
		a.fail(w, 400, "INVALID_REQUEST")
		return
	}
	if op, ok, e := a.existing(req.OperationID, "archive", req); ok {
		if e != nil {
			a.respondError(w, e)
		} else {
			a.json(w, 202, op)
		}
		return
	}
	var total int64
	seen := map[string]bool{}
	for _, im := range req.Images {
		if !validID(im.FrameID) || !validID(im.MediaKey) || !validHash(im.SHA256) || im.SizeBytes <= 0 || im.Width <= 0 || im.Height <= 0 || seen[im.FrameID] || im.SizeBytes > a.cfg.MaxArchiveImageBytes-total {
			a.fail(w, 400, "INVALID_REQUEST")
			return
		}
		seen[im.FrameID] = true
		total += im.SizeBytes
	}
	stage, _, e := a.stage()
	if e != nil {
		a.respondError(w, e)
		return
	}
	op, e := a.submitStaged(req.OperationID, "archive", "", "", req, stage, 0, func(ctx context.Context) (any, error) { return a.buildArchive(ctx, req, stage) })
	if e != nil {
		a.respondError(w, e)
		return
	}
	a.json(w, 202, op)
}
func (a *Agent) buildArchive(ctx context.Context, req mediaapi.ArchiveRequest, stage string) (any, error) {
	dir, e := a.safePath(stage)
	if e != nil {
		return nil, e
	}
	p := filepath.Join(dir, "archive.zip")
	f, e := os.OpenFile(p, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if e != nil {
		return nil, errors.New("NO_SPACE")
	}
	defer f.Close()
	z := zip.NewWriter(f)
	defer z.Close()
	for _, im := range req.Images {
		if e = ctx.Err(); e != nil {
			return nil, e
		}
		res, p, e := a.getResource(im.MediaKey)
		if e != nil {
			return nil, e
		}
		if res.ContentType != "image/jpeg" || res.SHA256 != im.SHA256 || res.SizeBytes != im.SizeBytes || res.Width != im.Width || res.Height != im.Height {
			return nil, errors.New("RESOURCE_CHANGED")
		}
		if e = copyArchiveImage(ctx, z, p, im); e != nil {
			return nil, e
		}
	}
	entry, e := z.Create("annotations.json")
	if e != nil {
		return nil, errors.New("PROCESS_FAILED")
	}
	if _, e = entry.Write(req.COCO); e != nil {
		return nil, errors.New("NO_SPACE")
	}
	if e = z.Close(); e != nil {
		return nil, errors.New("NO_SPACE")
	}
	if e = f.Sync(); e != nil {
		return nil, errors.New("NO_SPACE")
	}
	if e = f.Close(); e != nil {
		return nil, errors.New("NO_SPACE")
	}
	if e = ctx.Err(); e != nil {
		return nil, e
	}
	hash, size, e := hashFile(p)
	if e != nil {
		return nil, e
	}
	key := opaqueID()
	expires := time.Now().Add(a.cfg.ArchiveTTL)
	return preparedOutput{Manifest: mediaapi.ArchiveManifest{MediaKey: key, SHA256: hash, SizeBytes: size, ExpiresAt: expires.UTC().Format(time.RFC3339)}, Resources: map[string]resource{key: {Path: "archive.zip", ContentType: "application/zip", SHA256: hash, SizeBytes: size, ExpiresAt: expires}}}, nil
}
func copyArchiveImage(ctx context.Context, z *zip.Writer, path string, im mediaapi.ArchiveImage) error {
	f, e := os.Open(path)
	if e != nil {
		return errors.New("RESOURCE_NOT_FOUND")
	}
	defer f.Close()
	meta, e := jpeg.DecodeConfig(f)
	if e != nil || meta.Width != im.Width || meta.Height != im.Height {
		return errors.New("RESOURCE_CHANGED")
	}
	if _, e = f.Seek(0, io.SeekStart); e != nil {
		return errors.New("PROCESS_FAILED")
	}
	entry, e := z.CreateHeader(&zip.FileHeader{Name: "images/" + im.FrameID + ".jpg", Method: zip.Store})
	if e != nil {
		return errors.New("PROCESS_FAILED")
	}
	hash := sha256.New()
	n, e := io.Copy(io.MultiWriter(entry, hash), io.LimitReader(contextReader{ctx, f}, im.SizeBytes+1))
	if e != nil || n != im.SizeBytes || hex.EncodeToString(hash.Sum(nil)) != im.SHA256 {
		return errors.New("RESOURCE_CHANGED")
	}
	return nil
}
