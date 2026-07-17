package repositories

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/google/uuid"

	"gostartv2/internal/models"
	"gostartv2/internal/testutil"
)

func newRepo(t *testing.T) *Repositories {
	t.Helper()
	db := testutil.SetupTestDB(t)
	return NewRepositories(db)
}

func TestUserRepository_CreateAndGet(t *testing.T) {
	repos := newRepo(t)

	ctx := t.Context()

	created, err := repos.Users.Create(ctx, models.UserCreate{
		Email:        "alice@example.com",
		PasswordHash: "$2a$10$hash",
		Name:         "Alice",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.ID == uuid.Nil {
		t.Fatal("expected non-nil ID")
	}
	if created.CreatedAt.IsZero() {
		t.Fatal("expected non-zero CreatedAt")
	}

	fetched, err := repos.Users.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if fetched.Email != "alice@example.com" {
		t.Errorf("expected email alice@example.com, got %s", fetched.Email)
	}
	if fetched.PasswordHash != "$2a$10$hash" {
		t.Errorf("expected password hash, got %s", fetched.PasswordHash)
	}
}

func TestUserRepository_GetByEmail(t *testing.T) {
	repos := newRepo(t)

	ctx := t.Context()

	if _, err := repos.Users.Create(ctx, models.UserCreate{
		Email: "bob@example.com", PasswordHash: "h", Name: "Bob",
	}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := repos.Users.GetByEmail(ctx, "bob@example.com")
	if err != nil {
		t.Fatalf("GetByEmail: %v", err)
	}
	if got.Name != "Bob" {
		t.Errorf("expected name Bob, got %s", got.Name)
	}

	if _, err := repos.Users.GetByEmail(ctx, "missing@example.com"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("expected sql.ErrNoRows, got %v", err)
	}
}

func TestUserRepository_GetByID_NotFound(t *testing.T) {
	repos := newRepo(t)

	if _, err := repos.Users.GetByID(t.Context(), uuid.New()); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("expected sql.ErrNoRows, got %v", err)
	}
}

func TestUserRepository_Create_DuplicateEmail(t *testing.T) {
	repos := newRepo(t)

	ctx := t.Context()

	if _, err := repos.Users.Create(ctx, models.UserCreate{
		Email: "dup@example.com", PasswordHash: "h", Name: "Dup",
	}); err != nil {
		t.Fatalf("first Create: %v", err)
	}

	_, err := repos.Users.Create(ctx, models.UserCreate{
		Email: "dup@example.com", PasswordHash: "h", Name: "Dup2",
	})
	if err == nil {
		t.Fatal("expected duplicate email error, got nil")
	}
}

func TestUserRepository_List(t *testing.T) {
	repos := newRepo(t)

	ctx := t.Context()

	for i := range 3 {
		if _, err := repos.Users.Create(ctx, models.UserCreate{
			Email:        "user" + string(rune('a'+i)) + "@example.com",
			PasswordHash: "h",
			Name:         "User",
		}); err != nil {
			t.Fatalf("Create: %v", err)
		}
	}

	users, err := repos.Users.List(ctx, 10, 0)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(users) != 3 {
		t.Errorf("expected 3 users, got %d", len(users))
	}
}

func TestUserRepository_List_Pagination(t *testing.T) {
	repos := newRepo(t)

	ctx := t.Context()

	for i := range 5 {
		if _, err := repos.Users.Create(ctx, models.UserCreate{
			Email:        "p" + string(rune('a'+i)) + "@example.com",
			PasswordHash: "h",
			Name:         "P",
		}); err != nil {
			t.Fatalf("Create: %v", err)
		}
	}

	first, err := repos.Users.List(ctx, 2, 0)
	if err != nil {
		t.Fatalf("List page 1: %v", err)
	}
	if len(first) != 2 {
		t.Errorf("expected 2 users on page 1, got %d", len(first))
	}

	second, err := repos.Users.List(ctx, 2, 2)
	if err != nil {
		t.Fatalf("List page 2: %v", err)
	}
	if len(second) != 2 {
		t.Errorf("expected 2 users on page 2, got %d", len(second))
	}

	if first[0].ID == second[0].ID {
		t.Error("pagination returned the same user on different pages")
	}
}

func TestUserRepository_Update_Partial(t *testing.T) {
	repos := newRepo(t)

	ctx := t.Context()

	created, err := repos.Users.Create(ctx, models.UserCreate{
		Email: "update@example.com", PasswordHash: "old-hash", Name: "Old",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	newName := "New Name"
	updated, err := repos.Users.Update(ctx, created.ID, models.UserUpdate{
		Name: &newName,
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Name != "New Name" {
		t.Errorf("expected name New Name, got %s", updated.Name)
	}
	if updated.Email != "update@example.com" {
		t.Errorf("email should be unchanged, got %s", updated.Email)
	}
	if updated.PasswordHash != "old-hash" {
		t.Errorf("password_hash should be unchanged, got %s", updated.PasswordHash)
	}
}

func TestUserRepository_Update_AllFields(t *testing.T) {
	repos := newRepo(t)

	ctx := t.Context()

	created, err := repos.Users.Create(ctx, models.UserCreate{
		Email: "all@example.com", PasswordHash: "old", Name: "Old",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	newEmail := "new@example.com"
	newName := "New"
	newHash := "new-hash"
	updated, err := repos.Users.Update(ctx, created.ID, models.UserUpdate{
		Email:        &newEmail,
		Name:         &newName,
		PasswordHash: &newHash,
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Email != "new@example.com" || updated.Name != "New" || updated.PasswordHash != "new-hash" {
		t.Errorf("unexpected updated user: %+v", updated)
	}
}

func TestUserRepository_Delete(t *testing.T) {
	repos := newRepo(t)

	ctx := t.Context()

	created, err := repos.Users.Create(ctx, models.UserCreate{
		Email: "del@example.com", PasswordHash: "h", Name: "Del",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := repos.Users.Delete(ctx, created.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	if _, err := repos.Users.GetByID(ctx, created.ID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("expected sql.ErrNoRows after delete, got %v", err)
	}
}

func TestUserRepository_Count(t *testing.T) {
	repos := newRepo(t)

	ctx := t.Context()

	count, err := repos.Users.Count(ctx)
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 users, got %d", count)
	}

	for i := range 2 {
		if _, err := repos.Users.Create(ctx, models.UserCreate{
			Email:        "c" + string(rune('a'+i)) + "@example.com",
			PasswordHash: "h",
			Name:         "C",
		}); err != nil {
			t.Fatalf("Create: %v", err)
		}
	}

	count, err = repos.Users.Count(ctx)
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2 users, got %d", count)
	}
}

func TestRepositories_WithTx_Commits(t *testing.T) {
	repos := newRepo(t)

	ctx := t.Context()

	err := repos.WithTx(ctx, func(ctx context.Context, txR *Repositories) error {
		_, err := txR.Users.Create(ctx, models.UserCreate{
			Email: "tx-commit@example.com", PasswordHash: "h", Name: "Tx",
		})
		return err
	})
	if err != nil {
		t.Fatalf("WithTx: %v", err)
	}

	count, err := repos.Users.Count(ctx)
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 user after committed tx, got %d", count)
	}
}

func TestRepositories_WithTx_RollsBackOnError(t *testing.T) {
	repos := newRepo(t)

	ctx := t.Context()

	sentinel := errors.New("simulated failure")
	err := repos.WithTx(ctx, func(ctx context.Context, txR *Repositories) error {
		if _, err := txR.Users.Create(ctx, models.UserCreate{
			Email: "tx-rollback@example.com", PasswordHash: "h", Name: "Tx",
		}); err != nil {
			return err
		}
		return sentinel
	})
	if err == nil {
		t.Fatal("expected error from WithTx, got nil")
	}

	if !errors.Is(err, sentinel) {
		t.Fatalf("expected returned error to wrap the original cause; errors.Is failed. got: %v", err)
	}

	count, err := repos.Users.Count(ctx)
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 users after rolled-back tx, got %d", count)
	}
}
