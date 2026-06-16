package gitdeploy

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/gshepptech/mozza/internal/proxy"
	"github.com/gshepptech/mozza/internal/store"
)

// Preview deployment limits and intervals.
const (
	maxPreviewsPerApp    = 10
	maxBranchLabelLen    = 63
	stalePreviewDuration = 7 * 24 * time.Hour
	cleanupInterval      = 1 * time.Hour
)

// branchClean matches characters that are not alphanumeric or hyphens.
var branchClean = regexp.MustCompile(`[^a-z0-9-]`)

// PreviewManager handles branch preview deploy lifecycle.
type PreviewManager struct {
	store  *store.Store
	router *proxy.Router

	stopOnce sync.Once
	done     chan struct{}
}

// NewPreviewManager creates a preview manager.
func NewPreviewManager(s *store.Store, router *proxy.Router) *PreviewManager {
	return &PreviewManager{
		store:  s,
		router: router,
		done:   make(chan struct{}),
	}
}

// Start begins the background stale-preview cleanup goroutine.
func (pm *PreviewManager) Start() {
	go pm.cleanupLoop()
	slog.Info("preview manager started",
		"cleanup_interval", cleanupInterval,
		"stale_after", stalePreviewDuration,
	)
}

// Stop shuts down the background cleanup goroutine.
func (pm *PreviewManager) Stop() {
	pm.stopOnce.Do(func() {
		close(pm.done)
	})
}

// SanitizeBranch converts a git branch name into a DNS-safe label.
// It lowercases, replaces "/" with "-", strips special characters,
// trims leading/trailing hyphens, and truncates to 63 characters.
func SanitizeBranch(branch string) string {
	s := strings.ToLower(branch)
	s = strings.ReplaceAll(s, "/", "-")
	s = branchClean.ReplaceAllString(s, "")
	s = strings.Trim(s, "-")

	if len(s) > maxBranchLabelLen {
		s = s[:maxBranchLabelLen]
		s = strings.TrimRight(s, "-")
	}

	if s == "" {
		s = "preview"
	}
	return s
}

// PreviewDomain builds the preview domain from a sanitized branch and app domain.
func PreviewDomain(branch, appDomain string) string {
	return SanitizeBranch(branch) + "." + appDomain
}

// Create creates or updates a preview deploy for the given app and branch.
// It enforces the per-app preview limit and registers a proxy route.
func (pm *PreviewManager) Create(ctx context.Context, appID int64, branch, appDomain, backendURL string) (*store.PreviewDeploy, error) {
	sanitized := SanitizeBranch(branch)
	domain := sanitized + "." + appDomain

	// Check for an existing preview on this branch.
	existing, err := pm.store.FindPreviewDeployByAppAndBranch(ctx, appID, sanitized)
	if err == nil {
		// Update activity and re-register route.
		if updateErr := pm.store.UpdatePreviewDeployActivity(ctx, existing.ID); updateErr != nil {
			slog.Warn("preview: failed to update activity", "id", existing.ID, "error", updateErr)
		}
		if routeErr := pm.registerRoute(domain, backendURL); routeErr != nil {
			return nil, fmt.Errorf("Create preview: register route: %w", routeErr)
		}
		slog.Info("preview updated", "app_id", appID, "branch", sanitized, "domain", domain)
		// Re-fetch for updated timestamp.
		updated, fetchErr := pm.store.GetPreviewDeploy(ctx, existing.ID)
		if fetchErr != nil {
			return existing, nil
		}
		return updated, nil
	}
	if !errors.Is(err, store.ErrNotFound) {
		return nil, fmt.Errorf("Create preview: lookup: %w", err)
	}

	// Enforce per-app limit.
	count, err := pm.store.CountPreviewDeploysByApp(ctx, appID)
	if err != nil {
		return nil, fmt.Errorf("Create preview: count: %w", err)
	}
	if count >= maxPreviewsPerApp {
		return nil, fmt.Errorf("Create preview: app %d has reached the maximum of %d previews", appID, maxPreviewsPerApp)
	}

	preview, err := pm.store.CreatePreviewDeploy(ctx, appID, sanitized, domain, 0)
	if err != nil {
		return nil, fmt.Errorf("Create preview: %w", err)
	}

	if routeErr := pm.registerRoute(domain, backendURL); routeErr != nil {
		return nil, fmt.Errorf("Create preview: register route: %w", routeErr)
	}

	slog.Info("preview created",
		"id", preview.ID,
		"app_id", appID,
		"branch", sanitized,
		"domain", domain,
	)
	return preview, nil
}

// List returns all preview deploys for an app.
func (pm *PreviewManager) List(ctx context.Context, appID int64) ([]store.PreviewDeploy, error) {
	deploys, err := pm.store.ListPreviewDeploysByApp(ctx, appID)
	if err != nil {
		return nil, fmt.Errorf("List previews: %w", err)
	}
	return deploys, nil
}

// Delete removes a preview deploy by app ID and branch, and unregisters the proxy route.
func (pm *PreviewManager) Delete(ctx context.Context, appID int64, branch, appDomain string) error {
	sanitized := SanitizeBranch(branch)
	domain := sanitized + "." + appDomain

	if err := pm.store.DeletePreviewDeployByAppAndBranch(ctx, appID, sanitized); err != nil {
		return fmt.Errorf("Delete preview: %w", err)
	}

	if pm.router != nil {
		pm.router.RemoveRoute(domain)
	}

	slog.Info("preview deleted", "app_id", appID, "branch", sanitized, "domain", domain)
	return nil
}

// Clean removes all stale preview deploys (no activity for 7 days)
// and unregisters their proxy routes.
func (pm *PreviewManager) Clean(ctx context.Context) (int64, error) {
	cutoff := time.Now().Add(-stalePreviewDuration)

	// List stale previews first so we can remove their routes.
	all, err := pm.store.ListPreviewDeploys(ctx)
	if err != nil {
		return 0, fmt.Errorf("Clean previews: list: %w", err)
	}

	for _, p := range all {
		if p.LastActivity != nil && p.LastActivity.Before(cutoff) && pm.router != nil && p.Domain != "" {
			pm.router.RemoveRoute(p.Domain)
		}
	}

	n, err := pm.store.CleanStalePreviewDeploys(ctx, cutoff)
	if err != nil {
		return 0, fmt.Errorf("Clean previews: %w", err)
	}

	if n > 0 {
		slog.Info("stale previews cleaned", "count", n)
	}
	return n, nil
}

// CleanStale is an alias for Clean, for symmetry with the store method.
func (pm *PreviewManager) CleanStale(ctx context.Context) (int64, error) {
	return pm.Clean(ctx)
}

// registerRoute adds a route to the proxy router if configured.
func (pm *PreviewManager) registerRoute(domain, backendURL string) error {
	if pm.router == nil {
		return nil
	}
	return pm.router.AddRoute(domain, backendURL, "/healthz")
}

// cleanupLoop periodically removes stale previews.
func (pm *PreviewManager) cleanupLoop() {
	ticker := time.NewTicker(cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			if _, err := pm.Clean(ctx); err != nil {
				slog.Error("preview cleanup failed", "error", err)
			}
			cancel()
		case <-pm.done:
			return
		}
	}
}
