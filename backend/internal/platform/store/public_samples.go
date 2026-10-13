package store

import (
	"context"
	"errors"
	"fmt"
	"math"
	"regexp"

	"github.com/XB-Dong/classroom-annotator/backend/internal/domain"
)

var ErrSampleConflict = errors.New("public sample identity changed")
var ErrSampleUnavailable = errors.New("public sample unavailable")

var sampleHash = regexp.MustCompile(`^[0-9a-f]{64}$`)
var sampleKey = regexp.MustCompile(`^videos/[0-9a-f]{64}\.(mp4|webm)$`)

type PublicSample struct {
	SampleID        string
	Version         string
	Name            string
	Description     string
	Creator         string
	LicenseName     string
	LicenseURL      string
	SourcePageURL   string
	SHA256          string
	SizeBytes       int64
	Width           int
	Height          int
	DurationSeconds float64
	StorageKey      string
	CacheStatus     string
	VideoID         string
}

const sampleColumns = `sample_id,version,name,description,author,license,license_url,source_page,media_sha256,media_size_bytes,width,height,duration_seconds,storage_key,cache_status,coalesce(video_id,'')`

func scanSample(r interface{ Scan(...any) error }) (PublicSample, error) {
	var row PublicSample
	err := r.Scan(&row.SampleID, &row.Version, &row.Name, &row.Description, &row.Creator, &row.LicenseName, &row.LicenseURL, &row.SourcePageURL, &row.SHA256, &row.SizeBytes, &row.Width, &row.Height, &row.DurationSeconds, &row.StorageKey, &row.CacheStatus, &row.VideoID)
	return row, err
}

func validSample(s PublicSample) bool {
	return s.SampleID != "" && len(s.SampleID) <= 128 && s.Version != "" && len(s.Version) <= 64 && s.Name != "" && s.Creator != "" && s.LicenseName != "" && s.LicenseURL != "" && s.SourcePageURL != "" && sampleHash.MatchString(s.SHA256) && sampleKey.MatchString(s.StorageKey) && s.SizeBytes > 0 && s.Width > 0 && s.Height > 0 && s.DurationSeconds > 0 && !math.IsNaN(s.DurationSeconds) && !math.IsInf(s.DurationSeconds, 0) && s.CacheStatus == "ready"
}

// UpsertPublicSample keeps an existing version and its video/annotations immutable.
func (s *Store) UpsertPublicSample(ctx context.Context, sample PublicSample) error {
	if !validSample(sample) {
		return ErrSampleConflict
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	// Serialization is scoped to this publisher-defined identity, even though the primary key includes SHA.
	if _, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext($1),hashtext($2))`, sample.SampleID, sample.Version); err != nil {
		return err
	}
	rows, err := tx.QueryContext(ctx, `SELECT `+sampleColumns+` FROM public_samples WHERE sample_id=$1 AND version=$2 FOR UPDATE`, sample.SampleID, sample.Version)
	if err != nil {
		return err
	}
	if rows.Next() {
		previous, scanErr := scanSample(rows)
		if scanErr != nil {
			rows.Close()
			return scanErr
		}
		if rows.Next() {
			rows.Close()
			return ErrSampleConflict
		}
		if err = rows.Err(); err != nil {
			rows.Close()
			return err
		}
		rows.Close()
		if previous.SHA256 != sample.SHA256 || previous.SizeBytes != sample.SizeBytes || previous.Width != sample.Width || previous.Height != sample.Height || previous.DurationSeconds != sample.DurationSeconds || previous.StorageKey != sample.StorageKey || previous.CacheStatus != "ready" || previous.Name != sample.Name || previous.Description != sample.Description || previous.Creator != sample.Creator || previous.LicenseName != sample.LicenseName || previous.LicenseURL != sample.LicenseURL || previous.SourcePageURL != sample.SourcePageURL {
			return ErrSampleConflict
		}
		return tx.Commit()
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	_, err = tx.ExecContext(ctx, `INSERT INTO public_samples(sample_id,version,name,description,author,license,license_url,source_page,media_sha256,media_size_bytes,width,height,duration_seconds,storage_key,cache_status,video_id) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,'ready',NULL)`, sample.SampleID, sample.Version, sample.Name, sample.Description, sample.Creator, sample.LicenseName, sample.LicenseURL, sample.SourcePageURL, sample.SHA256, sample.SizeBytes, sample.Width, sample.Height, sample.DurationSeconds, sample.StorageKey)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) PublicSamples(ctx context.Context) ([]PublicSample, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT `+sampleColumns+` FROM public_samples ORDER BY sample_id,version`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []PublicSample{}
	for rows.Next() {
		row, err := scanSample(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (s *Store) PublicSampleByIdentity(ctx context.Context, sampleID, version, hash string) (PublicSample, error) {
	return scanSample(s.DB.QueryRowContext(ctx, `SELECT `+sampleColumns+` FROM public_samples WHERE sample_id=$1 AND version=$2 AND media_sha256=$3`, sampleID, version, hash))
}

func (s *Store) PublicSampleByVideo(ctx context.Context, videoID string) (PublicSample, error) {
	return scanSample(s.DB.QueryRowContext(ctx, `SELECT `+sampleColumns+` FROM public_samples WHERE video_id=$1`, videoID))
}

// ImportPublicSample registers a cached sample and never changes an existing video or its frames.
func (s *Store) ImportPublicSample(ctx context.Context, sampleID, version, hash string) (domain.Video, bool, error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return domain.Video{}, false, err
	}
	defer tx.Rollback()
	row, err := scanSample(tx.QueryRowContext(ctx, `SELECT `+sampleColumns+` FROM public_samples WHERE sample_id=$1 AND version=$2 AND media_sha256=$3 FOR UPDATE`, sampleID, version, hash))
	if err != nil {
		return domain.Video{}, false, err
	}
	if row.CacheStatus != "ready" || !sampleKey.MatchString(row.StorageKey) {
		return domain.Video{}, false, ErrSampleUnavailable
	}
	if row.VideoID != "" {
		video, err := scanVideo(tx.QueryRowContext(ctx, `SELECT `+videoColumns+` FROM videos WHERE id=$1`, row.VideoID))
		if err != nil {
			return domain.Video{}, false, err
		}
		if err = tx.Commit(); err != nil {
			return domain.Video{}, false, err
		}
		return video, false, nil
	}
	video := domain.Video{ID: domain.ID(), Name: row.Name, Width: row.Width, Height: row.Height, DurationSeconds: row.DurationSeconds, Status: "ready", CreatedAt: domain.Now(), StorageKey: row.StorageKey, SourceKind: "public_sample", SourceID: row.SampleID, StorageProvider: "local", MediaSHA256: row.SHA256, MediaSizeBytes: row.SizeBytes}
	if _, err = tx.ExecContext(ctx, insertVideo, videoArgs(video)...); err != nil {
		return domain.Video{}, false, err
	}
	result, err := tx.ExecContext(ctx, `UPDATE public_samples SET video_id=$1 WHERE sample_id=$2 AND version=$3 AND media_sha256=$4 AND video_id IS NULL`, video.ID, sampleID, version, hash)
	if err != nil {
		return domain.Video{}, false, err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return domain.Video{}, false, err
	}
	if n != 1 {
		return domain.Video{}, false, fmt.Errorf("%w: sample link changed", ErrSampleConflict)
	}
	if err = tx.Commit(); err != nil {
		return domain.Video{}, false, err
	}
	return video, true, nil
}
