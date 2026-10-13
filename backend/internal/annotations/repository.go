package annotations

import (
	"context"
	"database/sql"
	"errors"
	"github.com/XB-Dong/classroom-annotator/backend/internal/auth"
	"github.com/XB-Dong/classroom-annotator/backend/internal/reviewstate"
	"math"
	"sort"
	"strings"

	"github.com/XB-Dong/classroom-annotator/backend/internal/domain"
	"github.com/XB-Dong/classroom-annotator/backend/internal/platform/store"
	"github.com/jackc/pgx/v5/pgconn"
)

// MaxAnnotations bounds per-frame work independently of the HTTP body limit.
const MaxAnnotations = 10000
const MaxIDLength = 128

var ErrConflict = errors.New("annotation revision conflict")
var ErrInvalid = errors.New("invalid annotation snapshot")
var ErrIDCollision = errors.New("annotation ID belongs to another frame")

type Save struct {
	ExpectedRevision int64
	Status           string
	Annotations      []domain.Annotation
}
type Repository struct{ Store *store.Store }

func Validate(save Save, width, height int) error {
	if save.ExpectedRevision < 0 || save.ExpectedRevision == math.MaxInt64 || len(save.Annotations) > MaxAnnotations {
		return ErrInvalid
	}
	switch save.Status {
	case "not_started", "in_progress", "completed":
	default:
		return ErrInvalid
	}
	ids := make(map[string]struct{}, len(save.Annotations))
	for _, a := range save.Annotations {
		if strings.TrimSpace(a.ID) == "" || len(a.ID) > MaxIDLength || (a.Source != "manual" && a.Source != "ai" && a.Source != "ai_corrected") {
			return ErrInvalid
		}
		if (a.Source == "manual" && (a.CandidateID != "" || a.RunID != "")) || (a.Source != "manual" && (a.CandidateID == "" || a.RunID == "")) {
			return ErrInvalid
		}
		if _, exists := ids[a.ID]; exists {
			return ErrInvalid
		}
		ids[a.ID] = struct{}{}
		for _, n := range a.BBox {
			if math.IsNaN(n) || math.IsInf(n, 0) {
				return ErrInvalid
			}
		}
		b := a.BBox
		if b[0] < 0 || b[1] < 0 || b[2] <= 0 || b[3] <= 0 || b[0]+b[2] > float64(width) || b[1]+b[3] > float64(height) {
			return ErrInvalid
		}
	}
	return nil
}

// ReadAnnotations uses the caller's transaction, preserving its frame snapshot.
func ReadAnnotations(ctx context.Context, tx *sql.Tx, frameID string) ([]domain.Annotation, error) {
	rows, err := tx.QueryContext(ctx, `SELECT a.id,a.x,a.y,a.width,a.height,a.source,coalesce(p.candidate_id,''),coalesce(p.run_id,'') FROM annotations a LEFT JOIN annotation_provenance p ON p.annotation_id=a.id AND NOT p.deleted WHERE a.frame_id=$1 ORDER BY a.id`, frameID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.Annotation{}
	for rows.Next() {
		var a domain.Annotation
		if err = rows.Scan(&a.ID, &a.BBox[0], &a.BBox[1], &a.BBox[2], &a.BBox[3], &a.Source, &a.CandidateID, &a.RunID); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (r Repository) Get(ctx context.Context, id string) (domain.Snapshot, error) {
	ctx, cancel := r.Store.Context(ctx)
	defer cancel()
	tx, err := r.Store.DB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead})
	if err != nil {
		return domain.Snapshot{}, err
	}
	defer tx.Rollback()
	f, err := store.ScanFrame(tx.QueryRowContext(ctx, `SELECT `+store.FrameColumns+` FROM frames f WHERE f.id=$1`, id))
	if err != nil {
		return domain.Snapshot{}, err
	}
	anns, err := ReadAnnotations(ctx, tx, id)
	if err != nil {
		return domain.Snapshot{}, err
	}
	var updatedBy, updatedAt string
	if err = tx.QueryRowContext(ctx, `SELECT coalesce(updated_by,''),coalesce(updated_at::text,'') FROM frames WHERE id=$1`, id).Scan(&updatedBy, &updatedAt); err != nil {
		return domain.Snapshot{}, err
	}
	if err = tx.Commit(); err != nil {
		return domain.Snapshot{}, err
	}
	return domain.Snapshot{FrameID: id, Revision: f.Revision, Status: f.Status, Annotations: anns, UpdatedBy: updatedBy, UpdatedAt: updatedAt}, nil
}

func (r Repository) Save(ctx context.Context, id string, save Save) (domain.Snapshot, error) {
	ctx, cancel := r.Store.Context(ctx)
	defer cancel()
	tx, err := r.Store.DB.BeginTx(ctx, nil)
	if err != nil {
		return domain.Snapshot{}, err
	}
	defer tx.Rollback()
	f, err := store.ScanFrame(tx.QueryRowContext(ctx, `SELECT `+store.FrameColumns+` FROM frames f WHERE f.id=$1 FOR UPDATE`, id))
	if err != nil {
		return domain.Snapshot{}, err
	}
	if f.Revision != save.ExpectedRevision {
		return domain.Snapshot{}, ErrConflict
	}
	if err = Validate(save, f.Width, f.Height); err != nil {
		return domain.Snapshot{}, err
	}
	actor, _ := auth.ActorFrom(ctx)
	if err = reviewstate.Sync(ctx, tx, id, actor.ID, save.Annotations); err != nil {
		return domain.Snapshot{}, err
	}
	if save.Status == "completed" {
		if err = reviewstate.Gate(ctx, tx, id); err != nil {
			return domain.Snapshot{}, err
		}
	}
	for _, a := range save.Annotations {
		var owner string
		err = tx.QueryRowContext(ctx, `SELECT frame_id FROM annotations WHERE id=$1`, a.ID).Scan(&owner)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return domain.Snapshot{}, err
		}
		if err == nil && owner != id {
			return domain.Snapshot{}, ErrIDCollision
		}
	}
	result, err := tx.ExecContext(ctx, `UPDATE frames SET status=$1,revision=revision+1,updated_by=nullif($4,''),updated_at=now() WHERE id=$2 AND revision=$3`, save.Status, id, save.ExpectedRevision, actor.ID)
	if err != nil {
		return domain.Snapshot{}, err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return domain.Snapshot{}, err
	}
	if n != 1 {
		return domain.Snapshot{}, ErrConflict
	}
	keepIDs := make([]string, 0, len(save.Annotations))
	for _, a := range save.Annotations {
		keepIDs = append(keepIDs, a.ID)
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM annotations WHERE frame_id=$1 AND NOT(id=ANY($2::text[]))`, id, keepIDs); err != nil {
		return domain.Snapshot{}, err
	}
	ordered := append([]domain.Annotation(nil), save.Annotations...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].ID < ordered[j].ID })
	for _, a := range ordered {
		var inserted sql.Result
		if inserted, err = tx.ExecContext(ctx, `INSERT INTO annotations(id,frame_id,x,y,width,height,source) VALUES($1,$2,$3,$4,$5,$6,$7) ON CONFLICT(id) DO UPDATE SET x=excluded.x,y=excluded.y,width=excluded.width,height=excluded.height,source=excluded.source WHERE annotations.frame_id=excluded.frame_id`, a.ID, id, a.BBox[0], a.BBox[1], a.BBox[2], a.BBox[3], a.Source); err != nil {
			var pgError *pgconn.PgError
			if errors.As(err, &pgError) && pgError.Code == "23505" && pgError.ConstraintName == "annotations_global_id" {
				return domain.Snapshot{}, ErrIDCollision
			}
			return domain.Snapshot{}, err
		}
		n, err = inserted.RowsAffected()
		if err != nil {
			return domain.Snapshot{}, err
		}
		if n != 1 {
			return domain.Snapshot{}, ErrIDCollision
		}
	}
	action := "save"
	if save.Status == "completed" {
		action = "complete"
	} else if f.Status == "completed" {
		action = "reopen"
	}
	if err = reviewstate.Audit(ctx, tx, id, actor.ID, action, f.Revision+1); err != nil {
		return domain.Snapshot{}, err
	}
	var updatedAt string
	if err = tx.QueryRowContext(ctx, `SELECT updated_at::text FROM frames WHERE id=$1`, id).Scan(&updatedAt); err != nil {
		return domain.Snapshot{}, err
	}
	if err = tx.Commit(); err != nil {
		return domain.Snapshot{}, err
	}
	anns := save.Annotations
	if anns == nil {
		anns = []domain.Annotation{}
	}
	return domain.Snapshot{FrameID: id, Revision: f.Revision + 1, Status: save.Status, Annotations: anns, UpdatedBy: actor.ID, UpdatedAt: updatedAt}, nil
}
