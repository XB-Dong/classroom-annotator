package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/XB-Dong/classroom-annotator/backend/internal/migration"
	_ "github.com/jackc/pgx/v5/stdlib"
	_ "modernc.org/sqlite"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "SQLite import failed:", err)
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) != 2 || os.Args[1] == "" {
		return errors.New("usage: import-sqlite BACKUP.sqlite")
	}
	dsn := os.Getenv("APP_DATABASE_URL")
	if dsn == "" {
		return errors.New("PostgreSQL configuration required")
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return errors.New("PostgreSQL configuration invalid")
	}
	defer db.Close()
	db.SetMaxOpenConns(2)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		return errors.New("PostgreSQL target unavailable")
	}
	report, err := migration.Import(ctx, os.Args[1], db)
	if err != nil {
		return errors.New("source validation, import, or verification failed")
	}
	return json.NewEncoder(os.Stdout).Encode(report)
}
