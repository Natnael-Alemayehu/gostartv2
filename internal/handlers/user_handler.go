package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"

	"gostartv2/internal/httpx"
	"gostartv2/internal/models"
	"gostartv2/internal/services"
)

type userService interface {
	Create(ctx context.Context, input services.CreateUserInput) (*models.User, error)
	Get(ctx context.Context, id uuid.UUID) (*models.User, error)
	List(ctx context.Context, limit, offset int32) ([]*models.User, error)
	Update(ctx context.Context, id uuid.UUID, input services.UpdateUserInput) (*models.User, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type UserHandler struct {
	svc       userService
	validator *validator.Validate
}

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

type listUsersResponse struct {
	Users []userResponse `json:"users"`
}

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

func (h *UserHandler) List(w http.ResponseWriter, r *http.Request) {
	limit, offset := parsePagination(r)

	users, err := h.svc.List(r.Context(), limit, offset)
	if err != nil {
		respondServiceError(w, err)
		return
	}

	resp := listUsersResponse{Users: make([]userResponse, 0, len(users))}
	for _, u := range users {
		resp.Users = append(resp.Users, toUserResponse(u))
	}

	httpx.RespondJSON(w, http.StatusOK, resp)
}

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

func parsePagination(r *http.Request) (int32, int32) {
	limit := int32(20)
	offset := int32(0)

	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = int32(n)
		}
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = int32(n)
		}
	}

	return limit, offset
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
