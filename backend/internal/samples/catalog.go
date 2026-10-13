// Package samples validates the publisher-controlled public media catalog.
package samples

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"math"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var ErrCatalog = errors.New("invalid public sample catalog")
var sampleID = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,127}$`)
var versionID = regexp.MustCompile(`^[A-Za-z0-9._-]{1,64}$`)
var sha256Hex = regexp.MustCompile(`^[0-9a-f]{64}$`)

type Item struct {
	SampleID        string  `json:"sampleId"`
	Version         string  `json:"version"`
	Name            string  `json:"name"`
	Description     string  `json:"description"`
	Creator         string  `json:"creator"`
	SourcePageURL   string  `json:"sourcePageUrl"`
	LicenseName     string  `json:"licenseName"`
	LicenseURL      string  `json:"licenseUrl"`
	DownloadURL     string  `json:"downloadUrl"`
	SHA256          string  `json:"sha256"`
	SizeBytes       int64   `json:"sizeBytes"`
	Width           int     `json:"width"`
	Height          int     `json:"height"`
	DurationSeconds float64 `json:"durationSeconds"`
	RetrievedAt     string  `json:"retrievedAt"`
	SourceFile      string  `json:"sourceFile"`
}
type Catalog struct {
	CatalogVersion string `json:"catalogVersion"`
	Items          []Item `json:"items"`
}

func validHTTPS(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && u.Scheme == "https" && u.Hostname() != "" && u.User == nil && u.Fragment == ""
}
func safeSourceFile(name string) bool {
	if name == "" || filepath.IsAbs(name) || filepath.Base(name) != name || strings.ContainsAny(name, "\\/:") || name == "." || name == ".." {
		return false
	}
	return !strings.HasPrefix(name, ".")
}
func validateItem(item Item) error {
	if !sampleID.MatchString(item.SampleID) || !versionID.MatchString(item.Version) || len(item.Name) == 0 || len(item.Name) > 255 || len(item.Description) > 2000 || len(item.Creator) == 0 || len(item.Creator) > 255 || len(item.LicenseName) == 0 || len(item.LicenseName) > 255 || !validHTTPS(item.SourcePageURL) || !validHTTPS(item.LicenseURL) || (item.DownloadURL != "" && !validHTTPS(item.DownloadURL)) || !sha256Hex.MatchString(item.SHA256) || item.SizeBytes <= 0 || item.SizeBytes > 1<<30 || item.Width <= 0 || item.Height <= 0 || item.Width > 16384 || item.Height > 16384 || item.DurationSeconds <= 0 || math.IsNaN(item.DurationSeconds) || math.IsInf(item.DurationSeconds, 0) || !safeSourceFile(item.SourceFile) {
		return ErrCatalog
	}
	return nil
}

func Load(catalogPath string) ([]Item, error) {
	if !filepath.IsAbs(catalogPath) {
		return nil, ErrCatalog
	}
	info, err := os.Lstat(catalogPath)
	if err != nil || !info.Mode().IsRegular() || info.Size() > 1<<20 {
		return nil, ErrCatalog
	}
	f, err := os.Open(catalogPath)
	if err != nil {
		return nil, ErrCatalog
	}
	defer f.Close()
	var catalog Catalog
	decoder := json.NewDecoder(io.LimitReader(f, 1<<20+1))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&catalog) != nil {
		return nil, ErrCatalog
	}
	var tail any
	if decoder.Decode(&tail) != io.EOF || catalog.CatalogVersion != "1.0.0" || len(catalog.Items) == 0 || len(catalog.Items) > 100 {
		return nil, ErrCatalog
	}
	seen := map[string]bool{}
	for _, item := range catalog.Items {
		if err := validateItem(item); err != nil {
			return nil, err
		}
		key := item.SampleID + "\x00" + item.Version
		if seen[key] {
			return nil, ErrCatalog
		}
		seen[key] = true
	}
	return catalog.Items, nil
}

func VerifySource(catalogDir string, item Item) error {
	if !safeSourceFile(item.SourceFile) || !sha256Hex.MatchString(item.SHA256) || item.SizeBytes <= 0 || item.SizeBytes > 1<<30 {
		return ErrCatalog
	}
	root, err := filepath.Abs(catalogDir)
	if err != nil {
		return ErrCatalog
	}
	rootInfo, err := os.Lstat(root)
	if err != nil || !rootInfo.IsDir() || rootInfo.Mode()&os.ModeSymlink != 0 {
		return ErrCatalog
	}
	path := filepath.Join(root, item.SourceFile)
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Size() != item.SizeBytes {
		return ErrCatalog
	}
	f, err := os.Open(path)
	if err != nil {
		return ErrCatalog
	}
	defer f.Close()
	h := sha256.New()
	n, err := io.Copy(h, io.LimitReader(f, item.SizeBytes+1))
	if err != nil || n != item.SizeBytes || hex.EncodeToString(h.Sum(nil)) != item.SHA256 {
		return ErrCatalog
	}
	return nil
}
