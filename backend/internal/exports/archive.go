package exports

import (
	"archive/zip"
	"context"
	"encoding/json"
	"errors"
	"image/jpeg"
	"io"
	"os"
	"path/filepath"
	"strings"
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

func (s Service) copyImage(ctx context.Context, archive *zip.Writer, record Record, name string, remaining int64) (int64, error) {
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
	n, err := io.Copy(entry, io.LimitReader(contextReader{ctx: ctx, reader: file}, remaining+1))
	if err != nil {
		return n, err
	}
	if n > remaining {
		return n, ErrLimit
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
