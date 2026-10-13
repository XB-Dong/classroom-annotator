package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"github.com/XB-Dong/classroom-annotator/backend/internal/auth"
	"github.com/XB-Dong/classroom-annotator/backend/internal/platform/config"
	"github.com/XB-Dong/classroom-annotator/backend/internal/platform/store"
	_ "github.com/jackc/pgx/v5/stdlib"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func main() {
	username := flag.String("username", "", "Explicit administrator username")
	flag.Parse()
	if *username == "" {
		fail("username is required")
	}
	path := os.Getenv("APP_BOOTSTRAP_PASSWORD_FILE")
	if !filepath.IsAbs(path) {
		fail("absolute bootstrap password file is required")
	}
	f, e := os.Open(path)
	if e != nil {
		fail("password file unavailable")
	}
	raw, e := io.ReadAll(io.LimitReader(f, 131))
	f.Close()
	if e != nil || len(raw) > 130 {
		fail("password file invalid")
	}
	password := strings.TrimSuffix(strings.TrimSuffix(string(raw), "\n"), "\r")
	cfg, e := config.Load(os.Getenv)
	if e != nil {
		fail("configuration invalid")
	}
	db, e := sql.Open("pgx", cfg.DatabaseURL)
	if e != nil {
		fail("database unavailable")
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	s := store.Store{DB: db}
	if e = s.Ready(ctx); e != nil {
		fail("database schema is not ready")
	}
	service := auth.NewService(auth.NewPGRepository(db), "")
	u, e := service.BootstrapAdmin(ctx, *username, password)
	if e != nil {
		fail("administrator creation failed; existing accounts are not overwritten")
	}
	fmt.Printf("Created administrator %s in shared workspace\n", u.Username)
}
func fail(message string) { fmt.Fprintln(os.Stderr, message); os.Exit(1) }
