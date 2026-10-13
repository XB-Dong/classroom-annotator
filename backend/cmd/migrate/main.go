package main

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/XB-Dong/classroom-annotator/backend/internal/migration"
	_ "github.com/jackc/pgx/v5/stdlib"
	"os"
	"time"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "migration failed")
		os.Exit(1)
	}
}
func run() error {
	if len(os.Args) != 2 || (os.Args[1] != "up" && os.Args[1] != "status") {
		return fmt.Errorf("usage: migrate up|status")
	}
	dsn := os.Getenv("APP_DATABASE_URL")
	if dsn == "" {
		return fmt.Errorf("PostgreSQL configuration required")
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return err
	}
	defer db.Close()
	db.SetMaxOpenConns(2)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if err = db.PingContext(ctx); err != nil {
		return err
	}
	if os.Args[1] == "up" {
		if err = migration.Up(ctx, db); err != nil {
			return err
		}
		fmt.Println("migrations applied")
		return nil
	}
	var version int64
	if err = db.QueryRowContext(ctx, `SELECT version_id FROM goose_db_version WHERE is_applied ORDER BY id DESC LIMIT 1`).Scan(&version); err != nil {
		return err
	}
	fmt.Printf("schema version: %d\n", version)
	return nil
}
