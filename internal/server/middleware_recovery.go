package server

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"
)

// clusterErrorResponse is the structured error format for cluster API errors.
type clusterErrorResponse struct {
	Error   string `json:"error"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

// ClusterError writes a structured cluster error response.
func ClusterError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	data, err := json.Marshal(clusterErrorResponse{
		Error:   http.StatusText(status),
		Code:    code,
		Message: message,
	})
	if err != nil {
		// Last resort: write raw JSON.
		_, _ = w.Write([]byte(`{"error":"internal server error","code":"INTERNAL_ERROR"}`))
		return
	}
	_, _ = w.Write(data)
}

// recoveryMiddleware catches panics in handlers and returns a 500 JSON
// error response instead of dropping the TCP connection.
func recoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				stack := string(debug.Stack())
				slog.Error("handler panic recovered",
					"path", r.URL.Path,
					"panic", fmt.Sprintf("%v", rec),
					"stack", stack,
				)
				ClusterError(w, http.StatusInternalServerError,
					"INTERNAL_ERROR",
					"An internal error occurred. This has been logged.",
				)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// timeoutMiddleware enforces a maximum duration for handler execution.
// Uses http.TimeoutHandler which is safe against concurrent writes.
func timeoutMiddleware(timeout time.Duration) func(http.Handler) http.Handler {
	body := `{"error":"Gateway Timeout","code":"CLUSTER_TIMEOUT","message":"Request timed out. The cluster may be slow or unreachable."}`
	return func(next http.Handler) http.Handler {
		return http.TimeoutHandler(next, timeout, body)
	}
}
