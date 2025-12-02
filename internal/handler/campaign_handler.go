package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/Raymond9734/smsleopard-backend-challenge/internal/models"
	"github.com/Raymond9734/smsleopard-backend-challenge/internal/service"
)

// CampaignHandler handles campaign HTTP requests
type CampaignHandler struct {
	campaignService service.CampaignService
	logger          *slog.Logger
}

// NewCampaignHandler creates a new campaign handler
func NewCampaignHandler(campaignService service.CampaignService, logger *slog.Logger) *CampaignHandler {
	return &CampaignHandler{
		campaignService: campaignService,
		logger:          logger,
	}
}

// CreateCampaign handles POST /campaigns
func (h *CampaignHandler) CreateCampaign(w http.ResponseWriter, r *http.Request) {
	var req service.CreateCampaignRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_JSON", "Invalid JSON format")
		return
	}

	campaign, err := h.campaignService.Create(r.Context(), &req)
	if err != nil {
		handleError(w, err, h.logger)
		return
	}

	respondCreated(w, campaign)
}

// ListCampaigns handles GET /campaigns
func (h *CampaignHandler) ListCampaigns(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters
	query := r.URL.Query()

	page, _ := strconv.Atoi(query.Get("page"))
	pageSize, _ := strconv.Atoi(query.Get("page_size"))

	filter := models.CampaignFilter{
		Channel:  query.Get("channel"),
		Status:   query.Get("status"),
		Page:     page,
		PageSize: pageSize,
	}

	result, err := h.campaignService.List(r.Context(), filter)
	if err != nil {
		handleError(w, err, h.logger)
		return
	}

	respondSuccess(w, result)
}

// GetCampaign handles GET /campaigns/{id}
func (h *CampaignHandler) GetCampaign(w http.ResponseWriter, r *http.Request) {
	id, err := extractIDFromPath(r.URL.Path, "/campaigns/")
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_ID", "Invalid campaign ID")
		return
	}

	campaign, err := h.campaignService.GetByID(r.Context(), id)
	if err != nil {
		handleError(w, err, h.logger)
		return
	}

	respondSuccess(w, campaign)
}

// SendCampaign handles POST /campaigns/{id}/send
func (h *CampaignHandler) SendCampaign(w http.ResponseWriter, r *http.Request) {
	id, err := extractIDFromPath(r.URL.Path, "/campaigns/")
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_ID", "Invalid campaign ID")
		return
	}

	var req service.SendCampaignRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_JSON", "Invalid JSON format")
		return
	}

	result, err := h.campaignService.SendCampaign(r.Context(), id, &req)
	if err != nil {
		handleError(w, err, h.logger)
		return
	}

	respondSuccess(w, result)
}

// PreviewPersonalized handles POST /campaigns/{id}/personalized-preview
func (h *CampaignHandler) PreviewPersonalized(w http.ResponseWriter, r *http.Request) {
	id, err := extractIDFromPath(r.URL.Path, "/campaigns/")
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_ID", "Invalid campaign ID")
		return
	}

	var req service.PreviewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_JSON", "Invalid JSON format")
		return
	}

	result, err := h.campaignService.PreviewPersonalized(r.Context(), id, &req)
	if err != nil {
		handleError(w, err, h.logger)
		return
	}

	respondSuccess(w, result)
}

// extractIDFromPath extracts numeric ID from URL path
func extractIDFromPath(path, prefix string) (int64, error) {
	// Remove prefix to get ID part
	idPart := path[len(prefix):]

	// Find the end of the ID (before any slash)
	for i, c := range idPart {
		if c == '/' {
			idPart = idPart[:i]
			break
		}
	}

	return strconv.ParseInt(idPart, 10, 64)
}
