package services

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"gostartv2/internal/models"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"golang.org/x/crypto/bcrypt"
)

type userRepo interface {
	Create(ctx context.Context, params models.UserCreate) (*models.User, error)
	GetByID(ctx context.Context, id uuid.UUID) (*models.User, error)
	GetByEmail(ctx context.Context, email string) (*models.User, error)
	List(ctx context.Context, input models.ListUsersInput) ([]*models.User, error)
	Update(ctx context.Context, id uuid.UUID, params models.UserUpdate) (*models.User, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

// UserService applies the business rules around user accounts: password
// hashing, pagination clamping, and translation of repository errors into
// domain sentinels. It has no HTTP awareness.
type UserService struct {
	repo userRepo
}

// NewUserService returns a UserService backed by the given repository. The
// repository is accepted as the consumer-defined userRepo interface so the
// service can be unit-tested with a fake.
func NewUserService(repo userRepo) *UserService {
	return &UserService{repo: repo}
}

// CreateUserInput is the service-level request to register a new user. The
// Password is plaintext and is hashed before being persisted.
type CreateUserInput struct {
	Email    string
	Password string
	Name     string
}

// UpdateUserInput is the service-level request to patch an existing user.
// Each pointer field is optional; a nil field leaves that column unchanged so
// callers can update a subset of fields.
type UpdateUserInput struct {
	Email    *string
	Password *string
	Name     *string
}

// Create hashes the password, persists the user, and returns the resulting
// domain user. A unique-constraint violation from the repository is translated
// into ErrEmailAlreadyExists.
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

// Get returns the user with the given id. A missing row is translated into
// ErrUserNotFound.
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

// GetByEmail returns the user matching the given email, used primarily for
// credential lookup. A missing row is translated into ErrUserNotFound.
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

// ListResult is the service-level response for a paginated user list. It
// contains the users for the current page and a NextCursor that the caller
// can pass back to fetch the following page. NextCursor is nil when there
// are no more rows.
type ListResult struct {
	Users      []*models.User
	NextCursor *models.PageCursor
}

// List returns a page of users with the caller's limit clamped to sane bounds
// (default 20, maximum 100). When cursor is nil the first page is returned;
// otherwise the page starts after the row identified by the cursor. The
// returned NextCursor is non-nil when more rows may exist.
func (s *UserService) List(ctx context.Context, limit int32, cursor *models.PageCursor) (*ListResult, error) {
	if limit <= 0 {
		limit = 20
	}

	limit = min(limit, 100)

	users, err := s.repo.List(ctx, models.ListUsersInput{
		Limit:  limit,
		Cursor: cursor,
	})
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}

	result := &ListResult{
		Users: users,
	}

	if len(users) == int(limit) {
		last := users[len(users)-1]
		result.NextCursor = &models.PageCursor{
			CreatedAt: last.CreatedAt,
			ID:        last.ID,
		}
	}

	return result, nil
}

// Update patches the user identified by id with the non-nil fields of input.
// When Password is supplied it is hashed before persistence. Missing-row and
// unique-constraint outcomes are translated into ErrUserNotFound and
// ErrEmailAlreadyExists respectively.
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

// Delete removes the user with the given id. The repository error, if any, is
// wrapped and returned unchanged for the handler to translate.
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
