// Package gitdeploy provides push-to-deploy functionality for Git repositories.
// It handles GitHub webhook reception, repo connection management (PAT-based),
// and automated build/deploy pipelines triggered by git push events.
package gitdeploy

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"strings"

	"github.com/gshepptech/mozza/internal/store"
)

// Service manages git repository connections and automated builds.
type Service struct {
	store   *store.Store
	builder *Builder
	queue   *Queue
}

// NewService creates a new git deploy service.
func NewService(s *store.Store) *Service {
	svc := &Service{
		store: s,
	}
	svc.builder = NewBuilder(s)
	svc.queue = NewQueue(svc.builder)
	return svc
}

// Start begins processing the build queue. Call Stop to shut down.
func (s *Service) Start() {
	s.queue.Start()
	slog.Info("gitdeploy service started")
}

// Stop gracefully shuts down the build queue.
func (s *Service) Stop() {
	s.queue.Stop()
	slog.Info("gitdeploy service stopped")
}

// ConnectRequest holds parameters for connecting a repository.
type ConnectRequest struct {
	RepoURL string
	Token   string
}

// ConnectResponse holds the result of connecting a repository.
type ConnectResponse struct {
	Repo          *store.ConnectedRepo `json:"repo"`
	WebhookSecret string               `json:"webhook_secret"`
}

// Connect registers a GitHub repository for push-to-deploy.
// It stores the repo connection and returns a webhook secret that must be
// configured in the GitHub webhook settings.
func (s *Service) Connect(ctx context.Context, req ConnectRequest) (*ConnectResponse, error) {
	if req.RepoURL == "" {
		return nil, fmt.Errorf("Connect: repo URL is required")
	}
	if req.Token == "" {
		return nil, fmt.Errorf("Connect: token is required")
	}

	// Normalize the repo URL.
	repoURL := normalizeRepoURL(req.RepoURL)

	// Generate a webhook secret for HMAC validation.
	secret, err := generateWebhookSecret()
	if err != nil {
		return nil, fmt.Errorf("Connect: generate secret: %w", err)
	}

	repo, err := s.store.CreateConnectedRepo(ctx, repoURL, secret, []byte(req.Token), 0)
	if err != nil {
		return nil, fmt.Errorf("Connect: %w", err)
	}

	slog.Info("repository connected",
		"repo_url", repoURL,
		"repo_id", repo.ID,
	)

	return &ConnectResponse{
		Repo:          repo,
		WebhookSecret: secret,
	}, nil
}

// Disconnect removes a connected repository by ID.
func (s *Service) Disconnect(ctx context.Context, id int64) error {
	if err := s.store.DeleteConnectedRepo(ctx, id); err != nil {
		return fmt.Errorf("Disconnect: %w", err)
	}

	slog.Info("repository disconnected", "repo_id", id)
	return nil
}

// ListRepos returns all connected repositories.
func (s *Service) ListRepos(ctx context.Context) ([]store.ConnectedRepo, error) {
	repos, err := s.store.ListConnectedRepos(ctx)
	if err != nil {
		return nil, fmt.Errorf("ListRepos: %w", err)
	}
	return repos, nil
}

// GetRepo returns a connected repository by ID.
func (s *Service) GetRepo(ctx context.Context, id int64) (*store.ConnectedRepo, error) {
	repo, err := s.store.GetConnectedRepo(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("GetRepo: %w", err)
	}
	return repo, nil
}

// ListBuilds returns recent builds.
func (s *Service) ListBuilds(ctx context.Context, limit int) ([]store.Build, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	builds, err := s.store.ListBuilds(ctx, limit)
	if err != nil {
		return nil, fmt.Errorf("ListBuilds: %w", err)
	}
	return builds, nil
}

// GetBuild returns a build by ID.
func (s *Service) GetBuild(ctx context.Context, id int64) (*store.Build, error) {
	build, err := s.store.GetBuild(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("GetBuild: %w", err)
	}
	return build, nil
}

// EnqueueBuild adds a build to the processing queue.
func (s *Service) EnqueueBuild(ctx context.Context, repoURL, commitSHA, branch string) (*store.Build, error) {
	build, err := s.store.CreateBuild(ctx, repoURL, commitSHA, branch)
	if err != nil {
		return nil, fmt.Errorf("EnqueueBuild: %w", err)
	}

	s.queue.Enqueue(BuildJob{
		BuildID:   build.ID,
		RepoURL:   repoURL,
		CommitSHA: commitSHA,
		Branch:    branch,
	})

	slog.Info("build enqueued",
		"build_id", build.ID,
		"repo_url", repoURL,
		"commit", commitSHA[:minInt(7, len(commitSHA))],
		"branch", branch,
	)

	return build, nil
}

// normalizeRepoURL strips trailing slashes and .git suffix.
func normalizeRepoURL(url string) string {
	url = strings.TrimRight(url, "/")
	url = strings.TrimSuffix(url, ".git")
	url = strings.TrimRight(url, "/")
	return url
}

// generateWebhookSecret creates a cryptographically random hex string.
func generateWebhookSecret() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generateWebhookSecret: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// minInt returns the smaller of two integers.
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
