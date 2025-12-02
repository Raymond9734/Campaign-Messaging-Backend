package handler

import (
	"encoding/json"
	"net/http"
)

// ErrorResponse represents a standard error response
type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
}

// ErrorDetail contains error code and message
type ErrorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// respondJSON writes a JSON response with the given status code
func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if data != nil {
		if err := json.NewEncoder(w).Encode(data); err != nil {
			// If encoding fails, we can't do much at this point
			http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		}
	}
}

// respondError writes a standard error response
func respondError(w http.ResponseWriter, status int, code, message string) {
	response := ErrorResponse{
		Error: ErrorDetail{
			Code:    code,
			Message: message,
		},
	}
	respondJSON(w, status, response)
}

// respondSuccess writes a successful response with 200 OK
func respondSuccess(w http.ResponseWriter, data interface{}) {
	respondJSON(w, http.StatusOK, data)
}

// respondCreated writes a successful response with 201 Created
func respondCreated(w http.ResponseWriter, data interface{}) {
	respondJSON(w, http.StatusCreated, data)
}
