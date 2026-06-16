package monitor

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

const (
	// healthCheckInterval is how often health endpoints are polled.
	healthCheckInterval = 30 * time.Second
	// healthCheckTimeout is the HTTP timeout for each health check.
	healthCheckTimeout = 5 * time.Second
	// healthyThreshold is the number of consecutive successes to become healthy.
	healthyThreshold = 3
	// downThreshold is the number of consecutive failures to become down.
	downThreshold = 3
)

// HealthStatus represents the health state of an app.
type HealthStatus string

const (
	// StatusHealthy means the app has passed at least healthyThreshold consecutive checks.
	StatusHealthy HealthStatus = "healthy"
	// StatusDegraded means the app has 1-2 consecutive failures.
	StatusDegraded HealthStatus = "degraded"
	// StatusDown means the app has at least downThreshold consecutive failures.
	StatusDown HealthStatus = "down"
	// StatusUnknown means no checks have run yet.
	StatusUnknown HealthStatus = "unknown"
)

// AppHealth holds the current health state for a single app.
type AppHealth struct {
	AppID           int64        `json:"app_id"`
	DeploymentID    string       `json:"deployment_id"`
	Status          HealthStatus `json:"status"`
	URL             string       `json:"url"`
	LastCheck       time.Time    `json:"last_check"`
	LastSuccess     time.Time    `json:"last_success,omitempty"`
	ConsecutiveOK   int          `json:"consecutive_ok"`
	ConsecutiveFail int          `json:"consecutive_fail"`
	LastError       string       `json:"last_error,omitempty"`
}

// healthTarget tracks a registered health endpoint.
type healthTarget struct {
	AppID        int64
	DeploymentID string
	URL          string
}

// HealthChecker polls health endpoints and tracks per-app health status.
type HealthChecker struct {
	client  *http.Client
	mu      sync.RWMutex
	targets []healthTarget
	status  map[int64]*AppHealth
	cancel  context.CancelFunc
	done    chan struct{}
}

// NewHealthChecker creates a new health checker.
func NewHealthChecker() *HealthChecker {
	return &HealthChecker{
		client: &http.Client{Timeout: healthCheckTimeout},
		status: make(map[int64]*AppHealth),
		done:   make(chan struct{}),
	}
}

// RegisterApp adds a health endpoint to poll.
func (h *HealthChecker) RegisterApp(appID int64, deploymentID, healthURL string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	for _, t := range h.targets {
		if t.AppID == appID {
			return
		}
	}

	h.targets = append(h.targets, healthTarget{
		AppID:        appID,
		DeploymentID: deploymentID,
		URL:          healthURL,
	})
	h.status[appID] = &AppHealth{
		AppID:        appID,
		DeploymentID: deploymentID,
		Status:       StatusUnknown,
		URL:          healthURL,
	}
}

// UnregisterApp removes an app from health checking.
func (h *HealthChecker) UnregisterApp(appID int64) {
	h.mu.Lock()
	defer h.mu.Unlock()

	for i, t := range h.targets {
		if t.AppID == appID {
			h.targets = append(h.targets[:i], h.targets[i+1:]...)
			break
		}
	}
	delete(h.status, appID)
}

// Status returns the current health status for an app. Returns nil if unknown.
func (h *HealthChecker) Status(appID int64) *AppHealth {
	h.mu.RLock()
	defer h.mu.RUnlock()
	s, ok := h.status[appID]
	if !ok {
		return nil
	}
	cp := *s
	return &cp
}

// AllStatus returns health status for all registered apps.
func (h *HealthChecker) AllStatus() []AppHealth {
	h.mu.RLock()
	defer h.mu.RUnlock()

	result := make([]AppHealth, 0, len(h.status))
	for _, s := range h.status {
		result = append(result, *s)
	}
	return result
}

// Start begins the background health check loop.
func (h *HealthChecker) Start() {
	ctx, cancel := context.WithCancel(context.Background())
	h.cancel = cancel

	go h.checkLoop(ctx)

	slog.Info("health checker started", "interval", healthCheckInterval)
}

// Stop shuts down the health checker.
func (h *HealthChecker) Stop() {
	if h.cancel != nil {
		h.cancel()
	}
	<-h.done
	slog.Info("health checker stopped")
}

func (h *HealthChecker) checkLoop(ctx context.Context) {
	defer close(h.done)

	ticker := time.NewTicker(healthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			h.checkAll(ctx)
		case <-ctx.Done():
			return
		}
	}
}

func (h *HealthChecker) checkAll(ctx context.Context) {
	h.mu.RLock()
	targets := make([]healthTarget, len(h.targets))
	copy(targets, h.targets)
	h.mu.RUnlock()

	for _, t := range targets {
		h.checkOne(ctx, t)
	}
}

func (h *HealthChecker) checkOne(ctx context.Context, t healthTarget) {
	reqCtx, cancel := context.WithTimeout(ctx, healthCheckTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, t.URL, nil)
	if err != nil {
		h.recordFailure(t.AppID, fmt.Sprintf("invalid URL: %v", err))
		return
	}

	resp, err := h.client.Do(req)
	if err != nil {
		h.recordFailure(t.AppID, err.Error())
		return
	}
	resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		h.recordSuccess(t.AppID)
	} else {
		h.recordFailure(t.AppID,
			fmt.Sprintf("HTTP %d", resp.StatusCode))
	}
}

func (h *HealthChecker) recordSuccess(appID int64) {
	h.mu.Lock()
	defer h.mu.Unlock()

	s := h.status[appID]
	if s == nil {
		return
	}

	now := time.Now()
	s.ConsecutiveOK++
	s.ConsecutiveFail = 0
	s.LastCheck = now
	s.LastSuccess = now
	s.LastError = ""

	if s.ConsecutiveOK >= healthyThreshold {
		s.Status = StatusHealthy
	}
}

func (h *HealthChecker) recordFailure(appID int64, errMsg string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	s := h.status[appID]
	if s == nil {
		return
	}

	s.ConsecutiveFail++
	s.ConsecutiveOK = 0
	s.LastCheck = time.Now()
	s.LastError = errMsg

	if s.ConsecutiveFail >= downThreshold {
		s.Status = StatusDown
	} else {
		s.Status = StatusDegraded
	}
}
