package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"gostartv2/internal/db/sqlc"
)

type Repositories struct {
	Users *UserRepository

	db *sql.DB
}

func NewRepositories(db *sql.DB) *Repositories {
	q := sqlc.New(db)
	return &Repositories{
		Users: NewUserRepository(q),
		db:    db,
	}
}

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
