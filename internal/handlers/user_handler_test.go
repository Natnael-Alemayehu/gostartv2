package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"gostartv2/internal/models"
	"gostartv2/internal/services"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type mockUserService struct {
	createFn func(ctx context.Context, input services.CreateUserInput) (*models.User, error)
	getFn    func(ctx context.Context, id uuid.UUID) (*models.User, error)
	listFn   func(ctx context.Context, limit int32, cursor *models.PageCursor) (*services.ListResult, error)
	updateFn func(ctx context.Context, id uuid.UUID, input services.UpdateUserInput) (*models.User, error)
	deleteFn func(ctx context.Context, id uuid.UUID) error
}

func (m *mockUserService) Create(ctx context.Context, input services.CreateUserInput) (*models.User, error) {
	if m.createFn != nil {
		return m.createFn(ctx, input)
	}

	return &models.User{
		ID: uuid.New(), Email: input.Email, Name: input.Name,
	}, nil
}

func (m *mockUserService) Get(ctx context.Context, id uuid.UUID) (*models.User, error) {
	if m.getFn != nil {
		return m.getFn(ctx, id)
	}

	return nil, services.ErrUserNotFound
}

func (m *mockUserService) List(ctx context.Context, limit int32, cursor *models.PageCursor) (*services.ListResult, error) {
	if m.listFn != nil {
		return m.listFn(ctx, limit, cursor)
	}

	return &services.ListResult{Users: nil}, nil
}

func (m *mockUserService) Update(ctx context.Context, id uuid.UUID, input services.UpdateUserInput) (*models.User, error) {
	if m.updateFn != nil {
		return m.updateFn(ctx, id, input)
	}

	return nil, services.ErrUserNotFound
}

func (m *mockUserService) Delete(ctx context.Context, id uuid.UUID) error {
	if m.deleteFn != nil {
		return m.deleteFn(ctx, id)
	}

	return nil
}

func newTestRouter(h *UserHandler) http.Handler {
	r := chi.NewRouter()
	r.Route("/api/v1/users", func(r chi.Router) {
		r.Get("/", h.List)
		r.Post("/", h.Create)
		r.Get("/{id}", h.Get)
		r.Put("/{id}", h.Update)
		r.Delete("/{id}", h.Delete)
	})

	return r
}

func doRequest(t *testing.T, h *UserHandler, method, target string, body any) *http.Response {
	t.Helper()

	var reqBody io.Reader

	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}

		reqBody = bytes.NewReader(b)
	}

	req := httptest.NewRequest(method, target, reqBody)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	w := httptest.NewRecorder()
	newTestRouter(h).ServeHTTP(w, req)

	return w.Result()
}

func decodeBody(t *testing.T, resp *http.Response) map[string]any {
	t.Helper()

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}

	var out map[string]any
	if len(b) == 0 {
		return out
	}

	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("decode body %q: %v", string(b), err)
	}

	return out
}

func TestUserHandler_Create(t *testing.T) {
	h := NewUserHandler(&mockUserService{})

	resp := doRequest(t, h, http.MethodPost, "/api/v1/users", map[string]string{
		"email":    "alice@example.com",
		"password": "supersecret",
		"name":     "Alice",
	})
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}

	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("expected json content-type, got %s", ct)
	}

	body := decodeBody(t, resp)
	if body["email"] != "alice@example.com" {
		t.Errorf("expected email in response, got %v", body["email"])
	}

	if _, ok := body["password_hash"]; ok {
		t.Error("password_hash must not be exposed in response")
	}
}

func TestUserHandler_Create_ValidationError(t *testing.T) {
	h := NewUserHandler(&mockUserService{})

	resp := doRequest(t, h, http.MethodPost, "/api/v1/users", map[string]string{
		"email":    "not-an-email",
		"password": "short",
		"name":     "",
	})
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}

	body := decodeBody(t, resp)

	errObj, ok := body["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected error envelope, got %v", body)
	}

	if errObj["code"] != "validation_failed" {
		t.Errorf("expected code validation_failed, got %v", errObj["code"])
	}
}

func TestUserHandler_Create_InvalidJSON(t *testing.T) {
	h := NewUserHandler(&mockUserService{})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/users", strings.NewReader("{not json"))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	newTestRouter(h).ServeHTTP(w, req)

	resp := w.Result()
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestUserHandler_Create_EmailConflict(t *testing.T) {
	h := NewUserHandler(&mockUserService{
		createFn: func(ctx context.Context, input services.CreateUserInput) (*models.User, error) {
			return nil, services.ErrEmailAlreadyExists
		},
	})

	resp := doRequest(t, h, http.MethodPost, "/api/v1/users", map[string]string{
		"email": "dup@example.com", "password": "supersecret", "name": "Dup",
	})
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409, got %d", resp.StatusCode)
	}
}

func TestUserHandler_Get_NotFound(t *testing.T) {
	h := NewUserHandler(&mockUserService{})

	id := uuid.New()

	resp := doRequest(t, h, http.MethodGet, "/api/v1/users/"+id.String(), nil)
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestUserHandler_Get_InvalidUUID(t *testing.T) {
	h := NewUserHandler(&mockUserService{})

	resp := doRequest(t, h, http.MethodGet, "/api/v1/users/not-a-uuid", nil)
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestUserHandler_Get_OK(t *testing.T) {
	id := uuid.New()
	h := NewUserHandler(&mockUserService{
		getFn: func(ctx context.Context, reqID uuid.UUID) (*models.User, error) {
			if reqID != id {
				t.Errorf("unexpected id: %v", reqID)
			}

			return &models.User{ID: id, Email: "alice@example.com", Name: "Alice"}, nil
		},
	})

	resp := doRequest(t, h, http.MethodGet, "/api/v1/users/"+id.String(), nil)
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	body := decodeBody(t, resp)
	if body["id"] != id.String() {
		t.Errorf("expected id %s, got %v", id, body["id"])
	}
}

func TestUserHandler_List(t *testing.T) {
	h := NewUserHandler(&mockUserService{
		listFn: func(ctx context.Context, limit int32, cursor *models.PageCursor) (*services.ListResult, error) {
			return &services.ListResult{
				Users: []*models.User{
					{ID: uuid.New(), Email: "a@example.com", Name: "A"},
					{ID: uuid.New(), Email: "b@example.com", Name: "B"},
				},
			}, nil
		},
	})

	resp := doRequest(t, h, http.MethodGet, "/api/v1/users?limit=10", nil)
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	body := decodeBody(t, resp)

	users, ok := body["users"].([]any)
	if !ok {
		t.Fatalf("expected users array, got %v", body)
	}

	if len(users) != 2 {
		t.Errorf("expected 2 users, got %d", len(users))
	}
}

func TestUserHandler_Update(t *testing.T) {
	id := uuid.New()
	h := NewUserHandler(&mockUserService{
		updateFn: func(ctx context.Context, reqID uuid.UUID, input services.UpdateUserInput) (*models.User, error) {
			if reqID != id {
				t.Errorf("unexpected id: %v", reqID)
			}

			return &models.User{ID: id, Email: "new@example.com", Name: "New"}, nil
		},
	})

	resp := doRequest(t, h, http.MethodPut, "/api/v1/users/"+id.String(), map[string]any{
		"name": "New",
	})
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestUserHandler_Update_NotFound(t *testing.T) {
	h := NewUserHandler(&mockUserService{
		updateFn: func(ctx context.Context, id uuid.UUID, input services.UpdateUserInput) (*models.User, error) {
			return nil, services.ErrUserNotFound
		},
	})

	resp := doRequest(t, h, http.MethodPut, "/api/v1/users/"+uuid.New().String(), map[string]any{
		"name": "New",
	})
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestUserHandler_Delete(t *testing.T) {
	id := uuid.New()
	deleted := false
	h := NewUserHandler(&mockUserService{
		deleteFn: func(ctx context.Context, reqID uuid.UUID) error {
			deleted = true
			return nil
		},
	})

	resp := doRequest(t, h, http.MethodDelete, "/api/v1/users/"+id.String(), nil)
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}

	if !deleted {
		t.Error("expected service Delete to be called")
	}
}

func TestUserHandler_InternalError(t *testing.T) {
	h := NewUserHandler(&mockUserService{
		getFn: func(ctx context.Context, id uuid.UUID) (*models.User, error) {
			return nil, errors.New("db exploded")
		},
	})

	resp := doRequest(t, h, http.MethodGet, "/api/v1/users/"+uuid.New().String(), nil)
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", resp.StatusCode)
	}

	body := decodeBody(t, resp)

	errObj, _ := body["error"].(map[string]any)
	if errObj["code"] != "internal_error" {
		t.Errorf("expected code internal_error, got %v", errObj["code"])
	}
}
