package exports

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"image/jpeg"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/XB-Dong/classroom-annotator/backend/internal/platform/store"
)

// Bound temporary disk usage even when a selected scope contains many images.
const MaxArchiveImageBytes int64 = 2 * 1024 * 1024 * 1024

type coco struct {
	Images      []cocoImage      `json:"images"`
	Annotations []cocoAnnotation `json:"annotations"`
	Categories  []cocoCategory   `json:"categories"`
}
type cocoImage struct {
	ID       int    `json:"id"`
	FileName string `json:"file_name"`
	Width    int    `json:"width"`
	Height   int    `json:"height"`
}
type cocoAnnotation struct {
	ID         int        `json:"id"`
	ImageID    int        `json:"image_id"`
	CategoryID int        `json:"category_id"`
	BBox       [4]float64 `json:"bbox"`
	Area       float64    `json:"area"`
	IsCrowd    int        `json:"iscrowd"`
}
type cocoCategory struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

func (s Service) Create(ctx context.Context, selection Selection) (string, error) {
	records, err := s.Snapshot(ctx, selection)
	if err != nil {
		return "", err
	}
	return s.Build(ctx, records)
}

// Build owns all archive resources and removes partial output on every failure.
func (s Service) Build(ctx context.Context, records []Record) (path string, err error) {
	if _, err = providerOf(records); err != nil {
		return "", err
	}
	temp, err := os.CreateTemp(filepath.Join(s.Store.Root, "staging"), "export-*.zip")
	if err != nil {
		return "", err
	}
	path = temp.Name()
	tempPath := path
	success := false
	defer func() {
		temp.Close()
		if !success {
			os.Remove(tempPath)
		}
	}()
	archive := zip.NewWriter(temp)
	doc := coco{Images: []cocoImage{}, Annotations: []cocoAnnotation{}, Categories: []cocoCategory{{ID: 1, Name: "head"}}}
	var imageBytes int64
	for i, record := range records {
		if err = ctx.Err(); err != nil {
			return "", err
		}
		id := record.Frame.ID
		if id == "" || id == "." || id == ".." || strings.ContainsAny(id, "/\\:") {
			return "", errors.New("invalid frame media identity")
		}
		name := "images/" + id + ".jpg"
		n, copyErr := s.copyImage(ctx, archive, record, name, MaxArchiveImageBytes-imageBytes)
		if copyErr != nil {
			return "", copyErr
		}
		imageBytes += n
		imageID := i + 1
		doc.Images = append(doc.Images, cocoImage{ID: imageID, FileName: name, Width: record.Frame.Width, Height: record.Frame.Height})
		for _, a := range record.Annotations {
			doc.Annotations = append(doc.Annotations, cocoAnnotation{ID: len(doc.Annotations) + 1, ImageID: imageID, CategoryID: 1, BBox: a.BBox, Area: a.BBox[2] * a.BBox[3], IsCrowd: 0})
		}
	}
	jsonFile, err := archive.Create("annotations.json")
	if err != nil {
		return "", err
	}
	if err = json.NewEncoder(jsonFile).Encode(doc); err != nil {
		return "", err
	}
	if err = s.writeCredits(ctx, archive, records); err != nil {
		return "", err
	}
	if err = archive.Close(); err != nil {
		return "", err
	}
	if err = temp.Close(); err != nil {
		return "", err
	}
	if err = ctx.Err(); err != nil {
		return "", err
	}
	success = true
	return path, nil
}

const publicSampleChanges = "Frames extracted from the original video; no rendered annotation overlays."

type credit struct {
	SampleID      string `json:"sampleId"`
	Version       string `json:"version"`
	VideoID       string `json:"videoId"`
	Creator       string `json:"creator"`
	SourcePageURL string `json:"sourcePageUrl"`
	LicenseName   string `json:"licenseName"`
	LicenseURL    string `json:"licenseUrl"`
	Changes       string `json:"changes"`
}

func creditsForRows(rows []store.PublicSample) []credit {
	credits := make([]credit, 0, len(rows))
	for _, row := range rows {
		credits = append(credits, credit{SampleID: row.SampleID, Version: row.Version, VideoID: row.VideoID, Creator: row.Creator, SourcePageURL: row.SourcePageURL, LicenseName: row.LicenseName, LicenseURL: row.LicenseURL, Changes: publicSampleChanges})
	}
	sort.Slice(credits, func(i, j int) bool {
		if credits[i].SampleID != credits[j].SampleID {
			return credits[i].SampleID < credits[j].SampleID
		}
		if credits[i].Version != credits[j].Version {
			return credits[i].Version < credits[j].Version
		}
		return credits[i].VideoID < credits[j].VideoID
	})
	return credits
}

func (s Service) writeCredits(ctx context.Context, archive *zip.Writer, records []Record) error {
	seen := map[string]bool{}
	rows := []store.PublicSample{}
	for _, record := range records {
		id := record.Frame.VideoID
		if seen[id] {
			continue
		}
		seen[id] = true
		row, err := s.Store.PublicSampleByVideo(ctx, id)
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			return err
		}
		rows = append(rows, row)
	}
	if len(rows) == 0 {
		return nil
	}
	credits := creditsForRows(rows)
	file, err := archive.Create("credits.json")
	if err != nil {
		return err
	}
	if err = json.NewEncoder(file).Encode(struct {
		SchemaVersion string   `json:"schemaVersion"`
		Items         []credit `json:"items"`
	}{"1.0.0", credits}); err != nil {
		return err
	}
	licenseFile, err := archive.Create("LICENSES.txt")
	if err != nil {
		return err
	}
	for _, entry := range credits {
		if _, err = io.WriteString(licenseFile, entry.SampleID+" (version "+entry.Version+")\nCreator: "+entry.Creator+"\nSource: "+entry.SourcePageURL+"\nLicense: "+entry.LicenseName+" — "+entry.LicenseURL+"\nChanges: "+entry.Changes+"\n\n"); err != nil {
			return err
		}
	}
	return nil
}

func (s Service) copyImage(ctx context.Context, archive *zip.Writer, record Record, name string, remaining int64) (int64, error) {
	if record.Frame.StorageProvider == "mac" {
		return 0, ErrRetired
	}
	if record.Frame.MediaSHA256 == "" || record.Frame.MediaSizeBytes <= 0 {
		return 0, ErrBackfill
	}
	key := record.Frame.StorageKey
	if filepath.IsAbs(key) {
		return 0, errors.New("invalid frame media key")
	}
	root, err := filepath.EvalSymlinks(s.Store.Root)
	if err != nil {
		return 0, err
	}
	path, err := filepath.EvalSymlinks(filepath.Join(root, key))
	if err != nil {
		return 0, err
	}
	rel, err := filepath.Rel(root, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return 0, errors.New("invalid frame media location")
	}
	file, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer file.Close()
	cfg, err := jpeg.DecodeConfig(file)
	if err != nil {
		return 0, err
	}
	if cfg.Width != record.Frame.Width || cfg.Height != record.Frame.Height {
		return 0, errors.New("frame media dimensions differ")
	}
	if _, err = file.Seek(0, io.SeekStart); err != nil {
		return 0, err
	}
	entry, err := archive.CreateHeader(&zip.FileHeader{Name: name, Method: zip.Store})
	if err != nil {
		return 0, err
	}
	hash := sha256.New()
	n, err := io.Copy(io.MultiWriter(entry, hash), io.LimitReader(contextReader{ctx: ctx, reader: file}, remaining+1))
	if err != nil {
		return n, err
	}
	if n > remaining {
		return n, ErrLimit
	}
	if n != record.Frame.MediaSizeBytes || hex.EncodeToString(hash.Sum(nil)) != record.Frame.MediaSHA256 {
		return n, ErrChanged
	}
	return n, nil
}

type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r contextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(p)
}
