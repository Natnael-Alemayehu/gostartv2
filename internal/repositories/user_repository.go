package repositories

import (
	"context"
	"database/sql"

	"github.com/google/uuid"

	"gostartv2/internal/db/sqlc"
	"gostartv2/internal/models"
)

type UserRepository struct {
	q *sqlc.Queries
}

func NewUserRepository(q *sqlc.Queries) *UserRepository {
	return &UserRepository{q: q}
}

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

func (r *UserRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	row, err := r.q.GetUserByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return userToModel(row), nil
}

func (r *UserRepository) GetByEmail(ctx context.Context, email string) (*models.User, error) {
	row, err := r.q.GetUserByEmail(ctx, email)
	if err != nil {
		return nil, err
	}
	return userToModel(row), nil
}

func (r *UserRepository) List(ctx context.Context, limit, offset int32) ([]*models.User, error) {
	rows, err := r.q.ListUsers(ctx, sqlc.ListUsersParams{
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		return nil, err
	}
	users := make([]*models.User, 0, len(rows))
	for _, row := range rows {
		users = append(users, userToModel(row))
	}
	return users, nil
}

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

func (r *UserRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.q.DeleteUser(ctx, id)
}

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
