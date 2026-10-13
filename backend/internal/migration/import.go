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
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"unicode"
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

// These digests pin sqlite_master SQL produced by P0's store/schema.sql. They
// cover table CHECK, NOT NULL, UNIQUE, PRIMARY KEY, and FOREIGN KEY clauses as
// well as named indexes. SQLite-generated autoindexes have NULL SQL and are
// implied by the pinned table definitions.
var legacyDDL = map[string]string{
	"index:annotations_frame": "ea18fd73ac85eb48c40191e81178bb764a8925c696822a25f54b3626ff4547cd",
	"index:frames_batch":      "6d2bdd81c43d44a767a356956f054789103bf000c1be492122d54e1680a425ef",
	"table:annotations":       "1b1f4cf92c24ad97f8b9d9bcb8b51308f1e76a79529a2b8be083f4ce09934b90",
	"table:batches":           "9fa4caa21410e30482497bb2c4cebf13797634a4c50e61de30bf237a5d038b2d",
	"table:frames":            "a319e3316442f8e5f6b5662de0194d028d16b18f19d47477c69e9951b90ff607",
	"table:jobs":              "fbebe9fce0413f3682aeccc2ba1269359dd9e8c78d411578158b395c63f9d69f",
	"table:schema_version":    "82c2124a1ec884c03eca690d2625dd197f3f098d5cec66907b5a3085db974c9d",
	"table:videos":            "f7e825eb04b536d1567430bc95ead2e2db6fcefebd60518e5459e94aa2c9176e",
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
	if err := rejectSidecars(abs); err != nil {
		return report, err
	}
	before, err := fileSHA256(abs)
	if err != nil {
		return report, fmt.Errorf("hash SQLite backup: %w", err)
	}
	src, err := sql.Open("sqlite", sqliteBackupURI(abs))
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
	if _, err = tx.ExecContext(ctx, `LOCK TABLE videos,jobs,batches,frames,annotations,public_samples,media_operations IN ACCESS EXCLUSIVE MODE`); err != nil {
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
	for _, table := range []string{"public_samples", "media_operations"} {
		var count int64
		if err := tx.QueryRowContext(ctx, "SELECT count(*) FROM "+table).Scan(&count); err != nil {
			return report, errors.New("check PostgreSQL target failed")
		}
		if count != 0 {
			return report, errors.New("PostgreSQL target business tables must be empty")
		}
	}
	for _, table := range legacyTables {
		columns := strings.Join(table.columns, ",")
		rows, err := src.QueryContext(ctx, "SELECT "+columns+" FROM "+table.name+" ORDER BY id COLLATE BINARY")
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
	if err := rejectSidecars(abs); err != nil {
		return report, err
	}
	if err := tx.Commit(); err != nil {
		return report, errors.New("commit PostgreSQL import failed")
	}
	report.BackupSHA256 = before
	return report, nil
}

func sqliteBackupURI(abs string) string {
	parts := strings.Split(filepath.ToSlash(abs), "/")
	for i := range parts {
		parts[i] = url.PathEscape(parts[i])
	}
	return "file:" + strings.Join(parts, "/") + "?mode=ro&immutable=1"
}

func rejectSidecars(abs string) error {
	for _, suffix := range []string{"-wal", "-shm"} {
		if _, err := os.Stat(abs + suffix); err == nil {
			return errors.New("SQLite backup has a sidecar file")
		} else if !errors.Is(err, os.ErrNotExist) {
			return errors.New("cannot inspect SQLite backup sidecars")
		}
	}
	return nil
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
	rows, err = db.QueryContext(ctx, `SELECT type,name,sql FROM sqlite_master WHERE sql IS NOT NULL ORDER BY type,name`)
	if err != nil {
		return errors.New("SQLite schema inspection failed")
	}
	actual := map[string]bool{}
	for rows.Next() {
		var kind, name, definition string
		if err := rows.Scan(&kind, &name, &definition); err != nil {
			rows.Close()
			return errors.New("SQLite schema inspection failed")
		}
		key := kind + ":" + name
		want, known := legacyDDL[key]
		digest := sha256.Sum256([]byte(canonicalDDL(definition)))
		if !known || fmt.Sprintf("%x", digest) != want {
			rows.Close()
			return errors.New("unexpected SQLite schema definition")
		}
		actual[key] = true
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return errors.New("SQLite schema inspection failed")
	}
	rows.Close()
	if len(actual) != len(legacyDDL) {
		return errors.New("incomplete SQLite schema")
	}
	var versionCount, minVersion, maxVersion int64
	if err := db.QueryRowContext(ctx, `SELECT count(*),coalesce(min(version),0),coalesce(max(version),0) FROM schema_version`).Scan(&versionCount, &minVersion, &maxVersion); err != nil || versionCount != 1 || minVersion != 1 || maxVersion != 1 {
		return errors.New("unsupported SQLite schema version")
	}
	return nil
}

func canonicalDDL(definition string) string {
	var out strings.Builder
	var closing rune
	for _, r := range definition {
		if closing != 0 {
			out.WriteRune(r)
			if r == closing {
				closing = 0
			}
			continue
		}
		switch r {
		case '\'', '"', '`':
			closing = r
			out.WriteRune(r)
		case '[':
			closing = ']'
			out.WriteRune(r)
		default:
			if !unicode.IsSpace(r) {
				out.WriteRune(r)
			}
		}
	}
	return out.String()
}

type queryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func digestQuery(ctx context.Context, q queryer, table, columns string, width int) (Digest, error) {
	rows, err := q.QueryContext(ctx, "SELECT "+columns+" FROM "+table+` ORDER BY id COLLATE "C"`)
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
