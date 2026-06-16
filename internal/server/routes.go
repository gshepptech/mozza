package server

import (
	"io/fs"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// registerRoutes mounts all API v1 endpoints and the embedded UI on the router.
func (s *Server) registerRoutes() {
	// Metrics endpoint and middleware (opt-in).
	if s.cfg.MetricsEnabled {
		s.router.Use(metricsMiddleware)
		s.router.Handle("/metrics", promhttp.Handler())
	}

	s.router.Route("/api/v1", func(r chi.Router) {
		r.Use(jsonContentType)

		// Public endpoints (no auth required).
		r.Get("/health", s.handleHealth())
		r.Get("/readyz", s.handleReadyz())
		r.Get("/version", s.handleVersion())

		// Auth endpoints (public, rate-limited).
		r.Group(func(r chi.Router) {
			r.Use(rateLimit(10))
			r.Post("/auth/register", s.handleRegister())
			r.Post("/auth/login", s.handleLogin())
			r.Post("/auth/logout", s.handleLogout())
		})

		// Webhook endpoint (public, called by GitHub — signature-validated internally).
		r.Post("/webhooks/github", s.handleGitHubWebhook())

		// Protected endpoints (require session cookie).
		r.Group(func(r chi.Router) {
			r.Use(s.sessionAuth)

			r.Get("/auth/me", s.handleMe())
			r.Put("/auth/me", s.handleUpdateMe())

			// Read-only endpoints — all authenticated users can access.
			r.Get("/plan", s.handlePlan())
			r.Get("/plan/slices", s.handleSlices())
			r.Get("/plan/slices/{name}", s.handleSlice())
			r.Get("/plan/ingredients", s.handleIngredients())
			r.Get("/doctor", s.handleDoctor())
			r.Get("/status", s.handleStatus())
			r.Get("/deploys/{id}", s.handleDeployStatus())
			r.Get("/users", s.handleListUsers())
			r.Get("/users/{id}", s.handleGetUser())
			r.Get("/teams", s.handleListTeams())
			r.Get("/teams/{id}", s.handleGetTeam())
			r.Get("/teams/{id}/members", s.handleListTeamMembers())
			r.Get("/recipes", s.handleListRecipes())
			r.Get("/recipes/{id}", s.handleGetRecipe())
			r.Get("/aliases", s.handleListAliases())
			r.Get("/deployments", s.handleListDeployments())
			r.Get("/deployments/{id}", s.handleGetDeployment())
			r.Get("/deployments/{id}/logs", s.handleDeploymentLogs())
			r.Get("/clusters", s.handleListClusters())
			r.Get("/clusters/{id}", s.handleGetCluster())
			r.Get("/templates", s.handleListTemplates())
			r.Get("/templates/{id}", s.handleGetTemplate())
			r.Get("/marketplace/recipes", s.handleListMarketplaceRecipes())
			r.Get("/marketplace/recipes/{name}", s.handleGetMarketplaceRecipe())
			r.Get("/repos", s.handleListRepos())
			r.Get("/builds", s.handleListBuilds())
			r.Get("/builds/{id}", s.handleGetBuild())
			r.Get("/builds/{id}/logs", s.handleBuildLogs())

			// Preview deploy endpoints — list and manage branch previews.
			r.Get("/apps/{id}/previews", s.handleListPreviews())

			// Monitoring endpoints — app metrics and health.
			r.Get("/apps/{id}/metrics", s.handleAppMetrics())
			r.Get("/apps/{id}/health", s.handleAppHealth())
			r.Get("/apps/{id}/timeseries", s.handleTimeSeries())
			r.Get("/monitoring/summary", s.handleMonitoringSummary())

			// Registry scanning — discover images in a Docker Hub namespace.
			r.Get("/registry/scan", s.handleScanNamespace())

			// Proxy endpoints — routing table and certificate status.
			r.Get("/proxy/routes", s.handleProxyRoutes())
			r.Get("/proxy/certificates", s.handleProxyCertificates())

			// Cluster (real K8s data) — read-only, wrapped with recovery + timeout.
			r.Route("/cluster", func(r chi.Router) {
				r.Use(recoveryMiddleware)
				r.Use(timeoutMiddleware(10 * time.Second))
				r.Get("/status", s.handleClusterStatus())
				r.Get("/nodes", s.handleClusterNodes())
				r.Get("/pods", s.handleClusterPods())
				r.Get("/pods/logs", s.handleClusterPodLogs())
				r.Get("/deployments", s.handleClusterDeployments())
				r.Get("/namespaces", s.handleClusterNamespaces())
				r.Get("/services", s.handleClusterServices())
				r.Get("/events", s.handleClusterEvents())
				r.Get("/metrics", s.handleClusterMetrics())
				r.Post("/namespaces/{ns}/deployments/{name}/restart", s.handleRestartDeployment())
			})

			// Mutation endpoints — deployer or admin role required, rate-limited.
			r.Group(func(r chi.Router) {
				r.Use(requireRole("deployer", "admin"))
				r.Use(rateLimit(30))

				r.Post("/deploy", s.handleDeploy())
				r.Post("/recipes", s.handleCreateRecipe())
				r.Put("/recipes/{id}", s.handleUpdateRecipe())
				r.Delete("/recipes/{id}", s.handleDeleteRecipe())
				r.Post("/recipes/validate", s.handleValidateRecipe())
				r.Post("/aliases", s.handleCreateAlias())
				r.Delete("/aliases/{name}", s.handleDeleteAlias())
				r.Post("/deploy/preview", s.handleDeployPreview())
				r.Post("/deployments", s.handleTriggerDeploy())
				r.Post("/deployments/{id}/rollback", s.handleRollbackDeploy())
				r.Delete("/deployments/{id}", s.handleDeleteDeployment())
				r.Post("/clusters", s.handleCreateCluster())
				r.Delete("/clusters/{id}", s.handleDeleteCluster())
				r.Post("/templates/{id}/deploy", s.handleDeployTemplate())
				r.Post("/marketplace/recipes/{name}/deploy", s.handleDeployMarketplaceRecipe())
				r.Post("/import/scan", s.handleScanRepo())
				r.Post("/import/deploy", s.handleDeployImport())
				r.Post("/import/compose", s.handleComposeImport())
				r.Post("/import/compose/deploy", s.handleComposeImportDeploy())
				r.Post("/detect", s.handleDetect())
				r.Post("/detect/generate", s.handleDetectGenerate())
				r.Post("/repos/connect", s.handleConnectRepo())
				r.Delete("/repos/{id}", s.handleDisconnectRepo())
				r.Delete("/apps/{id}/previews/{branch}", s.handleDeletePreview())
			})

			// Admin endpoints — admin role required.
			r.Group(func(r chi.Router) {
				r.Use(requireRole("admin"))

				r.Put("/users/{id}/role", s.handleUpdateRole())
				r.Post("/teams", s.handleCreateTeam())
				r.Delete("/teams/{id}", s.handleDeleteTeam())
				r.Post("/teams/{id}/members", s.handleAddTeamMember())
				r.Delete("/teams/{id}/members/{userID}", s.handleRemoveTeamMember())
			})
		})
	})

	// Serve embedded UI files if available.
	if s.cfg.UI != nil {
		s.router.Handle("/*", s.handleUI())
	}
}

// handleUI returns a handler that serves the embedded UI files.
// For SPA routing, requests for paths that do not match a static file
// are served index.html so the client-side router can handle them.
func (s *Server) handleUI() http.Handler {
	fileServer := http.FileServer(http.FS(s.cfg.UI))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		urlPath := path.Clean(r.URL.Path)
		if urlPath == "." {
			urlPath = "/"
		}

		// Known API-like paths that should NOT fall back to index.html.
		if urlPath == "/metrics" {
			http.NotFound(w, r)
			return
		}

		// For non-root paths, check if the file exists in the embedded FS.
		// Missing files fall back to "/" so the file server serves index.html
		// (SPA routing). Root "/" is left alone — http.FileServer serves
		// index.html for directory requests automatically.
		if urlPath != "/" {
			_, err := fs.Stat(s.cfg.UI, strings.TrimPrefix(urlPath, "/"))
			if err != nil {
				urlPath = "/"
			}
		}

		r.URL.Path = urlPath
		fileServer.ServeHTTP(w, r)
	})
}
