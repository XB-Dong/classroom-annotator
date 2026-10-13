package mediaagent

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/XB-Dong/classroom-annotator/backend/internal/mediaapi"
)

type legacyItem struct {
	ID          string `json:"id"`
	URL         string `json:"url"`
	SizeBytes   int64  `json:"size_bytes"`
	ContentType string `json:"content_type"`
	SHA256      string `json:"sha256"`
}

func legacyID(s string) bool {
	return len(s) > 0 && len(s) <= 255 && s != "." && s != ".." && strings.Trim(s, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789._-") == ""
}
func validHash(s string) bool { return len(s) == 64 && strings.Trim(s, "0123456789abcdef") == "" }
func (a *Agent) legacyRequest(ctx context.Context, path string) (*http.Response, error) {
	u, e := url.Parse(a.cfg.SourceURL)
	if e != nil || u.Scheme != "http" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Hostname() != "localhost" && u.Hostname() != "127.0.0.1" && u.Hostname() != "::1") {
		return nil, errors.New("PROCESS_FAILED")
	}
	u.Path = path
	u.RawPath = ""
	req, e := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
	if e != nil {
		return nil, errors.New("PROCESS_FAILED")
	}
	req.Header.Set("Authorization", "Bearer "+a.cfg.SourceToken)
	client := http.Client{Timeout: a.cfg.MaxOperation, Transport: &http.Transport{Proxy: nil, ResponseHeaderTimeout: 5 * time.Second}, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("RESOURCE_CHANGED") }}
	resp, e := client.Do(req)
	if e != nil {
		return nil, errors.New("PROCESS_FAILED")
	}
	if resp.StatusCode != 200 {
		resp.Body.Close()
		return nil, errors.New("PROCESS_FAILED")
	}
	return resp, nil
}
func (a *Agent) refreshSources(ctx context.Context) ([]mediaapi.Source, error) {
	resp, e := a.legacyRequest(ctx, "/v1/media")
	if e != nil {
		return nil, e
	}
	defer resp.Body.Close()
	var catalog struct {
		Items []legacyItem `json:"items"`
	}
	b, e := io.ReadAll(io.LimitReader(resp.Body, a.cfg.MaxControlBytes+1))
	if e != nil || int64(len(b)) > a.cfg.MaxControlBytes || json.Unmarshal(b, &catalog) != nil || catalog.Items == nil {
		return nil, errors.New("PROCESS_FAILED")
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	items := []mediaapi.Source{}
	seen := map[string]bool{}
	for _, old := range catalog.Items {
		if !legacyID(old.ID) || old.URL != "/v1/media/"+url.PathEscape(old.ID) || old.SizeBytes <= 0 || !validHash(old.SHA256) {
			continue
		}
		kind := ""
		if strings.HasPrefix(old.ContentType, "video/") {
			kind = "video"
		} else if strings.HasPrefix(old.ContentType, "image/") {
			kind = "image"
		} else {
			continue
		}
		identity := old.ID + ":" + old.SHA256
		if seen[identity] {
			continue
		}
		seen[identity] = true
		id := ""
		for existing, source := range a.state.Sources {
			if source.LegacyID == old.ID && source.Source.SHA256 == old.SHA256 {
				id = existing
				break
			}
		}
		if id == "" {
			id = opaqueID()
			a.state.Sources[id] = sourceRecord{Source: mediaapi.Source{SourceID: id, Name: old.ID, MediaKind: kind, SHA256: old.SHA256, SizeBytes: old.SizeBytes}, LegacyID: old.ID}
		}
		items = append(items, a.state.Sources[id].Source)
	}
	if e = a.persistLocked(); e != nil {
		return nil, e
	}
	sort.Slice(items, func(i, j int) bool { return items[i].SourceID < items[j].SourceID })
	return items, nil
}
func (a *Agent) sources(w http.ResponseWriter, r *http.Request) {
	items, e := a.refreshSources(r.Context())
	if e != nil {
		a.fail(w, 503, "PROCESS_FAILED")
		return
	}
	start := 0
	if cursor := r.URL.Query().Get("cursor"); cursor != "" {
		found := false
		for i, s := range items {
			if s.SourceID == cursor {
				start = i + 1
				found = true
				break
			}
		}
		if !found {
			a.fail(w, 400, "INVALID_REQUEST")
			return
		}
	}
	end := start + 20
	var next *string
	if end < len(items) {
		next = ptr(items[end-1].SourceID)
	} else {
		end = len(items)
	}
	a.json(w, 200, mediaapi.Sources{ContractVersion: mediaapi.ContractVersion, Items: items[start:end], NextCursor: next})
}
