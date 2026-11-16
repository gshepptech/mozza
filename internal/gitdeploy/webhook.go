package gitdeploy

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
)

// maxWebhookBodySize is the maximum size of a webhook request body (5 MB).
const maxWebhookBodySize = 5 << 20

// PushEvent represents the relevant fields from a GitHub push webhook payload.
type PushEvent struct {
	Ref        string     `json:"ref"`
	After      string     `json:"after"`
	Repository Repository `json:"repository"`
	HeadCommit *Commit    `json:"head_commit"`
}

// Repository holds the repository information from a GitHub webhook.
type Repository struct {
	FullName string `json:"full_name"`
	CloneURL string `json:"clone_url"`
	HTMLURL  string `json:"html_url"`
}

// Commit holds commit information from a GitHub webhook.
type Commit struct {
	ID      string `json:"id"`
	Message string `json:"message"`
	Author  Author `json:"author"`
}

// Author holds author information from a GitHub commit.
type Author struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

// WebhookHandler processes incoming GitHub webhook requests.
type WebhookHandler struct {
	service *Service
}

// NewWebhookHandler creates a handler for GitHub webhooks.
func NewWebhookHandler(svc *Service) *WebhookHandler {
	return &WebhookHandler{service: svc}
}

// Handle processes a GitHub webhook request. It validates the signature,
// parses the push event, and enqueues a build.
func (h *WebhookHandler) Handle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeWebhookError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	eventType := r.Header.Get("X-GitHub-Event")
	if eventType == "" {
		writeWebhookError(w, http.StatusBadRequest, "missing X-GitHub-Event header")
		return
	}

	// Only process push events.
	if eventType == "ping" {
		writeWebhookJSON(w, http.StatusOK, map[string]string{"status": "pong"})
		return
	}
	if eventType != "push" {
		writeWebhookJSON(w, http.StatusOK, map[string]string{
			"status": "ignored",
			"reason": "only push events are processed",
		})
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, maxWebhookBodySize))
	if err != nil {
		writeWebhookError(w, http.StatusBadRequest, "failed to read request body")
		return
	}

	// Parse the push event to get the repo URL for secret lookup.
	var event PushEvent
	if err := json.Unmarshal(body, &event); err != nil {
		writeWebhookError(w, http.StatusBadRequest, "invalid JSON payload")
		return
	}

	// Find the connected repo to get the webhook secret.
	repoURL := normalizeRepoURL(event.Repository.HTMLURL)
	repos, err := h.service.ListRepos(r.Context())
	if err != nil {
		slog.Error("webhook: failed to list repos", "error", err)
		writeWebhookError(w, http.StatusInternalServerError, "internal error")
		return
	}

	var matchedRepo *ConnectedRepoMatch
	for _, repo := range repos {
		if normalizeRepoURL(repo.RepoURL) == repoURL {
			matchedRepo = &ConnectedRepoMatch{
				ID:        repo.ID,
				RepoURL:   repo.RepoURL,
				WebhookID: repo.WebhookID,
			}
			break
		}
	}

	if matchedRepo == nil {
		writeWebhookError(w, http.StatusNotFound, "repository not connected")
		return
	}

	// Validate the webhook signature.
	signature := r.Header.Get("X-Hub-Signature-256")
	if signature == "" {
		writeWebhookError(w, http.StatusUnauthorized, "missing signature")
		return
	}
	if !ValidateSignature(body, signature, matchedRepo.WebhookID) {
		writeWebhookError(w, http.StatusUnauthorized, "invalid signature")
		return
	}

	// Extract branch name from ref (refs/heads/main -> main).
	branch := extractBranch(event.Ref)
	if branch == "" {
		writeWebhookJSON(w, http.StatusOK, map[string]string{
			"status": "ignored",
			"reason": "not a branch push",
		})
		return
	}

	commitSHA := event.After
	if commitSHA == "" && event.HeadCommit != nil {
		commitSHA = event.HeadCommit.ID
	}
	if commitSHA == "" || commitSHA == "0000000000000000000000000000000000000000" {
		writeWebhookJSON(w, http.StatusOK, map[string]string{
			"status": "ignored",
			"reason": "branch deleted",
		})
		return
	}

	// Enqueue the build.
	build, err := h.service.EnqueueBuild(r.Context(), repoURL, commitSHA, branch)
	if err != nil {
		slog.Error("webhook: failed to enqueue build",
			"error", err,
			"repo", repoURL,
			"commit", commitSHA,
		)
		writeWebhookError(w, http.StatusInternalServerError, "failed to enqueue build")
		return
	}

	slog.Info("webhook: build enqueued",
		"build_id", build.ID,
		"repo", repoURL,
		"branch", branch,
		"commit", commitSHA[:minInt(7, len(commitSHA))],
	)

	writeWebhookJSON(w, http.StatusAccepted, map[string]any{
		"status":   "accepted",
		"build_id": build.ID,
	})
}

// ConnectedRepoMatch holds the fields needed for webhook processing.
type ConnectedRepoMatch struct {
	ID        int64
	RepoURL   string
	WebhookID string // used as the webhook secret for HMAC validation
}

// ValidateSignature verifies the HMAC-SHA256 signature of the webhook payload.
// The signature header format is "sha256=<hex-encoded-hash>".
func ValidateSignature(payload []byte, signatureHeader, secret string) bool {
	if secret == "" {
		return false
	}

	parts := strings.SplitN(signatureHeader, "=", 2)
	if len(parts) != 2 || parts[0] != "sha256" {
		return false
	}

	expectedMAC, err := hex.DecodeString(parts[1])
	if err != nil {
		return false
	}

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	actualMAC := mac.Sum(nil)

	return hmac.Equal(actualMAC, expectedMAC)
}

// extractBranch returns the branch name from a git ref.
// Returns empty string if the ref is not a branch (e.g., a tag).
func extractBranch(ref string) string {
	const prefix = "refs/heads/"
	if strings.HasPrefix(ref, prefix) {
		return ref[len(prefix):]
	}
	return ""
}

// writeWebhookJSON writes a JSON response for webhook endpoints.
func writeWebhookJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	data, err := json.Marshal(v)
	if err != nil {
		slog.Error("webhook: failed to marshal response", "error", err)
		return
	}
	_, _ = w.Write(data)
}

// writeWebhookError writes a JSON error response for webhook endpoints.
func writeWebhookError(w http.ResponseWriter, status int, message string) {
	writeWebhookJSON(w, status, map[string]string{"error": message})
}

// SignPayload creates an HMAC-SHA256 signature for a payload.
// Returns the signature in the format "sha256=<hex-encoded-hash>".
func SignPayload(payload []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	return fmt.Sprintf("sha256=%s", hex.EncodeToString(mac.Sum(nil)))
}
