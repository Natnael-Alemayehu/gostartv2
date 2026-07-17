// Package repositories provides hand-written repositories over sqlc-generated
// queries, plus transaction helpers for running multiple repository operations
// atomically. Dependencies flow inward from services; repositories contain no
// business logic.
package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"gostartv2/internal/db/sqlc"
)

// Repositories groups the individual repository types used by the service
// layer. It is the unit passed around at runtime and the unit scoped to a
// transaction by WithTx.
type Repositories struct {
	Users *UserRepository

	db *sql.DB
}

// NewRepositories builds a Repositories value backed by the given database
// connection. Each sub-repository shares the same sqlc.Queries instance.
func NewRepositories(db *sql.DB) *Repositories {
	q := sqlc.New(db)

	return &Repositories{
		Users: NewUserRepository(q),
		db:    db,
	}
}

// WithTx executes fn within a database transaction, providing fn with a fresh
// Repositories value whose queries run against the transaction. The
// transaction is committed when fn returns nil and rolled back otherwise; a
// panic in fn triggers a rollback and is re-propagated. Use this to keep
// multi-step repository operations atomic.
func (r *Repositories) WithTx(ctx context.Context, fn func(ctx context.Context, txR *Repositories) error) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}

	txR := &Repositories{
		Users: NewUserRepository(sqlc.New(tx)),
	}

	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()

			panic(p)
		}
	}()

	if err := fn(ctx, txR); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return errors.Join(err, fmt.Errorf("rollback: %w", rbErr))
		}

		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	return nil
}
