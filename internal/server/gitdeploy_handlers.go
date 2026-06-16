package server

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"nhooyr.io/websocket"
	"nhooyr.io/websocket/wsjson"

	"github.com/gshepptech/mozza/internal/gitdeploy"
	"github.com/gshepptech/mozza/internal/store"
)

type connectRepoRequest struct {
	RepoURL string `json:"repo_url"`
	Token   string `json:"token"`
}

type connectRepoResponse struct {
	ID            int64  `json:"id"`
	RepoURL       string `json:"repo_url"`
	WebhookSecret string `json:"webhook_secret"`
	WebhookURL    string `json:"webhook_url"`
}

type repoResponse struct {
	ID        int64  `json:"id"`
	RepoURL   string `json:"repo_url"`
	WebhookID string `json:"webhook_id,omitempty"`
	CreatedAt string `json:"created_at"`
}

type repoListResponse struct {
	Repos []repoResponse `json:"repos"`
}

type buildResponse struct {
	ID         int64  `json:"id"`
	RepoURL    string `json:"repo_url"`
	CommitSHA  string `json:"commit_sha,omitempty"`
	Branch     string `json:"branch,omitempty"`
	Status     string `json:"status"`
	Logs       string `json:"logs,omitempty"`
	DurationMs int64  `json:"duration_ms,omitempty"`
	ImageTag   string `json:"image_tag,omitempty"`
	CreatedAt  string `json:"created_at"`
}

type buildListResponse struct {
	Builds []buildResponse `json:"builds"`
}

// handleConnectRepo connects a GitHub repository for push-to-deploy.
// POST /api/v1/repos/connect
func (s *Server) handleConnectRepo() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.cfg.GitDeploy == nil {
			Error(w, http.StatusServiceUnavailable, "git deploy not configured")
			return
		}

		var req connectRepoRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			Error(w, http.StatusBadRequest, "invalid request body")
			return
		}

		if req.RepoURL == "" {
			Error(w, http.StatusBadRequest, "repo_url is required")
			return
		}
		if req.Token == "" {
			Error(w, http.StatusBadRequest, "token is required")
			return
		}

		result, err := s.cfg.GitDeploy.Connect(r.Context(), connectRepoReq(req))
		if err != nil {
			Error(w, http.StatusInternalServerError, "failed to connect repository")
			return
		}

		JSON(w, http.StatusCreated, connectRepoResponse{
			ID:            result.Repo.ID,
			RepoURL:       result.Repo.RepoURL,
			WebhookSecret: result.WebhookSecret,
			WebhookURL:    "/api/v1/webhooks/github",
		})
	}
}

// handleDisconnectRepo removes a connected repository.
// DELETE /api/v1/repos/{id}
func (s *Server) handleDisconnectRepo() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.cfg.GitDeploy == nil {
			Error(w, http.StatusServiceUnavailable, "git deploy not configured")
			return
		}

		id, err := parseIDParam(r)
		if err != nil {
			Error(w, http.StatusBadRequest, "invalid repo ID")
			return
		}

		if err := s.cfg.GitDeploy.Disconnect(r.Context(), id); err != nil {
			if errors.Is(err, store.ErrNotFound) {
				Error(w, http.StatusNotFound, "repository not found")
				return
			}
			Error(w, http.StatusInternalServerError, "failed to disconnect repository")
			return
		}

		JSON(w, http.StatusOK, map[string]string{"status": "disconnected"})
	}
}

// handleListRepos returns all connected repositories.
// GET /api/v1/repos
func (s *Server) handleListRepos() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.cfg.GitDeploy == nil {
			Error(w, http.StatusServiceUnavailable, "git deploy not configured")
			return
		}

		repos, err := s.cfg.GitDeploy.ListRepos(r.Context())
		if err != nil {
			Error(w, http.StatusInternalServerError, "failed to list repositories")
			return
		}

		resp := repoListResponse{
			Repos: make([]repoResponse, len(repos)),
		}
		for i, repo := range repos {
			resp.Repos[i] = toRepoResponse(repo)
		}

		JSON(w, http.StatusOK, resp)
	}
}

// handleGitHubWebhook receives GitHub webhook events.
// POST /api/v1/webhooks/github
func (s *Server) handleGitHubWebhook() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.cfg.GitDeploy == nil {
			Error(w, http.StatusServiceUnavailable, "git deploy not configured")
			return
		}

		s.cfg.GitDeployWebhook.Handle(w, r)
	}
}

// handleListBuilds returns recent builds.
// GET /api/v1/builds
func (s *Server) handleListBuilds() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.cfg.GitDeploy == nil {
			Error(w, http.StatusServiceUnavailable, "git deploy not configured")
			return
		}

		limit := 50
		if l := r.URL.Query().Get("limit"); l != "" {
			if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 100 {
				limit = parsed
			}
		}

		builds, err := s.cfg.GitDeploy.ListBuilds(r.Context(), limit)
		if err != nil {
			Error(w, http.StatusInternalServerError, "failed to list builds")
			return
		}

		resp := buildListResponse{
			Builds: make([]buildResponse, len(builds)),
		}
		for i, b := range builds {
			resp.Builds[i] = toBuildResponse(b)
		}

		JSON(w, http.StatusOK, resp)
	}
}

// handleGetBuild returns a single build by ID.
// GET /api/v1/builds/{id}
func (s *Server) handleGetBuild() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.cfg.GitDeploy == nil {
			Error(w, http.StatusServiceUnavailable, "git deploy not configured")
			return
		}

		id, err := parseIDParam(r)
		if err != nil {
			Error(w, http.StatusBadRequest, "invalid build ID")
			return
		}

		build, err := s.cfg.GitDeploy.GetBuild(r.Context(), id)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				Error(w, http.StatusNotFound, "build not found")
				return
			}
			Error(w, http.StatusInternalServerError, "failed to get build")
			return
		}

		JSON(w, http.StatusOK, toBuildResponse(*build))
	}
}

// buildLogMessage is the JSON message sent over the WebSocket.
type buildLogMessage struct {
	Type      string `json:"type"`
	Content   string `json:"content"`
	Step      int    `json:"step"`
	Timestamp int64  `json:"timestamp"`
}

// buildLogPollInterval is how often we poll the build status while streaming.
const buildLogPollInterval = 2 * time.Second

// handleBuildLogs streams build logs over a WebSocket connection.
// If the build is already complete, all stored logs are sent at once.
// If the build is in progress, logs are polled and streamed as they grow.
// GET /api/v1/builds/{id}/logs (WebSocket upgrade)
func (s *Server) handleBuildLogs() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.cfg.GitDeploy == nil {
			Error(w, http.StatusServiceUnavailable, "git deploy not configured")
			return
		}

		id, err := parseIDParam(r)
		if err != nil {
			Error(w, http.StatusBadRequest, "invalid build ID")
			return
		}

		conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
			InsecureSkipVerify: true,
		})
		if err != nil {
			slog.Error("build logs: websocket accept failed", "error", err)
			return
		}
		defer conn.Close(websocket.StatusNormalClosure, "done")

		ctx := r.Context()

		s.streamBuildLogs(ctx, conn, id)
	}
}

// streamBuildLogs sends build log data over the WebSocket connection.
func (s *Server) streamBuildLogs(ctx context.Context, conn *websocket.Conn, buildID int64) {
	build, err := s.cfg.GitDeploy.GetBuild(ctx, buildID)
	if err != nil {
		_ = sendBuildLogMsg(ctx, conn, "error", "build not found", 0)
		return
	}

	// If build is already done, send all logs at once and close.
	if build.Status == "success" || build.Status == "failed" {
		s.sendCompletedBuildLogs(ctx, conn, build)
		return
	}

	// Build is in progress — poll and stream.
	s.pollBuildLogs(ctx, conn, buildID)
}

// sendCompletedBuildLogs sends all logs for a finished build and a
// completion message, then returns.
func (s *Server) sendCompletedBuildLogs(ctx context.Context, conn *websocket.Conn, build *store.Build) {
	if build.Logs != "" {
		lines := strings.Split(build.Logs, "\n")
		step := 0
		for _, line := range lines {
			if line == "" {
				continue
			}
			if strings.HasPrefix(line, "==> ") {
				_ = sendBuildLogMsg(ctx, conn, "step", line, step)
				step++
			} else {
				_ = sendBuildLogMsg(ctx, conn, "log", line, step)
			}
		}
	}
	_ = sendBuildLogMsg(ctx, conn, "complete", build.Status, 0)
}

// pollBuildLogs polls the build record and streams new log lines as
// they appear. Stops when the build completes or the context is cancelled.
func (s *Server) pollBuildLogs(ctx context.Context, conn *websocket.Conn, buildID int64) {
	var lastLogLen int
	ticker := time.NewTicker(buildLogPollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			build, err := s.cfg.GitDeploy.GetBuild(ctx, buildID)
			if err != nil {
				_ = sendBuildLogMsg(ctx, conn, "error", "failed to fetch build", 0)
				return
			}

			// Send any new log content since last poll.
			if len(build.Logs) > lastLogLen {
				newContent := build.Logs[lastLogLen:]
				lastLogLen = len(build.Logs)
				lines := strings.Split(newContent, "\n")
				step := strings.Count(build.Logs[:lastLogLen-len(newContent)], "==> ")
				for _, line := range lines {
					if line == "" {
						continue
					}
					if strings.HasPrefix(line, "==> ") {
						_ = sendBuildLogMsg(ctx, conn, "step", line, step)
						step++
					} else {
						_ = sendBuildLogMsg(ctx, conn, "log", line, step)
					}
				}
			}

			if build.Status == "success" || build.Status == "failed" {
				_ = sendBuildLogMsg(ctx, conn, "complete", build.Status, 0)
				return
			}
		}
	}
}

// sendBuildLogMsg writes a single build log message to the WebSocket.
func sendBuildLogMsg(ctx context.Context, conn *websocket.Conn, msgType, content string, step int) error {
	msg := buildLogMessage{
		Type:      msgType,
		Content:   content,
		Step:      step,
		Timestamp: time.Now().UnixMilli(),
	}
	return wsjson.Write(ctx, conn, msg)
}

// parseIDParam extracts the {id} URL parameter as an int64.
func parseIDParam(r *http.Request) (int64, error) {
	idStr := chi.URLParam(r, "id")
	return strconv.ParseInt(idStr, 10, 64)
}

// toRepoResponse converts a store.ConnectedRepo to a repoResponse.
func toRepoResponse(repo store.ConnectedRepo) repoResponse {
	return repoResponse{
		ID:        repo.ID,
		RepoURL:   repo.RepoURL,
		WebhookID: repo.WebhookID,
		CreatedAt: repo.CreatedAt.Format("2006-01-02T15:04:05Z"),
	}
}

// connectRepoReq converts a connectRepoRequest to a gitdeploy.ConnectRequest.
func connectRepoReq(req connectRepoRequest) gitdeploy.ConnectRequest {
	return gitdeploy.ConnectRequest{
		RepoURL: req.RepoURL,
		Token:   req.Token,
	}
}

// toBuildResponse converts a store.Build to a buildResponse.
func toBuildResponse(b store.Build) buildResponse {
	return buildResponse{
		ID:         b.ID,
		RepoURL:    b.RepoURL,
		CommitSHA:  b.CommitSHA,
		Branch:     b.Branch,
		Status:     b.Status,
		Logs:       b.Logs,
		DurationMs: b.DurationMs,
		ImageTag:   b.ImageTag,
		CreatedAt:  b.CreatedAt.Format("2006-01-02T15:04:05Z"),
	}
}
