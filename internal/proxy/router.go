// Package proxy provides a reverse proxy with domain-based routing,
// automatic TLS certificate management, and backend health checking.
package proxy

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// defaultRateLimit is the default requests-per-second limit per domain.
const defaultRateLimit = 100

// Backend represents a proxied upstream service.
type Backend struct {
	// URL is the upstream address to proxy traffic to.
	URL *url.URL `json:"-"`
	// RawURL is the string form of the backend URL for serialisation.
	RawURL string `json:"url"`
	// HealthEndpoint is the path to probe for health checks (e.g. "/healthz").
	HealthEndpoint string `json:"health_endpoint"`
	// RateLimit is the maximum requests per second for this domain.
	// 0 means unlimited; negative values are treated as unlimited.
	RateLimit int `json:"rate_limit"`
	// Healthy indicates whether the backend is currently passing health checks.
	Healthy bool `json:"healthy"`
	// LastCheck records the most recent health probe time.
	LastCheck time.Time `json:"last_check"`
	// limiter enforces per-domain rate limiting. nil means unlimited.
	limiter *rate.Limiter
	// failures tracks consecutive health check failures (not exported).
	failures int
	// successes tracks consecutive health check successes (not exported).
	successes int
}

// Router maintains an in-memory map of domain → backend and supports
// persistence for restart recovery.
type Router struct {
	mu       sync.RWMutex
	backends map[string]*Backend
	dataDir  string
}

// routeEntry is the on-disk representation of a route for persistence.
type routeEntry struct {
	Domain         string `json:"domain"`
	URL            string `json:"url"`
	HealthEndpoint string `json:"health_endpoint"`
	RateLimit      int    `json:"rate_limit,omitempty"`
}

// NewRouter creates a Router that persists route state to dataDir.
func NewRouter(dataDir string) *Router {
	return &Router{
		backends: make(map[string]*Backend),
		dataDir:  dataDir,
	}
}

// newLimiter creates a rate.Limiter for the given requests-per-second limit.
// Returns nil when rps <= 0 (unlimited).
func newLimiter(rps int) *rate.Limiter {
	if rps <= 0 {
		return nil
	}
	return rate.NewLimiter(rate.Limit(rps), rps)
}

// AddRoute registers a backend for the given domain. If a route for the
// domain already exists it is replaced. The route table is persisted to
// disk after every mutation.
func (rt *Router) AddRoute(domain, backendURL string, healthEndpoint string) error {
	return rt.AddRouteWithLimit(domain, backendURL, healthEndpoint, defaultRateLimit)
}

// AddRouteWithLimit registers a backend with a specific rate limit.
// A rateLimit of 0 or negative means unlimited.
func (rt *Router) AddRouteWithLimit(domain, backendURL, healthEndpoint string, rateLimit int) error {
	u, err := url.Parse(backendURL)
	if err != nil {
		return fmt.Errorf("AddRoute: invalid backend URL: %w", err)
	}

	rt.mu.Lock()
	rt.backends[domain] = &Backend{
		URL:            u,
		RawURL:         backendURL,
		HealthEndpoint: healthEndpoint,
		RateLimit:      rateLimit,
		Healthy:        true, // assume healthy until first check
		limiter:        newLimiter(rateLimit),
	}
	rt.mu.Unlock()

	if err := rt.persist(); err != nil {
		slog.Error("failed to persist routes", "error", err)
	}

	slog.Info("route added", "domain", domain, "backend", backendURL)
	return nil
}

// RemoveRoute deletes the route for the given domain.
func (rt *Router) RemoveRoute(domain string) {
	rt.mu.Lock()
	delete(rt.backends, domain)
	rt.mu.Unlock()

	if err := rt.persist(); err != nil {
		slog.Error("failed to persist routes", "error", err)
	}

	slog.Info("route removed", "domain", domain)
}

// Route looks up the backend for a host. It strips any port suffix from
// the host header before matching. Returns nil when no route matches.
func (rt *Router) Route(host string) *Backend {
	// Strip port from host header (e.g. "example.com:443" → "example.com").
	domain := host
	for i := len(host) - 1; i >= 0; i-- {
		if host[i] == ':' {
			domain = host[:i]
			break
		}
	}

	rt.mu.RLock()
	defer rt.mu.RUnlock()
	return rt.backends[domain]
}

// Backends returns a snapshot of all registered backends keyed by domain.
func (rt *Router) Backends() map[string]*Backend {
	rt.mu.RLock()
	defer rt.mu.RUnlock()

	out := make(map[string]*Backend, len(rt.backends))
	for k, v := range rt.backends {
		cp := *v
		out[k] = &cp
	}
	return out
}

// persist writes the current route table to a JSON file in dataDir.
func (rt *Router) persist() error {
	if rt.dataDir == "" {
		return nil
	}

	if err := os.MkdirAll(rt.dataDir, 0o755); err != nil {
		return fmt.Errorf("persist: mkdir: %w", err)
	}

	rt.mu.RLock()
	entries := make([]routeEntry, 0, len(rt.backends))
	for domain, b := range rt.backends {
		entries = append(entries, routeEntry{
			Domain:         domain,
			URL:            b.RawURL,
			HealthEndpoint: b.HealthEndpoint,
			RateLimit:      b.RateLimit,
		})
	}
	rt.mu.RUnlock()

	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return fmt.Errorf("persist: marshal: %w", err)
	}

	p := filepath.Join(rt.dataDir, "routes.json")
	return os.WriteFile(p, data, 0o644)
}

// LoadRoutes reads persisted routes from disk and populates the in-memory map.
func (rt *Router) LoadRoutes() error {
	if rt.dataDir == "" {
		return nil
	}

	p := filepath.Join(rt.dataDir, "routes.json")

	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("LoadRoutes: %w", err)
	}

	var entries []routeEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return fmt.Errorf("LoadRoutes: unmarshal: %w", err)
	}

	rt.mu.Lock()
	defer rt.mu.Unlock()

	for _, e := range entries {
		u, err := url.Parse(e.URL)
		if err != nil {
			slog.Warn("skipping invalid persisted route", "domain", e.Domain, "url", e.URL, "error", err)
			continue
		}
		rl := e.RateLimit
		if rl == 0 {
			rl = defaultRateLimit
		}
		rt.backends[e.Domain] = &Backend{
			URL:            u,
			RawURL:         e.URL,
			HealthEndpoint: e.HealthEndpoint,
			RateLimit:      rl,
			Healthy:        true,
			limiter:        newLimiter(rl),
		}
	}

	slog.Info("loaded persisted routes", "count", len(entries))
	return nil
}
