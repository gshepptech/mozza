package server

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

// errorResponse is the JSON structure returned for error responses.
type errorResponse struct {
	Error string `json:"error"`
}

// JSON writes a JSON response with the given status code.
// If marshaling fails, it logs the error and sends a 500 response.
func JSON(w http.ResponseWriter, status int, v any) {
	data, err := json.Marshal(v)
	if err != nil {
		slog.Error("failed to marshal response", "error", err)
		http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
		return
	}

	w.WriteHeader(status)
	_, _ = w.Write(data)
}

// Error writes a JSON error response with the given status code and message.
func Error(w http.ResponseWriter, status int, message string) {
	JSON(w, status, errorResponse{Error: message})
}
