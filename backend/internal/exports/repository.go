package exports

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/XB-Dong/classroom-annotator/backend/internal/annotations"
	"github.com/XB-Dong/classroom-annotator/backend/internal/domain"
	"github.com/XB-Dong/classroom-annotator/backend/internal/platform/store"
	"github.com/XB-Dong/classroom-annotator/backend/internal/reviewstate"
)

const DefaultMaxFrames = 10000
const MaxExportAnnotations = 100000

var ErrInvalid = errors.New("invalid export selection")
var ErrEmpty = errors.New("no completed frames")
var ErrLimit = errors.New("export limit exceeded")

type Selection struct {
	VideoID, BatchID string
	FrameIDs         []string
}
type Record struct {
	Frame       domain.Frame
	Annotations []domain.Annotation
}
type Service struct {
	Store     *store.Store
	MaxFrames int
}

func (s Selection) validate(limit int) error {
	if len(s.FrameIDs) > limit {
		return ErrLimit
	}
	if s.VideoID != "" {
		if len(s.VideoID) > 128 || strings.TrimSpace(s.VideoID) == "" || len(s.FrameIDs) > 0 || s.FrameIDs != nil || len(s.BatchID) > 128 {
			return ErrInvalid
		}
		return nil
	}
	if s.BatchID != "" || len(s.FrameIDs) == 0 {
		return ErrInvalid
	}
	seen := map[string]bool{}
	for _, id := range s.FrameIDs {
		if strings.TrimSpace(id) == "" || len(id) > 128 || seen[id] {
			return ErrInvalid
		}
		seen[id] = true
	}
	return nil
}

func (s Service) frameLimit() int {
	if s.MaxFrames > 0 && s.MaxFrames < DefaultMaxFrames {
		return s.MaxFrames
	}
	return DefaultMaxFrames
}

// Snapshot ends the read transaction before images are read or ZIP work starts.
func (s Service) Snapshot(ctx context.Context, selection Selection) ([]Record, error) {
	ctx, cancel := s.Store.Context(ctx)
	defer cancel()
	if err := selection.validate(s.frameLimit()); err != nil {
		return nil, err
	}
	tx, err := s.Store.DB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	frames, err := selectedFrames(ctx, tx, selection, s.frameLimit())
	if err != nil {
		return nil, err
	}
	if len(frames) == 0 {
		return nil, ErrEmpty
	}
	out := make([]Record, 0, len(frames))
	totalAnnotations := 0
	for _, f := range frames {
		if err = reviewstate.Gate(ctx, tx, f.ID); err != nil {
			return nil, ErrInvalid
		}
		anns, err := annotations.ReadAnnotations(ctx, tx, f.ID)
		if err != nil {
			return nil, err
		}
		totalAnnotations += len(anns)
		if totalAnnotations > MaxExportAnnotations {
			return nil, ErrLimit
		}
		if err = annotations.Validate(annotations.Save{ExpectedRevision: f.Revision, Status: f.Status, Annotations: anns}, f.Width, f.Height); err != nil {
			return nil, err
		}
		if err = reviewstate.Verify(ctx, tx, f.ID, anns); err != nil {
			return nil, ErrInvalid
		}
		out = append(out, Record{Frame: f, Annotations: anns})
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return out, nil
}

func selectedFrames(ctx context.Context, tx *sql.Tx, selection Selection, limit int) ([]domain.Frame, error) {
	frames := []domain.Frame{}
	if selection.VideoID == "" {
		// Resolve every explicit ID, including unfinished frames, to detect missing IDs.
		for _, id := range selection.FrameIDs {
			f, err := store.ScanFrame(tx.QueryRowContext(ctx, `SELECT `+store.FrameColumns+` FROM frames f WHERE f.id=$1`, id))
			if err != nil {
				return nil, err
			}
			if f.Status == "completed" {
				frames = append(frames, f)
			}
		}
		return frames, nil
	}
	var id string
	if err := tx.QueryRowContext(ctx, `SELECT id FROM videos WHERE id=$1`, selection.VideoID).Scan(&id); err != nil {
		return nil, err
	}
	if selection.BatchID != "" {
		if err := tx.QueryRowContext(ctx, `SELECT id FROM batches WHERE id=$1 AND video_id=$2`, selection.BatchID, selection.VideoID).Scan(&id); err != nil {
			return nil, err
		}
	}
	rows, err := tx.QueryContext(ctx, `SELECT `+store.FrameColumns+` FROM frames f JOIN batches b ON b.id=f.batch_id WHERE f.video_id=$1 AND ($2='' OR f.batch_id=$3) AND f.status='completed' ORDER BY b.created_at,b.id,f.timestamp_ms,f.id LIMIT $4`, selection.VideoID, selection.BatchID, selection.BatchID, limit+1)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		f, err := store.ScanFrame(rows)
		if err != nil {
			return nil, err
		}
		frames = append(frames, f)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	if len(frames) > limit {
		return nil, ErrLimit
	}
	return frames, nil
}
