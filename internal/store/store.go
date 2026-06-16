package store

// Store manages database connections and provides access to domain-specific repositories.
// It supports both SQLite (local dev) and PostgreSQL (production) backends.

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"log/slog"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	_ "modernc.org/sqlite"
)

// mustParseTime parses an RFC3339 timestamp from the database.
// Parse failure indicates data corruption and is logged as an error.
func mustParseTime(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		slog.Error("corrupt timestamp in database", "value", s, "error", err)
	}
	return t
}

//go:embed migrations/*.sql
var migrationsFS embed.FS

//go:embed migrations_pg/*.sql
var migrationsPgFS embed.FS

// DriverSQLite is the driver name for SQLite.
const DriverSQLite = "sqlite"

// DriverPostgres is the driver name for PostgreSQL.
const DriverPostgres = "pgx"

// Store wraps the database connection.
type Store struct {
	db            *sql.DB
	driver        string
	encryptionKey []byte // AES-256-GCM key for encrypting cluster kubeconfigs (optional)
}

// SetEncryptionKey sets the AES-256 key used for encrypting sensitive data.
func (s *Store) SetEncryptionKey(key []byte) {
	s.encryptionKey = key
}

// Driver returns the database driver name ("sqlite" or "pgx").
func (s *Store) Driver() string {
	return s.driver
}

// Open creates a new Store connected to the database at dsn.
// If dsn starts with "postgres://" or "postgresql://", it connects to
// PostgreSQL. Otherwise it treats dsn as a SQLite file path.
// It runs migrations after connecting.
func Open(dsn string) (*Store, error) {
	var db *sql.DB
	var driver string
	var err error

	if isPostgresDSN(dsn) {
		driver = DriverPostgres
		db, err = sql.Open("pgx", dsn)
	} else {
		driver = DriverSQLite
		db, err = sql.Open("sqlite", dsn)
	}
	if err != nil {
		return nil, fmt.Errorf("Open: %w", err)
	}

	// SQLite-specific pragmas.
	if driver == DriverSQLite {
		if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
			db.Close()
			return nil, fmt.Errorf("Open: enable WAL: %w", err)
		}
		if _, err := db.Exec("PRAGMA foreign_keys=ON"); err != nil {
			db.Close()
			return nil, fmt.Errorf("Open: enable foreign keys: %w", err)
		}
	}

	s := &Store{db: db, driver: driver}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("Open: %w", err)
	}

	slog.Info("store opened", "dsn", redactDSN(dsn), "driver", driver)
	return s, nil
}

// Ping verifies the database connection is alive.
func (s *Store) Ping(ctx context.Context) error {
	return s.db.PingContext(ctx)
}

// Close closes the underlying database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

// DB returns the underlying *sql.DB for use in transactions.
func (s *Store) DB() *sql.DB {
	return s.db
}

// migrationFiles lists all SQLite migration files in order.
var migrationFiles = []string{
	"migrations/001_initial.sql",
	"migrations/002_deploy_pipeline.sql",
	"migrations/003_order_number.sql",
	"migrations/004_image_aliases.sql",
	"migrations/005_clusters.sql",
	"migrations/006_deployment_cluster.sql",
	"migrations/007_deployment_access_url.sql",
	"migrations/008_deployment_version_rollback.sql",
	"migrations/009_cluster_snapshots.sql",
	"migrations/010_rbac_roles.sql",
	"migrations/011_connected_repos.sql",
	"migrations/012_builds.sql",
	"migrations/013_preview_deploys.sql",
	"migrations/014_metrics.sql",
	"migrations/015_marketplace_cache.sql",
	"migrations/016_certificates.sql",
}

// migrationFilesPg lists all PostgreSQL migration files in order.
var migrationFilesPg = []string{
	"migrations_pg/001_initial.sql",
	"migrations_pg/002_deploy_pipeline.sql",
	"migrations_pg/003_order_number.sql",
	"migrations_pg/004_image_aliases.sql",
	"migrations_pg/005_clusters.sql",
	"migrations_pg/006_deployment_cluster.sql",
	"migrations_pg/007_deployment_access_url.sql",
	"migrations_pg/008_deployment_version_rollback.sql",
	"migrations_pg/009_cluster_snapshots.sql",
	"migrations_pg/010_rbac_roles.sql",
	"migrations_pg/011_connected_repos.sql",
	"migrations_pg/012_builds.sql",
	"migrations_pg/013_preview_deploys.sql",
	"migrations_pg/014_metrics.sql",
	"migrations_pg/015_marketplace_cache.sql",
	"migrations_pg/016_certificates.sql",
}

// migrate runs all embedded SQL migration files in order.
func (s *Store) migrate() error {
	files := migrationFiles
	fs := migrationsFS
	if s.driver == DriverPostgres {
		files = migrationFilesPg
		fs = migrationsPgFS
	}

	for _, name := range files {
		data, err := fs.ReadFile(name)
		if err != nil {
			return fmt.Errorf("migrate: read %s: %w", name, err)
		}

		if _, err := s.db.Exec(string(data)); err != nil {
			msg := err.Error()
			// Idempotent migrations: ignore "already exists" errors.
			if strings.Contains(msg, "duplicate column") ||
				strings.Contains(msg, "already exists") {
				continue
			}
			return fmt.Errorf("migrate: exec %s: %w", name, err)
		}
	}

	slog.Info("migrations applied", "driver", s.driver)
	return nil
}

// isPostgresDSN returns true if the DSN is a PostgreSQL connection string.
func isPostgresDSN(dsn string) bool {
	return strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://")
}

// redactDSN removes credentials from a DSN for logging.
func redactDSN(dsn string) string {
	if !isPostgresDSN(dsn) {
		return dsn
	}
	// Redact the password portion: postgres://user:pass@host -> postgres://user:***@host
	atIdx := strings.LastIndex(dsn, "@")
	if atIdx == -1 {
		return dsn
	}
	prefix := dsn[:strings.Index(dsn, "://")+3]
	rest := dsn[len(prefix):atIdx]
	colonIdx := strings.Index(rest, ":")
	if colonIdx == -1 {
		return dsn
	}
	return prefix + rest[:colonIdx] + ":***" + dsn[atIdx:]
}
