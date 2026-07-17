// Package main is the entry point for the gostartv2 migration runner CLI.
// It applies, rolls back, and reports the status of the goose SQL migrations
// embedded in the binary. Subcommands: up, down, status, reset, redo.
package main

import (
	"database/sql"
	"errors"
	"fmt"
	"gostartv2/internal/config"
	"gostartv2/migrations"
	"log/slog"
	"os"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		slog.Error("migrate failed", "error", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return errors.New("no command given; expected one of: up, down, status, reset, redo")
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

	return runCommand(db, args[0])
}

func runCommand(db *sql.DB, cmd string) error {
	const dir = "."

	switch cmd {
	case "up":
		return wrapGooseErr("up", goose.Up(db, dir))
	case "down":
		return wrapGooseErr("down", goose.Down(db, dir))
	case "status":
		return wrapGooseErr("status", goose.Status(db, dir))
	case "reset":
		return wrapGooseErr("reset", goose.Reset(db, dir))
	case "redo":
		return wrapGooseErr("redo", goose.Redo(db, dir))
	default:
		return fmt.Errorf("unknown command %q; expected one of: up, down, status, reset, redo", cmd)
	}
}

func wrapGooseErr(cmd string, err error) error {
	if err != nil {
		return fmt.Errorf("%s: %w", cmd, err)
	}

	return nil
}
