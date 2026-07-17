package services

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"golang.org/x/crypto/bcrypt"

	"gostartv2/internal/models"
)

type userRepo interface {
	Create(ctx context.Context, params models.UserCreate) (*models.User, error)
	GetByID(ctx context.Context, id uuid.UUID) (*models.User, error)
	GetByEmail(ctx context.Context, email string) (*models.User, error)
	List(ctx context.Context, limit, offset int32) ([]*models.User, error)
	Update(ctx context.Context, id uuid.UUID, params models.UserUpdate) (*models.User, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type UserService struct {
	repo userRepo
}

func NewUserService(repo userRepo) *UserService {
	return &UserService{repo: repo}
}

type CreateUserInput struct {
	Email    string
	Password string
	Name     string
}

type UpdateUserInput struct {
	Email    *string
	Password *string
	Name     *string
}

func (s *UserService) Create(ctx context.Context, input CreateUserInput) (*models.User, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	user, err := s.repo.Create(ctx, models.UserCreate{
		Email:        input.Email,
		PasswordHash: string(hash),
		Name:         input.Name,
	})
	if err != nil {
		if isUniqueViolation(err) {
			return nil, ErrEmailAlreadyExists
		}
		return nil, fmt.Errorf("create user: %w", err)
	}
	return user, nil
}

func (s *UserService) Get(ctx context.Context, id uuid.UUID) (*models.User, error) {
	user, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("get user: %w", err)
	}
	return user, nil
}

func (s *UserService) GetByEmail(ctx context.Context, email string) (*models.User, error) {
	user, err := s.repo.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("get user by email: %w", err)
	}
	return user, nil
}

func (s *UserService) List(ctx context.Context, limit, offset int32) ([]*models.User, error) {
	if limit <= 0 {
		limit = 20
	}
	limit = min(limit, 100)
	offset = max(offset, 0)

	users, err := s.repo.List(ctx, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	return users, nil
}

func (s *UserService) Update(ctx context.Context, id uuid.UUID, input UpdateUserInput) (*models.User, error) {
	params := models.UserUpdate{
		Email: input.Email,
		Name:  input.Name,
	}

	if input.Password != nil {
		hash, err := bcrypt.GenerateFromPassword([]byte(*input.Password), bcrypt.DefaultCost)
		if err != nil {
			return nil, fmt.Errorf("hash password: %w", err)
		}
		params.PasswordHash = new(string(hash))
	}

	user, err := s.repo.Update(ctx, id, params)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		if isUniqueViolation(err) {
			return nil, ErrEmailAlreadyExists
		}
		return nil, fmt.Errorf("update user: %w", err)
	}
	return user, nil
}

func (s *UserService) Delete(ctx context.Context, id uuid.UUID) error {
	err := s.repo.Delete(ctx, id)
	if err != nil {
		return fmt.Errorf("delete user: %w", err)
	}
	return nil
}

func isUniqueViolation(err error) bool {
	if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok {
		return pgErr.Code == "23505"
	}
	return false
}
