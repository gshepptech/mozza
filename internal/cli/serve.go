package cli

import (
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/gshepptech/mozza/internal/auth"
	"github.com/gshepptech/mozza/internal/config"
	"github.com/gshepptech/mozza/internal/crypto"
	k8sdeployer "github.com/gshepptech/mozza/internal/deploy/k8s"
	localdeployer "github.com/gshepptech/mozza/internal/deploy/local"
	"github.com/gshepptech/mozza/internal/doctor"
	"github.com/gshepptech/mozza/internal/doctor/rules"
	"github.com/gshepptech/mozza/internal/marketplace"
	"github.com/gshepptech/mozza/internal/proxy"
	"github.com/gshepptech/mozza/internal/server"
	"github.com/gshepptech/mozza/internal/store"
	"github.com/gshepptech/mozza/internal/template"
	"github.com/gshepptech/mozza/internal/ui"
)

// defaultServeHost is the default host when --host is not set.
const defaultServeHost = "localhost"

// newServeCmd creates the "mozza serve" command.
func newServeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the Mozza HTTP dashboard",
		Long:  "Launch the Mozza web dashboard for monitoring and managing deployments.",
		RunE:  runServe,
	}

	cmd.Flags().Int("port", config.DefaultServerPort, "HTTP server port")
	cmd.Flags().String("host", defaultServeHost, "HTTP server host")
	cmd.Flags().String("db", "", "SQLite database path (default: mozza.db)")
	cmd.Flags().String("db-url", "", "Database URL (postgres://... or file path for SQLite)")
	cmd.Flags().Bool("no-auth", false, "Disable authentication for local development (use with caution)")
	cmd.Flags().String("tls-cert", "", "Path to TLS certificate file (env: MOZZA_TLS_CERT)")
	cmd.Flags().String("tls-key", "", "Path to TLS private key file (env: MOZZA_TLS_KEY)")
	cmd.Flags().Bool("metrics", false, "Enable Prometheus /metrics endpoint (env: MOZZA_METRICS)")
	cmd.Flags().StringSlice("domain", nil, "Domain(s) for the reverse proxy TLS certificates")
	cmd.Flags().Bool("self-signed", false, "Use self-signed certificates for local development")

	return cmd
}

// runServe loads the application plan, prepares the embedded UI, and starts
// the HTTP dashboard server.
func runServe(cmd *cobra.Command, _ []string) error {
	host, _ := cmd.Flags().GetString("host")
	port, _ := cmd.Flags().GetInt("port")
	noAuth, _ := cmd.Flags().GetBool("no-auth")
	dbPath, _ := cmd.Flags().GetString("db")
	dbURL, _ := cmd.Flags().GetString("db-url")
	tlsCert, _ := cmd.Flags().GetString("tls-cert")
	tlsKey, _ := cmd.Flags().GetString("tls-key")
	metricsEnabled, _ := cmd.Flags().GetBool("metrics")
	domains, _ := cmd.Flags().GetStringSlice("domain")
	selfSigned, _ := cmd.Flags().GetBool("self-signed")

	// Fall back to environment variables.
	if !metricsEnabled && os.Getenv("MOZZA_METRICS") == "true" {
		metricsEnabled = true
	}

	// Fall back to environment variables for TLS paths.
	if tlsCert == "" {
		tlsCert = os.Getenv("MOZZA_TLS_CERT")
	}
	if tlsKey == "" {
		tlsKey = os.Getenv("MOZZA_TLS_KEY")
	}

	if !isLoopback(host) && !noAuth {
		return fmt.Errorf("runServe: binding to %q exposes the API without authentication; use --no-auth to override", host)
	}

	if !isLoopback(host) && noAuth {
		slog.Warn("binding to non-localhost address without authentication",
			"host", host,
			"hint", "the API exposes plan data to anyone who can reach this address",
		)
	}

	recipePath := recipeFlagValue(cmd)

	p, err := loadPlan(recipePath)
	if err != nil {
		return err
	}

	// Get the embedded UI filesystem, stripping the "dist" prefix.
	uiFS, err := fs.Sub(ui.DistFS, "dist")
	if err != nil {
		return fmt.Errorf("runServe: %w", err)
	}

	collector := doctor.NewCollector(slog.Default())
	doctorEngine := doctor.New(collector, rules.Default()...)

	// Resolve database DSN.
	dsn := resolveDSN(dbURL, dbPath)

	// Open store (auto-detects SQLite vs PostgreSQL from the DSN).
	st, err := store.Open(dsn)
	if err != nil {
		return fmt.Errorf("runServe: %w", err)
	}
	defer st.Close()

	// Set encryption key if provided.
	if encKey := os.Getenv("MOZZA_ENCRYPTION_KEY"); encKey != "" {
		key, err := crypto.KeyFromBase64(encKey)
		if err != nil {
			return fmt.Errorf("runServe: invalid MOZZA_ENCRYPTION_KEY: %w", err)
		}
		st.SetEncryptionKey(key)
	}

	// Create auth service.
	authSvc := auth.New(st)

	projectDir := filepath.Dir(recipePath)

	// Create local Docker Compose deployer (always available).
	localDeploy := localdeployer.New(st, projectDir)

	// Create K8s deployer if kubeconfig is available.
	k8sDeploy := k8sdeployer.New(st)

	// Load template catalog (non-fatal if it fails).
	tmplCatalog, err := template.Load()
	if err != nil {
		slog.Warn("failed to load template catalog", "err", err)
	}

	// Create marketplace service on top of the template catalog.
	var mktSvc *marketplace.Service
	if tmplCatalog != nil {
		mktSvc = marketplace.New(tmplCatalog, st)
	}

	cfg := server.Config{
		Host:           host,
		Port:           port,
		Plan:           p,
		UI:             uiFS,
		Doctor:         doctorEngine,
		ProjectDir:     projectDir,
		Store:          st,
		Auth:           authSvc,
		Deployer:       k8sDeploy,
		LocalDeployer:  localDeploy,
		Templates:      tmplCatalog,
		Marketplace:    mktSvc,
		TLSCert:        tlsCert,
		TLSKey:         tlsKey,
		MetricsEnabled: metricsEnabled,
		NoAuth:         noAuth,
	}

	cmd.Printf("Starting Mozza dashboard on %s:%d...\n", host, port)

	srv := server.New(cfg)

	// Start reverse proxy alongside the API server if domains are configured.
	if len(domains) > 0 || selfSigned {
		proxyCfg := proxy.Config{
			SelfSigned: selfSigned,
			Domains:    domains,
			DataDir:    filepath.Join(projectDir, ".mozza", "proxy"),
		}
		proxySrv := proxy.New(proxyCfg)
		srv.SetProxy(proxySrv)

		go func() {
			if err := proxySrv.ListenAndServe(cmd.Context()); err != nil {
				slog.Error("proxy server error", "error", err)
			}
		}()

		cmd.Printf("Reverse proxy started (self-signed=%v, domains=%v)\n", selfSigned, domains)
	}

	return srv.ListenAndServe(cmd.Context())
}

// isLoopback returns true if host is a loopback address.
func isLoopback(host string) bool {
	return host == "localhost" || host == "127.0.0.1" || host == "::1" || host == ""
}

// resolveDSN determines the database DSN from flags and environment.
// Priority: --db-url flag > MOZZA_DATABASE_URL env > --db flag > config file > default.
func resolveDSN(dbURL, dbPath string) string {
	// 1. Explicit --db-url flag.
	if dbURL != "" {
		return dbURL
	}

	// 2. MOZZA_DATABASE_URL environment variable.
	if envURL := os.Getenv("MOZZA_DATABASE_URL"); envURL != "" {
		return envURL
	}

	// 3. Explicit --db flag (SQLite path).
	if dbPath != "" {
		return dbPath
	}

	// 4. Config file database.path.
	cfg, _ := config.Load()
	if cfg != nil && cfg.Database.Path != "" {
		return cfg.Database.Path
	}

	// 5. Default SQLite path.
	return "mozza.db"
}
