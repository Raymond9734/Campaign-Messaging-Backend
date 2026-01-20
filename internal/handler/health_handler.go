package handler

import (
	"context"
	"database/sql"
	"log/slog"
	"net/http"
	"time"

	"github.com/Raymond9734/campaign-messaging-backend/internal/queue"
)

// HealthHandler handles health check requests
type HealthHandler struct {
	db          *sql.DB
	queueClient queue.Client
	logger      *slog.Logger
}

// NewHealthHandler creates a new health handler
func NewHealthHandler(db *sql.DB, queueClient queue.Client, logger *slog.Logger) *HealthHandler {
	return &HealthHandler{
		db:          db,
		queueClient: queueClient,
		logger:      logger,
	}
}

// HealthResponse represents the health check response
type HealthResponse struct {
	Status   string            `json:"status"`
	Services map[string]string `json:"services"`
}

// Health handles GET /health
func (h *HealthHandler) Health(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	response := HealthResponse{
		Status:   "healthy",
		Services: make(map[string]string),
	}

	// Check database
	if err := h.db.PingContext(ctx); err != nil {
		h.logger.Error("database health check failed", slog.String("error", err.Error()))
		response.Status = "unhealthy"
		response.Services["database"] = "unhealthy"
	} else {
		response.Services["database"] = "healthy"
	}

	// Check queue
	if h.queueClient != nil {
		if err := h.queueClient.Health(ctx); err != nil {
			h.logger.Error("queue health check failed", slog.String("error", err.Error()))
			response.Status = "unhealthy"
			response.Services["queue"] = "unhealthy"
		} else {
			response.Services["queue"] = "healthy"
		}
	} else {
		response.Services["queue"] = "not_configured"
	}

	// Return appropriate status code
	if response.Status == "healthy" {
		respondSuccess(w, response)
	} else {
		respondJSON(w, http.StatusServiceUnavailable, response)
	}
}
