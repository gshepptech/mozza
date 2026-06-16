package proxy

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net/http"
	"path/filepath"

	"github.com/caddyserver/certmagic"
)

// ACMEConfig holds configuration for automatic certificate management
// via ACME (Let's Encrypt).
type ACMEConfig struct {
	// Email is the contact email for the ACME account.
	Email string
	// Domains is the list of domains to obtain certificates for.
	Domains []string
	// DataDir is the base directory for certificate storage.
	DataDir string
	// Staging uses Let's Encrypt staging when true (for testing).
	Staging bool
}

// acmeProvider manages certmagic-based TLS certificates.
type acmeProvider struct {
	cfg    ACMEConfig
	magic  *certmagic.Config
	issuer *certmagic.ACMEIssuer
}

// newACMEProvider creates a new ACME certificate provider. It configures
// certmagic with file-based storage and an HTTP-01 challenge solver.
func newACMEProvider(cfg ACMEConfig) (*acmeProvider, error) {
	if cfg.Email == "" {
		return nil, fmt.Errorf("acme: email is required")
	}
	if len(cfg.Domains) == 0 {
		return nil, fmt.Errorf("acme: at least one domain is required")
	}

	storageDir := filepath.Join(cfg.DataDir, "certificates")

	// Configure certmagic storage.
	certmagic.Default.Storage = &certmagic.FileStorage{Path: storageDir}

	magic := certmagic.NewDefault()

	issuer := certmagic.NewACMEIssuer(magic, certmagic.ACMEIssuer{
		Email:  cfg.Email,
		Agreed: true,
	})

	if cfg.Staging {
		issuer.CA = certmagic.LetsEncryptStagingCA
	} else {
		issuer.CA = certmagic.LetsEncryptProductionCA
	}

	magic.Issuers = []certmagic.Issuer{issuer}

	return &acmeProvider{
		cfg:    cfg,
		magic:  magic,
		issuer: issuer,
	}, nil
}

// tlsConfig returns a tls.Config that uses certmagic's certificate
// management for automatic provisioning and renewal.
func (a *acmeProvider) tlsConfig() *tls.Config {
	return a.magic.TLSConfig()
}

// httpChallengeHandler returns an HTTP handler that solves ACME HTTP-01
// challenges. Non-challenge requests are passed to the fallback handler.
func (a *acmeProvider) httpChallengeHandler(fallback http.Handler) http.Handler {
	return a.issuer.HTTPChallengeHandler(fallback)
}

// manageAsync starts certificate management for the configured domains
// in the background. It obtains and auto-renews certificates.
func (a *acmeProvider) manageAsync(ctx context.Context) error {
	slog.Info("acme: starting certificate management",
		"domains", a.cfg.Domains,
		"email", a.cfg.Email,
		"staging", a.cfg.Staging,
	)
	if err := a.magic.ManageAsync(ctx, a.cfg.Domains); err != nil {
		return fmt.Errorf("acme: manage domains: %w", err)
	}
	return nil
}
