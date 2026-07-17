package services

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/google/uuid"

	"gostartv2/internal/models"
)

type mockUserRepo struct {
	users map[uuid.UUID]*models.User

	createFn func(ctx context.Context, params models.UserCreate) (*models.User, error)
	getFn    func(ctx context.Context, id uuid.UUID) (*models.User, error)
	listFn   func(ctx context.Context, limit, offset int32) ([]*models.User, error)
	updateFn func(ctx context.Context, id uuid.UUID, params models.UserUpdate) (*models.User, error)
	deleteFn func(ctx context.Context, id uuid.UUID) error
}

func newMockUserRepo() *mockUserRepo {
	return &mockUserRepo{users: make(map[uuid.UUID]*models.User)}
}

func (m *mockUserRepo) Create(ctx context.Context, params models.UserCreate) (*models.User, error) {
	if m.createFn != nil {
		return m.createFn(ctx, params)
	}
	u := &models.User{
		ID:           uuid.New(),
		Email:        params.Email,
		PasswordHash: params.PasswordHash,
		Name:         params.Name,
	}
	m.users[u.ID] = u
	return u, nil
}

func (m *mockUserRepo) GetByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	if m.getFn != nil {
		return m.getFn(ctx, id)
	}
	u, ok := m.users[id]
	if !ok {
		return nil, sql.ErrNoRows
	}
	return u, nil
}

func (m *mockUserRepo) GetByEmail(ctx context.Context, email string) (*models.User, error) {
	for _, u := range m.users {
		if u.Email == email {
			return u, nil
		}
	}
	return nil, sql.ErrNoRows
}

func (m *mockUserRepo) List(ctx context.Context, limit, offset int32) ([]*models.User, error) {
	if m.listFn != nil {
		return m.listFn(ctx, limit, offset)
	}
	users := make([]*models.User, 0, len(m.users))
	for _, u := range m.users {
		users = append(users, u)
	}
	return users, nil
}

func (m *mockUserRepo) Update(ctx context.Context, id uuid.UUID, params models.UserUpdate) (*models.User, error) {
	if m.updateFn != nil {
		return m.updateFn(ctx, id, params)
	}
	u, ok := m.users[id]
	if !ok {
		return nil, sql.ErrNoRows
	}
	if params.Email != nil {
		u.Email = *params.Email
	}
	if params.Name != nil {
		u.Name = *params.Name
	}
	if params.PasswordHash != nil {
		u.PasswordHash = *params.PasswordHash
	}
	return u, nil
}

func (m *mockUserRepo) Delete(ctx context.Context, id uuid.UUID) error {
	if m.deleteFn != nil {
		return m.deleteFn(ctx, id)
	}
	if _, ok := m.users[id]; !ok {
		return sql.ErrNoRows
	}
	delete(m.users, id)
	return nil
}

func TestUserService_Create(t *testing.T) {
	repo := newMockUserRepo()
	svc := NewUserService(repo)

	user, err := svc.Create(context.Background(), CreateUserInput{
		Email:    "alice@example.com",
		Password: "supersecret",
		Name:     "Alice",
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if user.Email != "alice@example.com" {
		t.Errorf("expected email alice@example.com, got %s", user.Email)
	}
	if user.PasswordHash == "" || user.PasswordHash == "supersecret" {
		t.Error("expected password to be hashed, not empty or plaintext")
	}
	if user.ID == uuid.Nil {
		t.Error("expected non-nil ID")
	}
}

func TestUserService_Create_HashesPassword(t *testing.T) {
	repo := newMockUserRepo()
	svc := NewUserService(repo)

	user, err := svc.Create(context.Background(), CreateUserInput{
		Email:    "bob@example.com",
		Password: "supersecret",
		Name:     "Bob",
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	stored := repo.users[user.ID]
	if stored.PasswordHash == "supersecret" {
		t.Fatal("password must not be stored in plaintext")
	}
}

func TestUserService_Get_NotFound(t *testing.T) {
	repo := newMockUserRepo()
	svc := NewUserService(repo)

	_, err := svc.Get(context.Background(), uuid.New())
	if !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("expected ErrUserNotFound, got %v", err)
	}
}

func TestUserService_GetByEmail_NotFound(t *testing.T) {
	repo := newMockUserRepo()
	svc := NewUserService(repo)

	_, err := svc.GetByEmail(context.Background(), "missing@example.com")
	if !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("expected ErrUserNotFound, got %v", err)
	}
}

func TestUserService_List_DefaultsAndClamps(t *testing.T) {
	repo := newMockUserRepo()
	svc := NewUserService(repo)

	calls := []struct{ limit, offset int32 }{}
	repo.listFn = func(ctx context.Context, limit, offset int32) ([]*models.User, error) {
		calls = append(calls, struct{ limit, offset int32 }{limit, offset})
		return nil, nil
	}

	_, _ = svc.List(context.Background(), 0, -1)
	if calls[0].limit != 20 || calls[0].offset != 0 {
		t.Errorf("expected default limit=20 offset=0, got limit=%d offset=%d", calls[0].limit, calls[0].offset)
	}

	_, _ = svc.List(context.Background(), 500, 0)
	if calls[1].limit != 100 {
		t.Errorf("expected clamped limit=100, got %d", calls[1].limit)
	}
}

func TestUserService_Update(t *testing.T) {
	repo := newMockUserRepo()
	svc := NewUserService(repo)

	user, err := svc.Create(context.Background(), CreateUserInput{
		Email: "carol@example.com", Password: "supersecret", Name: "Carol",
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	newName := "Carol Updated"
	newPassword := "newpassword123"
	updated, err := svc.Update(context.Background(), user.ID, UpdateUserInput{
		Name:     &newName,
		Password: &newPassword,
	})
	if err != nil {
		t.Fatalf("Update returned error: %v", err)
	}
	if updated.Name != "Carol Updated" {
		t.Errorf("expected name Carol Updated, got %s", updated.Name)
	}
	if updated.PasswordHash == "supersecret" || updated.PasswordHash == "newpassword123" {
		t.Error("expected updated password to be hashed")
	}
}

func TestUserService_Update_NotFound(t *testing.T) {
	repo := newMockUserRepo()
	svc := NewUserService(repo)

	name := "x"
	_, err := svc.Update(context.Background(), uuid.New(), UpdateUserInput{Name: &name})
	if !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("expected ErrUserNotFound, got %v", err)
	}
}

func TestUserService_Delete(t *testing.T) {
	repo := newMockUserRepo()
	svc := NewUserService(repo)

	user, err := svc.Create(context.Background(), CreateUserInput{
		Email: "dave@example.com", Password: "supersecret", Name: "Dave",
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	if err := svc.Delete(context.Background(), user.ID); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}

	if _, err := svc.Get(context.Background(), user.ID); !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("expected ErrUserNotFound after delete, got %v", err)
	}
}
