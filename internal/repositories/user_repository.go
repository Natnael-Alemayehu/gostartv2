package repositories

import (
	"context"
	"database/sql"
	"gostartv2/internal/db/sqlc"
	"gostartv2/internal/models"

	"github.com/google/uuid"
)

// UserRepository persists and retrieves user records via sqlc-generated
// queries. It performs no business logic and returns domain models.
type UserRepository struct {
	q *sqlc.Queries
}

// NewUserRepository returns a UserRepository that issues queries through the
// given sqlc.Queries instance, which may wrap either a *sql.DB or a
// transaction.
func NewUserRepository(q *sqlc.Queries) *UserRepository {
	return &UserRepository{q: q}
}

// Create inserts a new user with the provided fields and returns the resulting
// domain user. Callers are responsible for hashing the password beforehand.
func (r *UserRepository) Create(ctx context.Context, params models.UserCreate) (*models.User, error) {
	row, err := r.q.CreateUser(ctx, sqlc.CreateUserParams{
		Email:        params.Email,
		PasswordHash: params.PasswordHash,
		Name:         params.Name,
	})
	if err != nil {
		return nil, err
	}

	return userToModel(row), nil
}

// GetByID fetches the user with the given UUID. A sql.ErrNoRows error is
// returned to the caller untranslated when no row matches.
func (r *UserRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	row, err := r.q.GetUserByID(ctx, id)
	if err != nil {
		return nil, err
	}

	return userToModel(row), nil
}

// GetByEmail fetches the user matching the given email address. Used by the
// service layer for credential lookup and uniqueness checks.
func (r *UserRepository) GetByEmail(ctx context.Context, email string) (*models.User, error) {
	row, err := r.q.GetUserByEmail(ctx, email)
	if err != nil {
		return nil, err
	}

	return userToModel(row), nil
}

// List returns a page of users ordered by created_at DESC, id DESC. When
// cursor is nil the first page is returned; otherwise the page starts after
// the row identified by the cursor. limit bounds the page size.
func (r *UserRepository) List(ctx context.Context, input models.ListUsersInput) ([]*models.User, error) {
	params := sqlc.ListUsersParams{
		MaxRows: input.Limit,
	}

	if input.Cursor != nil {
		params.CursorCreatedAt = sql.NullTime{Time: input.Cursor.CreatedAt, Valid: true}
		params.CursorID = uuid.NullUUID{UUID: input.Cursor.ID, Valid: true}
	}

	rows, err := r.q.ListUsers(ctx, params)
	if err != nil {
		return nil, err
	}

	users := make([]*models.User, 0, len(rows))
	for _, row := range rows {
		users = append(users, userToModel(row))
	}

	return users, nil
}

// Update patches the user identified by id with the non-nil fields of params
// and returns the updated row. Nil pointer fields are left unchanged.
func (r *UserRepository) Update(ctx context.Context, id uuid.UUID, params models.UserUpdate) (*models.User, error) {
	row, err := r.q.UpdateUser(ctx, sqlc.UpdateUserParams{
		ID:           id,
		Email:        toNullString(params.Email),
		PasswordHash: toNullString(params.PasswordHash),
		Name:         toNullString(params.Name),
	})
	if err != nil {
		return nil, err
	}

	return userToModel(row), nil
}

// Delete removes the user with the given id. It is idempotent with respect to
// missing rows only if the underlying query does not enforce existence.
func (r *UserRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.q.DeleteUser(ctx, id)
}

// Count returns the total number of user rows in the table, used for
// pagination metadata and readiness checks.
func (r *UserRepository) Count(ctx context.Context) (int64, error) {
	return r.q.CountUsers(ctx)
}

func userToModel(row sqlc.User) *models.User {
	return &models.User{
		ID:           row.ID,
		Email:        row.Email,
		PasswordHash: row.PasswordHash,
		Name:         row.Name,
		CreatedAt:    row.CreatedAt,
		UpdatedAt:    row.UpdatedAt,
	}
}

func toNullString(s *string) sql.NullString {
	if s == nil {
		return sql.NullString{}
	}

	return sql.NullString{String: *s, Valid: true}
}
