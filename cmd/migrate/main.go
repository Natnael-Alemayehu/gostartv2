package main

import (
	"database/sql"
	"fmt"
	"log/slog"
	"os"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"gostartv2/internal/config"
	"gostartv2/migrations"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		slog.Error("migrate failed", "error", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no command given; expected one of: up, down, status, reset, redo")
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	db, err := sql.Open("pgx", cfg.DB.DSN())
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			slog.Error("close db", "error", err)
		}
	}()

	goose.SetBaseFS(migrations.FS)
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("set dialect: %w", err)
	}

	const dir = "."

	cmd := args[0]
	switch cmd {
	case "up":
		if err := goose.Up(db, dir); err != nil {
			return fmt.Errorf("up: %w", err)
		}
	case "down":
		if err := goose.Down(db, dir); err != nil {
			return fmt.Errorf("down: %w", err)
		}
	case "status":
		if err := goose.Status(db, dir); err != nil {
			return fmt.Errorf("status: %w", err)
		}
	case "reset":
		if err := goose.Reset(db, dir); err != nil {
			return fmt.Errorf("reset: %w", err)
		}
	case "redo":
		if err := goose.Redo(db, dir); err != nil {
			return fmt.Errorf("redo: %w", err)
		}
	default:
		return fmt.Errorf("unknown command %q; expected one of: up, down, status, reset, redo", cmd)
	}

	return nil
}
