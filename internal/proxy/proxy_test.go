package proxy

import (
	"context"
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRouterAddAndRoute(t *testing.T) {
	rt := NewRouter("")

	err := rt.AddRoute("app.example.com", "http://localhost:3000", "/healthz")
	require.NoError(t, err)

	b := rt.Route("app.example.com")
	require.NotNil(t, b)
	assert.Equal(t, "http://localhost:3000", b.RawURL)
	assert.Equal(t, "/healthz", b.HealthEndpoint)
	assert.True(t, b.Healthy)
}

func TestRouterRouteStripsPort(t *testing.T) {
	rt := NewRouter("")

	err := rt.AddRoute("app.example.com", "http://localhost:3000", "")
	require.NoError(t, err)

	b := rt.Route("app.example.com:443")
	require.NotNil(t, b, "should match domain after stripping port")
}

func TestRouterRouteNotFound(t *testing.T) {
	rt := NewRouter("")

	b := rt.Route("unknown.example.com")
	assert.Nil(t, b)
}

func TestRouterRemoveRoute(t *testing.T) {
	rt := NewRouter("")

	err := rt.AddRoute("app.example.com", "http://localhost:3000", "")
	require.NoError(t, err)

	rt.RemoveRoute("app.example.com")

	b := rt.Route("app.example.com")
	assert.Nil(t, b)
}

func TestRouterBackendsSnapshot(t *testing.T) {
	rt := NewRouter("")
	_ = rt.AddRoute("a.com", "http://localhost:3000", "")
	_ = rt.AddRoute("b.com", "http://localhost:4000", "")

	backends := rt.Backends()
	assert.Len(t, backends, 2)
	assert.Contains(t, backends, "a.com")
	assert.Contains(t, backends, "b.com")
}

func TestRouterPersistAndLoad(t *testing.T) {
	dir := t.TempDir()

	rt := NewRouter(dir)
	require.NoError(t, rt.AddRoute("persist.test", "http://localhost:5000", "/health"))

	// Create a new router and load persisted routes.
	rt2 := NewRouter(dir)
	require.NoError(t, rt2.LoadRoutes())

	b := rt2.Route("persist.test")
	require.NotNil(t, b)
	assert.Equal(t, "http://localhost:5000", b.RawURL)
	assert.Equal(t, "/health", b.HealthEndpoint)
}

func TestProxyHandlerNoRoute(t *testing.T) {
	cfg := Config{SelfSigned: false}
	srv := New(cfg)

	handler := srv.proxyHandler()

	req := httptest.NewRequest(http.MethodGet, "http://unknown.test/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadGateway, w.Code)
}

func TestProxyHandlerUnhealthyBackend(t *testing.T) {
	cfg := Config{SelfSigned: false}
	srv := New(cfg)

	_ = srv.router.AddRoute("sick.test", "http://localhost:9999", "")

	// Mark backend as unhealthy.
	srv.router.mu.Lock()
	srv.router.backends["sick.test"].Healthy = false
	srv.router.mu.Unlock()

	handler := srv.proxyHandler()
	req := httptest.NewRequest(http.MethodGet, "http://sick.test/", nil)
	req.Host = "sick.test"
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestProxyHandlerForwardsToBackend(t *testing.T) {
	// Spin up a fake backend.
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.NotEmpty(t, r.Header.Get("X-Forwarded-For"))
		assert.NotEmpty(t, r.Header.Get("X-Real-IP"))
		assert.NotEmpty(t, r.Header.Get("X-Request-ID"))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	defer backend.Close()

	cfg := Config{SelfSigned: false}
	srv := New(cfg)

	_ = srv.router.AddRoute("forward.test", backend.URL, "")

	handler := srv.proxyHandler()
	req := httptest.NewRequest(http.MethodGet, "/some/path", nil)
	req.Host = "forward.test"
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "ok", w.Body.String())
}

func TestHealthCheckerUpdatesStatus(t *testing.T) {
	// Create a backend that returns 200.
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	rt := NewRouter("")
	_ = rt.AddRoute("hc.test", backend.URL, "/")

	hc := NewHealthChecker(rt)

	// Run a single check cycle.
	hc.checkAll()

	b := rt.Route("hc.test")
	require.NotNil(t, b)
	assert.True(t, b.Healthy)
	assert.False(t, b.LastCheck.IsZero())
}

func TestHealthCheckerMarksUnhealthy(t *testing.T) {
	rt := NewRouter("")
	// Point at a port that won't respond.
	_ = rt.AddRoute("dead.test", "http://127.0.0.1:1", "/")

	hc := NewHealthChecker(rt)

	// Run enough checks to exceed the failure threshold.
	for i := 0; i < failureThreshold+1; i++ {
		hc.checkAll()
	}

	b := rt.Route("dead.test")
	require.NotNil(t, b)
	assert.False(t, b.Healthy)
}

func TestSelfSignedTLSConfig(t *testing.T) {
	cfg := Config{
		SelfSigned: true,
		Domains:    []string{"localhost", "test.local"},
	}
	srv := New(cfg)

	tlsCfg, err := srv.selfSignedTLSConfig()
	require.NoError(t, err)
	require.NotNil(t, tlsCfg)
	assert.Len(t, tlsCfg.Certificates, 1)
	assert.Equal(t, uint16(tls.VersionTLS12), tlsCfg.MinVersion)
}

func TestIsWebSocket(t *testing.T) {
	tests := []struct {
		name    string
		headers map[string]string
		want    bool
	}{
		{
			name: "websocket upgrade",
			headers: map[string]string{
				"Connection": "Upgrade",
				"Upgrade":    "websocket",
			},
			want: true,
		},
		{
			name:    "normal request",
			headers: map[string]string{},
			want:    false,
		},
		{
			name: "upgrade without websocket",
			headers: map[string]string{
				"Connection": "Upgrade",
				"Upgrade":    "h2c",
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}
			assert.Equal(t, tt.want, isWebSocket(req))
		})
	}
}

func TestServerShutdown(t *testing.T) {
	cfg := Config{
		HTTPAddr:  ":0",
		HTTPSAddr: ":0",
	}
	srv := New(cfg)

	ctx, cancel := context.WithCancel(context.Background())

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.ListenAndServe(ctx)
	}()

	// Give the server a moment to start.
	time.Sleep(100 * time.Millisecond)

	cancel()

	err := <-errCh
	assert.NoError(t, err)
}

func TestRateLimitReturns429(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	cfg := Config{SelfSigned: false}
	srv := New(cfg)

	// Add route with a very low rate limit (1 req/s).
	err := srv.router.AddRouteWithLimit("limited.test", backend.URL, "", 1)
	require.NoError(t, err)

	handler := srv.proxyHandler()

	// First request should succeed (uses the burst token).
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Host = "limited.test"
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// Second request immediately should be rate limited.
	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	req2.Host = "limited.test"
	w2 := httptest.NewRecorder()
	handler.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusTooManyRequests, w2.Code)
	assert.Equal(t, "1", w2.Header().Get("Retry-After"))
}

func TestRateLimitUnlimited(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	cfg := Config{SelfSigned: false}
	srv := New(cfg)

	// Add route with unlimited rate (0).
	err := srv.router.AddRouteWithLimit("unlimited.test", backend.URL, "", 0)
	require.NoError(t, err)

	handler := srv.proxyHandler()

	// Send multiple requests rapidly — none should be rate limited.
	for i := 0; i < 10; i++ {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Host = "unlimited.test"
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code, "request %d should not be rate limited", i)
	}
}

func TestDefaultRateLimitApplied(t *testing.T) {
	rt := NewRouter("")

	err := rt.AddRoute("default.test", "http://localhost:3000", "")
	require.NoError(t, err)

	b := rt.Route("default.test")
	require.NotNil(t, b)
	assert.Equal(t, defaultRateLimit, b.RateLimit)
	assert.NotNil(t, b.limiter, "default rate limit should create a limiter")
}

func TestBuildTLSConfigSelfSigned(t *testing.T) {
	cfg := Config{
		SelfSigned: true,
		Domains:    []string{"localhost"},
	}
	srv := New(cfg)

	tlsCfg, err := srv.buildTLSConfig(context.Background())
	require.NoError(t, err)
	require.NotNil(t, tlsCfg)
	assert.Len(t, tlsCfg.Certificates, 1)
	assert.Equal(t, uint16(tls.VersionTLS12), tlsCfg.MinVersion)
	assert.Nil(t, srv.acme, "acme should not be set for self-signed")
}

func TestBuildTLSConfigNoTLS(t *testing.T) {
	cfg := Config{
		SelfSigned: false,
		ACMEEmail:  "",
	}
	srv := New(cfg)

	tlsCfg, err := srv.buildTLSConfig(context.Background())
	require.NoError(t, err)
	assert.Nil(t, tlsCfg, "no TLS config when neither self-signed nor ACME")
	assert.Nil(t, srv.acme)
}

func TestNewACMEProviderRequiresEmail(t *testing.T) {
	_, err := newACMEProvider(ACMEConfig{
		Domains: []string{"example.com"},
		DataDir: t.TempDir(),
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "email is required")
}

func TestNewACMEProviderRequiresDomains(t *testing.T) {
	_, err := newACMEProvider(ACMEConfig{
		Email:   "test@example.com",
		DataDir: t.TempDir(),
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "at least one domain")
}

func TestNewACMEProviderSuccess(t *testing.T) {
	provider, err := newACMEProvider(ACMEConfig{
		Email:   "test@example.com",
		Domains: []string{"example.com"},
		DataDir: t.TempDir(),
		Staging: true,
	})
	require.NoError(t, err)
	require.NotNil(t, provider)
	assert.NotNil(t, provider.magic)
	assert.NotNil(t, provider.issuer)
}

func TestACMEProviderTLSConfig(t *testing.T) {
	provider, err := newACMEProvider(ACMEConfig{
		Email:   "test@example.com",
		Domains: []string{"example.com"},
		DataDir: t.TempDir(),
		Staging: true,
	})
	require.NoError(t, err)

	tlsCfg := provider.tlsConfig()
	require.NotNil(t, tlsCfg)
	// certmagic sets GetCertificate to dynamically provide certs
	assert.NotNil(t, tlsCfg.GetCertificate)
}

func TestHTTPRedirectWithACME(t *testing.T) {
	cfg := Config{
		SelfSigned: false,
		ACMEEmail:  "test@example.com",
	}
	srv := New(cfg)

	fallback := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := srv.httpRedirectHandler(fallback)

	req := httptest.NewRequest(http.MethodGet, "http://example.com/path", nil)
	req.Host = "example.com"
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusMovedPermanently, w.Code)
	assert.Contains(t, w.Header().Get("Location"), "https://example.com/path")
}

func TestHTTPRedirectNoTLS(t *testing.T) {
	cfg := Config{
		SelfSigned: false,
		ACMEEmail:  "",
	}
	srv := New(cfg)

	fallback := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := srv.httpRedirectHandler(fallback)

	req := httptest.NewRequest(http.MethodGet, "http://example.com/path", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	// Without TLS, should fall through to proxy handler (200).
	assert.Equal(t, http.StatusOK, w.Code)
}
