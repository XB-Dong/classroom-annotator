package migration

import (
	"context"
	"database/sql"
	"github.com/XB-Dong/classroom-annotator/backend/migrations"
	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/lock"
)

func Provider(db *sql.DB) (*goose.Provider, error) {
	locker, err := lock.NewPostgresSessionLocker()
	if err != nil {
		return nil, err
	}
	return goose.NewProvider(goose.DialectPostgres, db, migrations.Files, goose.WithSessionLocker(locker), goose.WithLogger(goose.NopLogger()))
}
func Up(ctx context.Context, db *sql.DB) error {
	provider, err := Provider(db)
	if err != nil {
		return err
	}
	_, err = provider.Up(ctx)
	return err
}
