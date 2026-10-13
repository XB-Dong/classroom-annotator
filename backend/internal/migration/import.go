package migration

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
)

type Digest struct {
	Count  int64  `json:"count"`
	SHA256 string `json:"sha256"`
}
type TableReport struct {
	Name   string `json:"name"`
	Source Digest `json:"source"`
	Target Digest `json:"target"`
}
type Report struct {
	BackupSHA256 string        `json:"backupSha256"`
	Tables       []TableReport `json:"tables"`
}

var legacyTables = []struct {
	name    string
	columns []string
}{
	{"videos", []string{"id", "name", "width", "height", "duration_seconds", "status", "created_at", "storage_key"}},
	{"jobs", []string{"id", "kind", "video_id", "batch_id", "status", "progress", "error", "created_at"}},
	{"batches", []string{"id", "video_id", "job_id", "created_at"}},
	{"frames", []string{"id", "video_id", "batch_id", "timestamp_ms", "width", "height", "status", "revision", "storage_key"}},
	{"annotations", []string{"id", "frame_id", "x", "y", "width", "height", "source"}},
}

// Import reads an offline SQLite backup and atomically copies the five P0 tables.
func Import(ctx context.Context, source string, target *sql.DB) (Report, error) {
	var report Report
	abs, err := filepath.Abs(source)
	if err != nil {
		return report, fmt.Errorf("resolve SQLite backup: %w", err)
	}
	stat, err := os.Stat(abs)
	if err != nil || !stat.Mode().IsRegular() {
		return report, errors.New("SQLite backup must be a regular file")
	}
	if _, err = os.Stat(abs + "-wal"); err == nil {
		return report, errors.New("SQLite backup has an unresolved WAL")
	} else if !errors.Is(err, os.ErrNotExist) {
		return report, errors.New("cannot inspect SQLite WAL")
	}
	before, err := fileSHA256(abs)
	if err != nil {
		return report, fmt.Errorf("hash SQLite backup: %w", err)
	}
	src, err := sql.Open("sqlite", "file:"+filepath.ToSlash(abs)+"?mode=ro")
	if err != nil {
		return report, errors.New("open SQLite backup failed")
	}
	defer src.Close()
	src.SetMaxOpenConns(1)
	if err := validateLegacy(ctx, src); err != nil {
		return report, err
	}
	if target == nil {
		return report, errors.New("PostgreSQL target is required")
	}
	tx, err := target.BeginTx(ctx, nil)
	if err != nil {
		return report, errors.New("begin PostgreSQL import failed")
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `LOCK TABLE videos,jobs,batches,frames,annotations IN ACCESS EXCLUSIVE MODE`); err != nil {
		return report, errors.New("lock PostgreSQL target failed")
	}
	for _, table := range legacyTables {
		var count int64
		if err := tx.QueryRowContext(ctx, "SELECT count(*) FROM "+table.name).Scan(&count); err != nil {
			return report, errors.New("check PostgreSQL target failed")
		}
		if count != 0 {
			return report, errors.New("PostgreSQL target business tables must be empty")
		}
	}
	for _, table := range legacyTables {
		columns := strings.Join(table.columns, ",")
		rows, err := src.QueryContext(ctx, "SELECT "+columns+" FROM "+table.name+" ORDER BY id")
		if err != nil {
			return report, fmt.Errorf("read SQLite %s failed: %w", table.name, err)
		}
		placeholders := make([]string, len(table.columns))
		for i := range placeholders {
			placeholders[i] = fmt.Sprintf("$%d", i+1)
		}
		insert := "INSERT INTO " + table.name + "(" + columns + ") VALUES(" + strings.Join(placeholders, ",") + ")"
		hash := sha256.New()
		var count int64
		for rows.Next() {
			values, err := scanValues(rows, len(table.columns))
			if err != nil {
				rows.Close()
				return report, fmt.Errorf("scan SQLite %s failed: %w", table.name, err)
			}
			if err := writeCanonical(hash, values); err != nil {
				rows.Close()
				return report, err
			}
			if _, err := tx.ExecContext(ctx, insert, values...); err != nil {
				rows.Close()
				return report, fmt.Errorf("insert PostgreSQL %s failed: %w", table.name, err)
			}
			count++
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return report, fmt.Errorf("read SQLite %s failed: %w", table.name, err)
		}
		if err := rows.Close(); err != nil {
			return report, err
		}
		sourceDigest := Digest{Count: count, SHA256: hex.EncodeToString(hash.Sum(nil))}
		targetDigest, err := digestQuery(ctx, tx, table.name, columns, len(table.columns))
		if err != nil {
			return report, err
		}
		report.Tables = append(report.Tables, TableReport{Name: table.name, Source: sourceDigest, Target: targetDigest})
		if sourceDigest != targetDigest {
			return report, fmt.Errorf("%s source and target digests differ", table.name)
		}
	}
	after, err := fileSHA256(abs)
	if err != nil || before != after {
		return report, errors.New("SQLite backup changed during import")
	}
	if err := tx.Commit(); err != nil {
		return report, errors.New("commit PostgreSQL import failed")
	}
	report.BackupSHA256 = before
	return report, nil
}

func validateLegacy(ctx context.Context, db *sql.DB) error {
	var result string
	if err := db.QueryRowContext(ctx, `PRAGMA integrity_check`).Scan(&result); err != nil {
		return fmt.Errorf("SQLite integrity check failed: %w", err)
	} else if result != "ok" {
		return errors.New("SQLite integrity check failed")
	}
	rows, err := db.QueryContext(ctx, `PRAGMA foreign_key_check`)
	if err != nil {
		return errors.New("SQLite foreign key check failed")
	}
	if rows.Next() {
		rows.Close()
		return errors.New("SQLite foreign key check failed")
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return errors.New("SQLite foreign key check failed")
	}
	rows.Close()
	rows, err = db.QueryContext(ctx, `SELECT name FROM sqlite_master WHERE type='table' ORDER BY name`)
	if err != nil {
		return errors.New("SQLite schema inspection failed")
	}
	actual := map[string]bool{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			rows.Close()
			return errors.New("SQLite schema inspection failed")
		}
		actual[name] = true
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return errors.New("SQLite schema inspection failed")
	}
	rows.Close()
	if len(actual) != len(legacyTables)+1 || !actual["schema_version"] {
		return errors.New("unexpected SQLite tables")
	}
	var versionCount, minVersion, maxVersion int64
	if err := db.QueryRowContext(ctx, `SELECT count(*),coalesce(min(version),0),coalesce(max(version),0) FROM schema_version`).Scan(&versionCount, &minVersion, &maxVersion); err != nil || versionCount != 1 || minVersion != 1 || maxVersion != 1 {
		return errors.New("unsupported SQLite schema version")
	}
	for _, table := range legacyTables {
		if !actual[table.name] {
			return fmt.Errorf("missing SQLite %s table", table.name)
		}
		rows, err := db.QueryContext(ctx, "PRAGMA table_info("+table.name+")")
		if err != nil {
			return errors.New("SQLite schema inspection failed")
		}
		columns := []string{}
		for rows.Next() {
			var cid, notNull, pk int
			var name, typ string
			var defaultValue sql.NullString
			if err := rows.Scan(&cid, &name, &typ, &notNull, &defaultValue, &pk); err != nil {
				rows.Close()
				return errors.New("SQLite schema inspection failed")
			}
			columns = append(columns, name)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return errors.New("SQLite schema inspection failed")
		}
		rows.Close()
		if !reflect.DeepEqual(columns, table.columns) {
			return fmt.Errorf("unexpected SQLite %s columns", table.name)
		}
	}
	return nil
}

type queryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func digestQuery(ctx context.Context, q queryer, table, columns string, width int) (Digest, error) {
	rows, err := q.QueryContext(ctx, "SELECT "+columns+" FROM "+table+" ORDER BY id")
	if err != nil {
		return Digest{}, fmt.Errorf("read PostgreSQL %s failed: %w", table, err)
	}
	defer rows.Close()
	hash := sha256.New()
	var count int64
	for rows.Next() {
		values, err := scanValues(rows, width)
		if err != nil {
			return Digest{}, err
		}
		if err := writeCanonical(hash, values); err != nil {
			return Digest{}, err
		}
		count++
	}
	if err := rows.Err(); err != nil {
		return Digest{}, err
	}
	return Digest{Count: count, SHA256: hex.EncodeToString(hash.Sum(nil))}, nil
}
func scanValues(rows *sql.Rows, width int) ([]any, error) {
	values := make([]any, width)
	dest := make([]any, width)
	for i := range values {
		dest[i] = &values[i]
	}
	if err := rows.Scan(dest...); err != nil {
		return nil, err
	}
	return values, nil
}
func writeCanonical(w io.Writer, values []any) error {
	for i, v := range values {
		if bytes, ok := v.([]byte); ok {
			values[i] = string(bytes)
		}
	}
	encoded, err := json.Marshal(values)
	if err != nil {
		return err
	}
	if _, err = w.Write(encoded); err != nil {
		return err
	}
	_, err = w.Write([]byte{'\n'})
	return err
}
func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
