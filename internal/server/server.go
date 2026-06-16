// Package server provides the Mozza HTTP dashboard API server.
// It exposes plan, slice, and ingredient data over a JSON REST API
// using the Chi router.
package server

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"k8s.io/client-go/kubernetes"

	"github.com/gshepptech/mozza/internal/auth"
	"github.com/gshepptech/mozza/internal/cluster"
	"github.com/gshepptech/mozza/internal/deploy"
	"github.com/gshepptech/mozza/internal/doctor"
	"github.com/gshepptech/mozza/internal/gitdeploy"
	"github.com/gshepptech/mozza/internal/marketplace"
	"github.com/gshepptech/mozza/internal/monitor"
	"github.com/gshepptech/mozza/internal/plan"
	"github.com/gshepptech/mozza/internal/proxy"
	"github.com/gshepptech/mozza/internal/store"
	"github.com/gshepptech/mozza/internal/template"
)

// shutdownTimeout is the maximum duration the server waits for in-flight
// requests to complete during graceful shutdown.
const (
	shutdownTimeout   = 5 * time.Second
	readTimeout       = 10 * time.Second
	readHeaderTimeout = 5 * time.Second
	writeTimeout      = 30 * time.Second
	idleTimeout       = 120 * time.Second
	maxHeaderBytes    = 1 << 20 // 1 MB
	maxSubprocesses   = 3       // max concurrent subprocess-spawning requests
)

// Config holds server configuration.
type Config struct {
	// Host is the address to bind the server to.
	Host string
	// Port is the port number for the server.
	Port int
	// Plan is the loaded application plan to serve.
	Plan *plan.AppPlan
	// UI is the filesystem containing the embedded web dashboard assets.
	// When nil, no UI routes are registered.
	UI fs.FS
	// Doctor is the diagnostic engine. When nil, the doctor endpoint returns 503.
	Doctor *doctor.Engine
	// ProjectDir is the working directory for Docker Compose operations.
	// Used by the status endpoint to query container state.
	ProjectDir string
	// Store is the SQLite persistence layer. Required for auth and CRUD endpoints.
	Store *store.Store
	// Auth is the authentication service. When nil, auth endpoints return 503.
	Auth *auth.Service
	// Deployer is the Kubernetes deployment engine. When nil, K8s deployments
	// fail with "no deployer configured".
	Deployer deploy.Deployer
	// LocalDeployer is the Docker Compose deployment engine. When nil, local
	// deployments fail with "no local deployer configured".
	LocalDeployer deploy.Deployer
	// Templates is the template catalog. When nil, template endpoints return 503.
	Templates *template.Catalog
	// Marketplace is the recipe marketplace service. When nil, marketplace endpoints return 503.
	Marketplace *marketplace.Service
	// GitDeploy is the git push-to-deploy service. When nil, git deploy endpoints return 503.
	GitDeploy *gitdeploy.Service
	// GitDeployWebhook is the webhook handler for GitHub events.
	GitDeployWebhook *gitdeploy.WebhookHandler
	// TLSCert is the path to the TLS certificate file. Both TLSCert and TLSKey
	// must be set to enable TLS; if only one is provided the server returns an error.
	TLSCert string
	// TLSKey is the path to the TLS private key file.
	TLSKey string
	// MetricsEnabled enables the /metrics Prometheus endpoint and request metrics middleware.
	MetricsEnabled bool
	// NoAuth disables authentication for local development. When true, all
	// requests are treated as an admin user.
	NoAuth bool
	// Proxy is the optional reverse proxy server. When set, proxy status
	// endpoints are available in the API.
	Proxy *proxy.Server
}

// Server is the Mozza HTTP dashboard server.
type Server struct {
	router chi.Router
	cfg    Config
	// subSem limits the number of concurrent requests that spawn subprocesses
	// (docker, lsof) to prevent resource exhaustion.
	subSem chan struct{}
	// k8sClient is the lazily-initialized Kubernetes clientset for cluster API.
	k8sClient kubernetes.Interface
	// healthMon probes cluster reachability on a background goroutine.
	healthMon *cluster.HealthMonitor
	// clusterCache holds pre-processed K8s resource data refreshed by informers.
	clusterCache *cluster.ClusterCache
	// informerMgr watches K8s resources and keeps clusterCache current.
	informerMgr *cluster.InformerManager
	// snapshotMgr persists cluster cache to SQLite for fast restarts.
	snapshotMgr *cluster.SnapshotManager
	// monitorCollector collects Docker container metrics in the background.
	monitorCollector *monitor.Collector
	// monitorHealth polls app health endpoints in the background.
	monitorHealth *monitor.HealthChecker
	// tsStore holds in-memory time-series metrics for fast chart queries.
	tsStore *monitor.TimeSeriesStore
}

// New creates a Server with the given configuration.
// It initialises the router, applies middleware, and registers routes.
func New(cfg Config) *Server {
	s := &Server{
		router: chi.NewRouter(),
		cfg:    cfg,
		subSem: make(chan struct{}, maxSubprocesses),
	}
	s.healthMon = cluster.NewHealthMonitor(s.kubeClient, 10*time.Second)
	s.clusterCache = cluster.NewClusterCache()
	s.informerMgr = cluster.NewInformerManager(s.kubeClient, s.clusterCache)
	if cfg.Store != nil {
		s.snapshotMgr = cluster.NewSnapshotManager(s.clusterCache, cfg.Store)
		s.snapshotMgr.LoadSnapshot()
		s.monitorCollector = monitor.NewCollector(cfg.Store)
		s.tsStore = monitor.NewTimeSeriesStore(0)
		s.monitorCollector.TSStore = s.tsStore
	}
	s.monitorHealth = monitor.NewHealthChecker()
	if cfg.NoAuth && cfg.Store != nil {
		s.ensureNoAuthUser()
	}
	s.applyMiddleware()
	s.registerRoutes()
	return s
}

// ListenAndServe starts the HTTP server and blocks until the provided
// context is cancelled. It performs graceful shutdown with a 5-second timeout.
// When both TLSCert and TLSKey are configured, the server listens over HTTPS.
func (s *Server) ListenAndServe(ctx context.Context) error {
	tlsEnabled, err := s.validateTLS()
	if err != nil {
		return err
	}

	addr := net.JoinHostPort(s.cfg.Host, strconv.Itoa(s.cfg.Port))

	srv := &http.Server{
		Addr:              addr,
		Handler:           s.router,
		ReadTimeout:       readTimeout,
		ReadHeaderTimeout: readHeaderTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
		MaxHeaderBytes:    maxHeaderBytes,
	}

	s.healthMon.Start()
	if s.snapshotMgr != nil {
		s.snapshotMgr.Start()
	}

	// Start background session cleanup (every hour).
	if s.cfg.Store != nil {
		go func() {
			ticker := time.NewTicker(1 * time.Hour)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					if err := s.cfg.Store.CleanExpiredSessions(); err != nil {
						slog.Error("session cleanup failed", "error", err)
					}
				case <-ctx.Done():
					return
				}
			}
		}()
	}

	if s.monitorCollector != nil {
		s.monitorCollector.Start()
	}
	s.monitorHealth.Start()

	// Start the informer manager in the background so it doesn't block
	// server startup when no Kubernetes cluster is reachable.
	go func() {
		if err := s.informerMgr.Start(); err != nil {
			slog.Warn("informer manager started, cache sync timed out (will continue in background)", "error", err)
		}
	}()

	errCh := make(chan error, 1)

	go func() {
		slog.Info("server starting", "addr", addr, "tls", tlsEnabled)
		var listenErr error
		if tlsEnabled {
			listenErr = srv.ListenAndServeTLS(s.cfg.TLSCert, s.cfg.TLSKey)
		} else {
			listenErr = srv.ListenAndServe()
		}
		if listenErr != nil && !errors.Is(listenErr, http.ErrServerClosed) {
			errCh <- listenErr
		}
		close(errCh)
	}()

	select {
	case err := <-errCh:
		return fmt.Errorf("ListenAndServe: %w", err)
	case <-ctx.Done():
		return s.shutdown(srv)
	}
}

// validateTLS checks the TLS configuration. It returns true when TLS is
// enabled (both cert and key are set), false when neither is set, and an
// error when only one of the two is provided.
func (s *Server) validateTLS() (bool, error) {
	hasCert := s.cfg.TLSCert != ""
	hasKey := s.cfg.TLSKey != ""

	switch {
	case hasCert && hasKey:
		return true, nil
	case !hasCert && !hasKey:
		return false, nil
	default:
		return false, fmt.Errorf("ListenAndServe: both --tls-cert and --tls-key are required")
	}
}

// Handler returns the underlying http.Handler for testing.
func (s *Server) Handler() http.Handler {
	return s.router
}

// shutdown gracefully stops the HTTP server within the shutdown timeout.
func (s *Server) shutdown(srv *http.Server) error {
	slog.Info("server shutting down")
	s.monitorHealth.Stop()
	if s.monitorCollector != nil {
		s.monitorCollector.Stop()
	}
	if s.snapshotMgr != nil {
		s.snapshotMgr.Stop()
	}
	s.informerMgr.Stop()
	s.healthMon.Stop()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("ListenAndServe: shutdown: %w", err)
	}
	return nil
}
