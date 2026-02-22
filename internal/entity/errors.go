// Package entity defines main entities for business logic (services), data base mapping and
// HTTP response objects if suitable. Each logic group entities in own file.
package entity

import (
	"errors"
	"fmt"
	"net/http"
)

// AppError represents a structured application error with an HTTP status code.
type AppError struct {
	Code       string `json:"code"`
	Message    string `json:"message"`
	HTTPStatus int    `json:"-"`
}

// Error implements the error interface.
func (e *AppError) Error() string {
	return e.Message
}

// Sentinel application errors.
var (
	ErrNotFound        = &AppError{Code: "NOT_FOUND", Message: "resource not found", HTTPStatus: http.StatusNotFound}
	ErrValidation      = &AppError{Code: "VALIDATION_ERROR", Message: "invalid input", HTTPStatus: http.StatusBadRequest}
	ErrBadRequest      = &AppError{Code: "BAD_REQUEST", Message: "bad request", HTTPStatus: http.StatusBadRequest}
	ErrUnauthorized    = &AppError{Code: "UNAUTHORIZED", Message: "authentication required", HTTPStatus: http.StatusUnauthorized}
	ErrForbidden       = &AppError{Code: "FORBIDDEN", Message: "access denied", HTTPStatus: http.StatusForbidden}
	ErrInternal        = &AppError{Code: "INTERNAL_ERROR", Message: "internal server error", HTTPStatus: http.StatusInternalServerError}
	ErrExternalService = &AppError{Code: "EXTERNAL_SERVICE_ERROR", Message: "external service unavailable", HTTPStatus: http.StatusBadGateway}
)

// NewAppError creates a new AppError wrapping an underlying error.
func NewAppError(appErr *AppError, underlying error) error {
	return fmt.Errorf("%w: %w", appErr, underlying)
}

// GetAppError extracts AppError from an error chain. Returns ErrInternal if not found.
func GetAppError(err error) *AppError {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr
	}

	return ErrInternal
}
