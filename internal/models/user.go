package models

import (
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID           uuid.UUID
	Email        string
	PasswordHash string
	Name         string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type UserCreate struct {
	Email        string
	PasswordHash string
	Name         string
}

type UserUpdate struct {
	Email        *string
	PasswordHash *string
	Name         *string
}
