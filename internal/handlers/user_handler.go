// Package handlers implements the HTTP transport layer. Handlers parse and
// validate requests, call services, and translate service errors into HTTP
// responses via the httpx helpers. They perform no database access and hold no
// business logic.
package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"gostartv2/internal/httpx"
	"gostartv2/internal/models"
	"gostartv2/internal/services"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
)

//nolint:dupl // test mock intentionally mirrors this interface
type userService interface {
	Create(ctx context.Context, input services.CreateUserInput) (*models.User, error)
	Get(ctx context.Context, id uuid.UUID) (*models.User, error)
	List(ctx context.Context, limit int32, cursor *models.PageCursor) (*services.ListResult, error)
	Update(ctx context.Context, id uuid.UUID, input services.UpdateUserInput) (*models.User, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

// UserHandler exposes the user resource as HTTP endpoints. It owns request
// decoding, validation, and the mapping between service errors and HTTP status
// codes.
type UserHandler struct {
	svc       userService
	validator *validator.Validate
}

// NewUserHandler returns a UserHandler backed by the given service. A fresh
// validator instance is constructed for request validation.
func NewUserHandler(svc userService) *UserHandler {
	return &UserHandler{
		svc:       svc,
		validator: validator.New(),
	}
}

type createUserRequest struct {
	Email    string `json:"email" validate:"required,email,max=254"`
	Password string `json:"password" validate:"required,min=8,max=72"`
	Name     string `json:"name" validate:"required,min=1,max=100"`
}

type updateUserRequest struct {
	Email    *string `json:"email" validate:"omitempty,email,max=254"`
	Password *string `json:"password" validate:"omitempty,min=8,max=72"`
	Name     *string `json:"name" validate:"omitempty,min=1,max=100"`
}

type userResponse struct {
	ID        uuid.UUID `json:"id"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type cursorResponse struct {
	CreatedAt time.Time `json:"created_at"`
	ID        uuid.UUID `json:"id"`
}

type listUsersResponse struct {
	Users      []userResponse  `json:"users"`
	NextCursor *cursorResponse `json:"next_cursor,omitempty"`
}

// Create handles POST requests that register a new user. It decodes and
// validates the request body, delegates to the user service, and responds with
// the created user and a 201 status.
func (h *UserHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req createUserRequest
	if !decodeAndValidate(w, r, h.validator, &req) {
		return
	}

	user, err := h.svc.Create(r.Context(), services.CreateUserInput{
		Email:    req.Email,
		Password: req.Password,
		Name:     req.Name,
	})
	if err != nil {
		respondServiceError(w, err)
		return
	}

	httpx.RespondJSON(w, http.StatusCreated, toUserResponse(user))
}

// Get handles GET requests for a single user by id path parameter. An invalid
// or missing UUID yields a 400; a missing user yields a 404.
func (h *UserHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUserID(w, r)
	if !ok {
		return
	}

	user, err := h.svc.Get(r.Context(), id)
	if err != nil {
		respondServiceError(w, err)
		return
	}

	httpx.RespondJSON(w, http.StatusOK, toUserResponse(user))
}

// List handles GET requests returning a page of users. Pagination uses
// cursor-based paging: the first page omits the cursor query params, and
// subsequent pages pass back the next_cursor value from the previous response.
func (h *UserHandler) List(w http.ResponseWriter, r *http.Request) {
	limit := parseLimit(r)
	cursor := parseCursor(r)

	result, err := h.svc.List(r.Context(), limit, cursor)
	if err != nil {
		respondServiceError(w, err)
		return
	}

	resp := listUsersResponse{Users: make([]userResponse, 0, len(result.Users))}
	for _, u := range result.Users {
		resp.Users = append(resp.Users, toUserResponse(u))
	}

	if result.NextCursor != nil {
		resp.NextCursor = &cursorResponse{
			CreatedAt: result.NextCursor.CreatedAt,
			ID:        result.NextCursor.ID,
		}
	}

	httpx.RespondJSON(w, http.StatusOK, resp)
}

// Update handles PATCH requests that modify an existing user identified by the
// id path parameter. Only the fields present in the request body are changed.
func (h *UserHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUserID(w, r)
	if !ok {
		return
	}

	var req updateUserRequest
	if !decodeAndValidate(w, r, h.validator, &req) {
		return
	}

	user, err := h.svc.Update(r.Context(), id, services.UpdateUserInput{
		Email:    req.Email,
		Password: req.Password,
		Name:     req.Name,
	})
	if err != nil {
		respondServiceError(w, err)
		return
	}

	httpx.RespondJSON(w, http.StatusOK, toUserResponse(user))
}

// Delete handles DELETE requests that remove the user identified by the id
// path parameter. On success it responds with 204 No Content.
func (h *UserHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUserID(w, r)
	if !ok {
		return
	}

	if err := h.svc.Delete(r.Context(), id); err != nil {
		respondServiceError(w, err)
		return
	}

	httpx.RespondNoContent(w)
}

func toUserResponse(u *models.User) userResponse {
	return userResponse{
		ID:        u.ID,
		Email:     u.Email,
		Name:      u.Name,
		CreatedAt: u.CreatedAt,
		UpdatedAt: u.UpdatedAt,
	}
}

func parseUserID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	raw := chi.URLParam(r, "id")

	id, err := uuid.Parse(raw)
	if err != nil {
		httpx.RespondError(w, http.StatusBadRequest, "invalid_id", "user id must be a valid UUID")
		return uuid.Nil, false
	}

	return id, true
}

func parseLimit(r *http.Request) int32 {
	limit := int32(20)

	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 100 {
			limit = int32(n) //nolint:gosec // bounds checked: n <= 100
		}
	}

	return limit
}

func parseCursor(r *http.Request) *models.PageCursor {
	createdAtStr := r.URL.Query().Get("cursor_created_at")

	idStr := r.URL.Query().Get("cursor_id")
	if createdAtStr == "" || idStr == "" {
		return nil
	}

	createdAt, err := time.Parse(time.RFC3339Nano, createdAtStr)
	if err != nil {
		return nil
	}

	id, err := uuid.Parse(idStr)
	if err != nil {
		return nil
	}

	return &models.PageCursor{
		CreatedAt: createdAt,
		ID:        id,
	}
}

func respondServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, services.ErrUserNotFound):
		httpx.RespondError(w, http.StatusNotFound, "user_not_found", "user not found")
	case errors.Is(err, services.ErrEmailAlreadyExists):
		httpx.RespondError(w, http.StatusConflict, "email_already_exists", "a user with that email already exists")
	default:
		httpx.RespondError(w, http.StatusInternalServerError, "internal_error", "something went wrong")
	}
}

func decodeAndValidate(w http.ResponseWriter, r *http.Request, v *validator.Validate, dst any) bool {
	if err := httpx.DecodeJSON(r, dst); err != nil {
		if _, ok := errors.AsType[*json.SyntaxError](err); ok {
			httpx.RespondError(w, http.StatusBadRequest, "invalid_body", "request body is not valid JSON")
		} else if _, ok := errors.AsType[*json.UnmarshalTypeError](err); ok {
			httpx.RespondError(w, http.StatusBadRequest, "invalid_body", "request body is not valid JSON")
		} else {
			httpx.RespondError(w, http.StatusBadRequest, "invalid_body", err.Error())
		}

		return false
	}

	if err := v.Struct(dst); err != nil {
		httpx.RespondError(w, http.StatusBadRequest, "validation_failed", err.Error())
		return false
	}

	return true
}
