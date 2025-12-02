package models

import (
	"errors"
	"fmt"
)

// Common error types
var (
	ErrNotFound      = errors.New("resource not found")
	ErrAlreadyExists = errors.New("resource already exists")
	ErrConflict      = errors.New("operation conflicts with current state")
)

// AppError represents an application-level error with context
type AppError struct {
	Code    string
	Message string
	Err     error
}

func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}
	return e.Message
}

func (e *AppError) Unwrap() error {
	return e.Err
}

// ErrInvalidInput creates a validation error
func ErrInvalidInput(message string) error {
	return &AppError{
		Code:    "INVALID_INPUT",
		Message: message,
	}
}

// ErrNotFoundWithMsg creates a not found error with custom message
func ErrNotFoundWithMsg(message string) error {
	return &AppError{
		Code:    "NOT_FOUND",
		Message: message,
		Err:     ErrNotFound,
	}
}

// ErrConflictWithMsg creates a conflict error with custom message
func ErrConflictWithMsg(message string) error {
	return &AppError{
		Code:    "CONFLICT",
		Message: message,
		Err:     ErrConflict,
	}
}
