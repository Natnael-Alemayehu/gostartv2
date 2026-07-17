// Package models defines the plain domain types shared across the handler,
// service, and repository layers. These types carry no HTTP or database
// awareness; they describe the core business entities and the input shapes
// used to create or update them.
package models

import (
	"time"

	"github.com/google/uuid"
)

// User is the core domain representation of an application user, mapping the
// persisted record to fields used across the service and handler layers.
type User struct {
	ID           uuid.UUID
	Email        string
	PasswordHash string
	Name         string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// UserCreate carries the fields required to create a new user. It is accepted
// by the service layer after the handler has validated input and hashed the
// password.
type UserCreate struct {
	Email        string
	PasswordHash string
	Name         string
}

// UserUpdate carries the optional fields for modifying an existing user. A
// nil pointer means the field should be left unchanged, letting callers
// perform partial updates.
type UserUpdate struct {
	Email        *string
	PasswordHash *string
	Name         *string
}

// PageCursor identifies the last row of a page so the caller can request the
// next page without using OFFSET. It is built from the (created_at, id) tuple
// of the last returned row.
type PageCursor struct {
	CreatedAt time.Time
	ID        uuid.UUID
}

// ListUsersInput is the repository-level request for a page of users. Cursor
// is nil for the first page; for subsequent pages it is the PageCursor of the
// last row from the previous page.
type ListUsersInput struct {
	Limit  int32
	Cursor *PageCursor
}
