// Package services implements the business logic layer of the application.
// Services orchestrate repositories, enforce business rules, translate
// repository errors into domain sentinels, and never reference HTTP concerns.
package services

import "errors"

var (
	// ErrUserNotFound is returned when a lookup by id or email matches no row,
	// or when an update targets a user that does not exist.
	ErrUserNotFound = errors.New("services: user not found")
	// ErrEmailAlreadyExists is returned when a create or update would violate
	// the email uniqueness constraint.
	ErrEmailAlreadyExists = errors.New("services: email already exists")
)
