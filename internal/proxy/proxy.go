package proxy

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"net"
	"net/http"
	"net/http/httputil"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	httpPort  = ":80"
	httpsPort = ":443"

	proxyReadTimeout  = 10 * time.Second
	proxyWriteTimeout = 30 * time.Second
	proxyIdleTimeout  = 120 * time.Second

	selfSignedDuration = 365 * 24 * time.Hour
)

// Config holds configuration for the reverse proxy server.
type Config struct {
	// HTTPAddr overrides the HTTP listen address (default ":80").
	HTTPAddr string
	// HTTPSAddr overrides the HTTPS listen address (default ":443").
	HTTPSAddr string
	// SelfSigned enables self-signed TLS certificates for local development.
	// When false and ACMEEmail is set, certmagic handles TLS automatically.
	SelfSigned bool
	// Domains is the initial set of domains to configure for TLS.
	Domains []string
	// DataDir is the directory for persisted route and cert state.
	DataDir string
	// ACMEEmail is the contact email for Let's Encrypt certificate registration.
	// Required when SelfSigned is false for automatic TLS via certmagic.
	ACMEEmail string
	// ACMEStaging uses the Let's Encrypt staging environment (for testing).
	ACMEStaging bool
}

// Server is the reverse proxy that routes traffic by domain to backends.
type Server struct {
	cfg     Config
	router  *Router
	health  *HealthChecker
	httpSrv *http.Server
	tlsSrv  *http.Server
	acme    *acmeProvider
}

// New creates a proxy Server with the given configuration.
func New(cfg Config) *Server {
	if cfg.HTTPAddr == "" {
		cfg.HTTPAddr = httpPort
	}
	if cfg.HTTPSAddr == "" {
		cfg.HTTPSAddr = httpsPort
	}

	rt := NewRouter(cfg.DataDir)
	hc := NewHealthChecker(rt)

	return &Server{
		cfg:    cfg,
		router: rt,
		health: hc,
	}
}

// Router returns the underlying Router for external route management.
func (s *Server) Router() *Router {
	return s.router
}

// ListenAndServe starts the proxy on HTTP and HTTPS ports and blocks
// until the context is cancelled, then shuts down gracefully.
func (s *Server) ListenAndServe(ctx context.Context) error {
	if err := s.router.LoadRoutes(); err != nil {
		slog.Warn("failed to load persisted routes", "error", err)
	}

	s.health.Start()
	defer s.health.Stop()

	handler := s.proxyHandler()

	// HTTP server: redirects to HTTPS or serves ACME challenges.
	s.httpSrv = &http.Server{
		Addr:              s.cfg.HTTPAddr,
		Handler:           s.httpRedirectHandler(handler),
		ReadTimeout:       proxyReadTimeout,
		ReadHeaderTimeout: proxyReadTimeout,
		WriteTimeout:      proxyWriteTimeout,
		IdleTimeout:       proxyIdleTimeout,
	}

	// HTTPS server with TLS.
	s.tlsSrv = &http.Server{
		Addr:              s.cfg.HTTPSAddr,
		Handler:           handler,
		ReadTimeout:       proxyReadTimeout,
		ReadHeaderTimeout: proxyReadTimeout,
		WriteTimeout:      proxyWriteTimeout,
		IdleTimeout:       proxyIdleTimeout,
	}

	tlsCfg, err := s.buildTLSConfig(ctx)
	if err != nil {
		return err
	}
	if tlsCfg != nil {
		s.tlsSrv.TLSConfig = tlsCfg
	}

	// If ACME is active, wrap the HTTP handler with the challenge solver.
	if s.acme != nil {
		s.httpSrv.Handler = s.acme.httpChallengeHandler(s.httpSrv.Handler)
	}

	errCh := make(chan error, 2)

	// Start HTTP listener.
	go func() {
		slog.Info("proxy HTTP starting", "addr", s.cfg.HTTPAddr)
		if err := s.httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- fmt.Errorf("proxy HTTP: %w", err)
		}
	}()

	// Start HTTPS listener.
	go func() {
		slog.Info("proxy HTTPS starting", "addr", s.cfg.HTTPSAddr)
		var listenErr error
		if tlsCfg != nil {
			// TLSConfig is set; use ListenAndServeTLS with empty cert/key
			// to signal "use TLSConfig".
			listenErr = s.tlsSrv.ListenAndServeTLS("", "")
		} else {
			// Without TLS config, serve plain HTTP on the HTTPS port as a fallback.
			slog.Warn("no TLS configured, HTTPS port serving plain HTTP")
			listenErr = s.tlsSrv.ListenAndServe()
		}
		if listenErr != nil && !errors.Is(listenErr, http.ErrServerClosed) {
			errCh <- fmt.Errorf("proxy HTTPS: %w", listenErr)
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		return s.shutdown()
	}
}

// shutdown gracefully stops both HTTP and HTTPS servers.
func (s *Server) shutdown() error {
	slog.Info("proxy shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var errs []error
	if s.httpSrv != nil {
		if err := s.httpSrv.Shutdown(shutdownCtx); err != nil {
			errs = append(errs, fmt.Errorf("HTTP shutdown: %w", err))
		}
	}
	if s.tlsSrv != nil {
		if err := s.tlsSrv.Shutdown(shutdownCtx); err != nil {
			errs = append(errs, fmt.Errorf("HTTPS shutdown: %w", err))
		}
	}

	return errors.Join(errs...)
}

// proxyHandler returns the main reverse proxy HTTP handler.
func (s *Server) proxyHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		backend := s.router.Route(r.Host)
		if backend == nil {
			http.Error(w, "no route for host", http.StatusBadGateway)
			return
		}

		if !backend.Healthy {
			http.Error(w, "backend unhealthy", http.StatusServiceUnavailable)
			return
		}

		// Per-domain rate limiting.
		if backend.limiter != nil && !backend.limiter.Allow() {
			w.Header().Set("Retry-After", "1")
			http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
			return
		}

		// WebSocket passthrough: detect Upgrade header.
		if isWebSocket(r) {
			s.proxyWebSocket(w, r, backend)
			return
		}

		s.proxyHTTP(w, r, backend)
	})
}

// proxyHTTP proxies a standard HTTP request to the backend.
func (s *Server) proxyHTTP(w http.ResponseWriter, r *http.Request, b *Backend) {
	rp := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = b.URL.Scheme
			req.URL.Host = b.URL.Host
			req.Host = b.URL.Host

			setForwardHeaders(req, r)
		},
		ErrorHandler: func(rw http.ResponseWriter, req *http.Request, err error) {
			slog.Error("proxy error",
				"host", r.Host,
				"backend", b.URL.String(),
				"error", err,
			)
			http.Error(rw, "proxy error", http.StatusBadGateway)
		},
	}

	rp.ServeHTTP(w, r)
}

// proxyWebSocket proxies a WebSocket upgrade request using a transparent
// reverse proxy (httputil handles the Upgrade transparently).
func (s *Server) proxyWebSocket(w http.ResponseWriter, r *http.Request, b *Backend) {
	// httputil.ReverseProxy handles WebSocket upgrades transparently
	// when FlushInterval is set to -1.
	rp := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = b.URL.Scheme
			req.URL.Host = b.URL.Host
			req.Host = b.URL.Host

			setForwardHeaders(req, r)
		},
		FlushInterval: -1, // streaming mode for WebSockets
	}

	rp.ServeHTTP(w, r)
}

// httpRedirectHandler returns a handler that redirects HTTP to HTTPS.
// If TLS is not configured, it falls through to the proxy handler.
// When ACME is active, ACME challenge requests are handled before redirect.
func (s *Server) httpRedirectHandler(fallback http.Handler) http.Handler {
	tlsEnabled := s.cfg.SelfSigned || s.cfg.ACMEEmail != ""
	if !tlsEnabled {
		return fallback
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		target := "https://" + r.Host + r.URL.RequestURI()
		http.Redirect(w, r, target, http.StatusMovedPermanently)
	})
}

// buildTLSConfig selects the TLS provider based on configuration:
//   - SelfSigned=true: generates a self-signed certificate
//   - ACMEEmail set: uses certmagic for automatic Let's Encrypt certificates
//   - ACMEEmail set but certmagic fails: falls back to self-signed with a warning
//   - Neither: returns nil (no TLS)
func (s *Server) buildTLSConfig(ctx context.Context) (*tls.Config, error) {
	if s.cfg.SelfSigned {
		slog.Info("proxy: using self-signed TLS certificates")
		tlsCfg, err := s.selfSignedTLSConfig()
		if err != nil {
			return nil, fmt.Errorf("proxy: self-signed TLS: %w", err)
		}
		return tlsCfg, nil
	}

	if s.cfg.ACMEEmail != "" {
		acmeCfg := ACMEConfig{
			Email:   s.cfg.ACMEEmail,
			Domains: s.cfg.Domains,
			DataDir: s.cfg.DataDir,
			Staging: s.cfg.ACMEStaging,
		}

		provider, err := newACMEProvider(acmeCfg)
		if err != nil {
			slog.Warn("proxy: certmagic setup failed, falling back to self-signed",
				"error", err,
			)
			return s.selfSignedFallback()
		}

		if err := provider.manageAsync(ctx); err != nil {
			slog.Warn("proxy: certmagic domain management failed, falling back to self-signed",
				"error", err,
			)
			return s.selfSignedFallback()
		}

		s.acme = provider
		slog.Info("proxy: using certmagic for automatic TLS",
			"domains", s.cfg.Domains,
			"email", s.cfg.ACMEEmail,
		)

		tlsCfg := provider.tlsConfig()
		tlsCfg.MinVersion = tls.VersionTLS12
		return tlsCfg, nil
	}

	return nil, nil
}

// selfSignedFallback generates a self-signed TLS config as a graceful
// fallback when certmagic fails.
func (s *Server) selfSignedFallback() (*tls.Config, error) {
	slog.Warn("proxy: generating self-signed certificate as fallback")
	tlsCfg, err := s.selfSignedTLSConfig()
	if err != nil {
		return nil, fmt.Errorf("proxy: self-signed fallback: %w", err)
	}
	return tlsCfg, nil
}

// selfSignedTLSConfig generates a self-signed certificate for local development.
func (s *Server) selfSignedTLSConfig() (*tls.Config, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate key: %w", err)
	}

	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, fmt.Errorf("generate serial: %w", err)
	}

	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{Organization: []string{"Mozza Dev"}},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(selfSignedDuration),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     s.cfg.Domains,
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
	}

	if len(tmpl.DNSNames) == 0 {
		tmpl.DNSNames = []string{"localhost"}
	}

	certDER, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return nil, fmt.Errorf("create certificate: %w", err)
	}

	cert := tls.Certificate{
		Certificate: [][]byte{certDER},
		PrivateKey:  key,
	}

	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	}, nil
}

// setForwardHeaders sets standard reverse-proxy forwarding headers.
func setForwardHeaders(dst, src *http.Request) {
	clientIP, _, err := net.SplitHostPort(src.RemoteAddr)
	if err != nil {
		clientIP = src.RemoteAddr
	}

	dst.Header.Set("X-Forwarded-For", clientIP)
	dst.Header.Set("X-Real-IP", clientIP)

	proto := "http"
	if src.TLS != nil {
		proto = "https"
	}
	dst.Header.Set("X-Forwarded-Proto", proto)

	if dst.Header.Get("X-Request-ID") == "" {
		dst.Header.Set("X-Request-ID", uuid.New().String())
	}
}

// isWebSocket returns true when the request is a WebSocket upgrade.
func isWebSocket(r *http.Request) bool {
	return strings.EqualFold(r.Header.Get("Connection"), "upgrade") &&
		strings.EqualFold(r.Header.Get("Upgrade"), "websocket")
}
