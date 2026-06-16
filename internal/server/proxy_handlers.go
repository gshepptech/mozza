package server

import (
	"net/http"
	"time"

	"github.com/gshepptech/mozza/internal/proxy"
)

// proxyRouteResponse is a single entry in the routing table API response.
type proxyRouteResponse struct {
	Domain         string    `json:"domain"`
	BackendURL     string    `json:"backend_url"`
	HealthEndpoint string    `json:"health_endpoint"`
	Healthy        bool      `json:"healthy"`
	LastCheck      time.Time `json:"last_check"`
}

// proxyCertResponse is a single entry in the certificate status API response.
type proxyCertResponse struct {
	Domain    string     `json:"domain"`
	Status    string     `json:"status"`
	Provider  string     `json:"provider,omitempty"`
	IssuedAt  *time.Time `json:"issued_at,omitempty"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

// handleProxyRoutes returns the current proxy routing table.
func (s *Server) handleProxyRoutes() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		if s.cfg.Proxy == nil {
			Error(w, http.StatusServiceUnavailable, "proxy not configured")
			return
		}

		backends := s.cfg.Proxy.Router().Backends()

		routes := make([]proxyRouteResponse, 0, len(backends))
		for domain, b := range backends {
			routes = append(routes, proxyRouteResponse{
				Domain:         domain,
				BackendURL:     b.RawURL,
				HealthEndpoint: b.HealthEndpoint,
				Healthy:        b.Healthy,
				LastCheck:      b.LastCheck,
			})
		}

		JSON(w, http.StatusOK, routes)
	}
}

// handleProxyCertificates returns TLS certificate status per domain.
func (s *Server) handleProxyCertificates() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.cfg.Store == nil {
			Error(w, http.StatusServiceUnavailable, "store not configured")
			return
		}

		certs, err := s.cfg.Store.ListCertificates(r.Context())
		if err != nil {
			Error(w, http.StatusInternalServerError, "failed to list certificates")
			return
		}

		resp := make([]proxyCertResponse, 0, len(certs))
		for _, c := range certs {
			resp = append(resp, proxyCertResponse{
				Domain:    c.Domain,
				Status:    c.Status,
				Provider:  c.Provider,
				IssuedAt:  c.IssuedAt,
				ExpiresAt: c.ExpiresAt,
			})
		}

		JSON(w, http.StatusOK, resp)
	}
}

// SetProxy attaches a running proxy server to the API server so that
// proxy status endpoints can query it. This allows the proxy to be
// optional — the server works without it.
func (s *Server) SetProxy(p *proxy.Server) {
	s.cfg.Proxy = p
}
