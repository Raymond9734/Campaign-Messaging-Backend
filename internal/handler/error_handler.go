package handler

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/Raymond9734/smsleopard-backend-challenge/internal/models"
)

// handleError maps service errors to HTTP responses
func handleError(w http.ResponseWriter, err error, logger *slog.Logger) {
	// Check for custom AppError
	var appErr *models.AppError
	if errors.As(err, &appErr) {
		status := mapErrorCodeToHTTPStatus(appErr.Code)
		respondError(w, status, appErr.Code, appErr.Message)
		return
	}

	// Check for common errors
	switch {
	case errors.Is(err, models.ErrNotFound):
		respondError(w, http.StatusNotFound, "NOT_FOUND", err.Error())

	case errors.Is(err, models.ErrConflict):
		respondError(w, http.StatusConflict, "CONFLICT", err.Error())

	default:
		// Log internal errors but don't expose details to client
		logger.Error("internal server error",
			slog.String("error", err.Error()),
		)
		respondError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "An unexpected error occurred")
	}
}

// mapErrorCodeToHTTPStatus maps error codes to HTTP status codes
func mapErrorCodeToHTTPStatus(code string) int {
	switch code {
	case "INVALID_INPUT":
		return http.StatusBadRequest
	case "NOT_FOUND":
		return http.StatusNotFound
	case "CONFLICT":
		return http.StatusConflict
	case "UNAUTHORIZED":
		return http.StatusUnauthorized
	case "FORBIDDEN":
		return http.StatusForbidden
	default:
		return http.StatusInternalServerError
	}
}
