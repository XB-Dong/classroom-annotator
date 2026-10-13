package dataset

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/XB-Dong/classroom-annotator/backend/internal/platform/config"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var ErrConfig = errors.New("DATASET_NOT_CONFIGURED")
var ErrResponse = errors.New("DATASET_INVALID_RESPONSE")
var ErrUpstream = errors.New("DATASET_UPSTREAM_FAILED")

type Adapter struct {
	base, key, datasetID string
	api, media           *http.Client
	policy               *Policy
}

func New(c config.Config) *Adapter {
	timeout := c.HTTPTimeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	p := &Policy{PrivateOrigins: c.AllowedPrivateOrigins, MediaOrigins: c.AllowedMediaOrigins}
	base := strings.TrimRight(c.DatasetAPIURL, "/")
	api := &http.Client{Timeout: timeout, Transport: p.Transport()}
	api.CheckRedirect = func(r *http.Request, via []*http.Request) error {
		if len(via) >= 5 || Origin(r.URL) != Origin(via[0].URL) {
			return ErrUpstream
		}
		return p.Validate(r.URL, false)
	}
	mediaTimeout := c.MediaDownloadTimeout
	if mediaTimeout <= 0 {
		mediaTimeout = timeout
	}
	media := &http.Client{Timeout: mediaTimeout, Transport: p.Transport()}
	media.CheckRedirect = func(r *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			return ErrUpstream
		}
		return p.Validate(r.URL, true)
	}
	return &Adapter{base: base, key: c.DatasetAPIKey, datasetID: c.DatasetID, api: api, media: media, policy: p}
}
func (a *Adapter) Configured() bool { return a.base != "" && a.datasetID != "" }
func (a *Adapter) request(ctx context.Context, path string, dst any) error {
	if !a.Configured() {
		return ErrConfig
	}
	u, e := url.Parse(a.base + path)
	if e != nil || a.policy.Validate(u, false) != nil {
		return ErrUpstream
	}
	r, e := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
	if e != nil {
		return ErrUpstream
	}
	r.Header.Set("X-API-Key", a.key)
	resp, e := a.api.Do(r)
	if e != nil {
		return ErrUpstream
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return ErrUpstream
	}
	b, e := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024+1))
	if e != nil || len(b) > 2*1024*1024 {
		return ErrResponse
	}
	if json.Unmarshal(b, dst) != nil {
		return ErrResponse
	}
	return nil
}
func (a *Adapter) Lessons(ctx context.Context, page, size int) (Page, error) {
	var p Page
	e := a.request(ctx, fmt.Sprintf("/api/v1/datasets/%s/lessons?page=%d&page_size=%d", url.PathEscape(a.datasetID), page, size), &p)
	if e != nil {
		return p, e
	}
	if p.Items == nil || p.Type == "" || p.Total < 0 || p.Stats == nil {
		return p, ErrResponse
	}
	return p, nil
}
func (a *Adapter) Download(ctx context.Context, id string) (io.ReadCloser, error) {
	if !ValidCatalogID(id) {
		return nil, ErrResponse
	}
	var play struct {
		URL string `json:"url"`
	}
	if e := a.request(ctx, "/api/v1/catalog-files/"+url.PathEscape(id)+"/play-url", &play); e != nil {
		return nil, e
	}
	u, e := url.Parse(play.URL)
	if e != nil || a.policy.Validate(u, true) != nil {
		return nil, ErrUpstream
	}
	r, e := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
	if e != nil {
		return nil, ErrUpstream
	}
	resp, e := a.media.Do(r)
	if e != nil {
		return nil, ErrUpstream
	}
	if resp.StatusCode != 200 {
		resp.Body.Close()
		return nil, ErrUpstream
	}
	return resp.Body, nil
}
