package proxy

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

const (
	// healthCheckInterval is the time between consecutive health probes.
	healthCheckInterval = 30 * time.Second
	// healthCheckTimeout is how long each probe waits for a response.
	healthCheckTimeout = 5 * time.Second
	// failureThreshold is consecutive failures before marking unhealthy.
	failureThreshold = 3
	// successThreshold is consecutive successes before restoring healthy.
	successThreshold = 3
)

// HealthChecker periodically probes backend health endpoints and updates
// the Healthy flag on each Backend in the Router.
type HealthChecker struct {
	router *Router
	client *http.Client
	cancel context.CancelFunc
}

// NewHealthChecker creates a HealthChecker for the given router.
func NewHealthChecker(router *Router) *HealthChecker {
	return &HealthChecker{
		router: router,
		client: &http.Client{Timeout: healthCheckTimeout},
	}
}

// Start begins the background health check loop. It must be called at
// most once; call Stop to terminate.
func (hc *HealthChecker) Start() {
	ctx, cancel := context.WithCancel(context.Background())
	hc.cancel = cancel

	go hc.loop(ctx)

	slog.Info("health checker started", "interval", healthCheckInterval)
}

// Stop terminates the background health check loop.
func (hc *HealthChecker) Stop() {
	if hc.cancel != nil {
		hc.cancel()
	}
}

// loop runs health checks on all backends every healthCheckInterval.
func (hc *HealthChecker) loop(ctx context.Context) {
	ticker := time.NewTicker(healthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			hc.checkAll()
		}
	}
}

// checkAll probes every registered backend once.
func (hc *HealthChecker) checkAll() {
	backends := hc.router.Backends()

	for domain, backend := range backends {
		healthy := hc.probe(backend)
		hc.updateStatus(domain, healthy)
	}
}

// probe performs an HTTP GET against the backend health endpoint.
func (hc *HealthChecker) probe(b *Backend) bool {
	endpoint := b.HealthEndpoint
	if endpoint == "" {
		endpoint = "/healthz"
	}

	target := fmt.Sprintf("%s%s", b.URL.String(), endpoint)

	resp, err := hc.client.Get(target)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode >= 200 && resp.StatusCode < 400
}

// updateStatus applies the 3-strike logic to a backend's health state.
func (hc *HealthChecker) updateStatus(domain string, healthy bool) {
	hc.router.mu.Lock()
	defer hc.router.mu.Unlock()

	b, ok := hc.router.backends[domain]
	if !ok {
		return
	}

	b.LastCheck = time.Now()

	if healthy {
		b.failures = 0
		b.successes++

		if !b.Healthy && b.successes >= successThreshold {
			b.Healthy = true
			slog.Info("backend restored to healthy", "domain", domain)
		}
	} else {
		b.successes = 0
		b.failures++

		if b.Healthy && b.failures >= failureThreshold {
			b.Healthy = false
			slog.Warn("backend marked unhealthy",
				"domain", domain,
				"failures", b.failures,
			)
		}
	}
}
