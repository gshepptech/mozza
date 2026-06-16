package server

import (
	"log/slog"
	"net"
	"net/http"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"time"
)

// isLoopbackIP returns true if the IP is a loopback address.
func isLoopbackIP(ip string) bool {
	parsed := net.ParseIP(ip)
	return parsed != nil && parsed.IsLoopback()
}

// applyMiddleware attaches global middleware to the router in the correct order.
// Note: jsonContentType is applied only to API routes (see registerRoutes).
func (s *Server) applyMiddleware() {
	s.router.Use(recoverer)
	s.router.Use(securityHeaders)
	s.router.Use(maxRequestBody)
	s.router.Use(requestLogger)
}

// jsonContentType sets the Content-Type header to application/json for every response.
func jsonContentType(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		next.ServeHTTP(w, r)
	})
}

// responseRecorder wraps http.ResponseWriter to capture the status code
// written by downstream handlers.
type responseRecorder struct {
	http.ResponseWriter
	statusCode int
}

// WriteHeader captures the status code before delegating to the wrapped writer.
func (rr *responseRecorder) WriteHeader(code int) {
	rr.statusCode = code
	rr.ResponseWriter.WriteHeader(code)
}

// requestLogger logs the method, path, status code, and duration of each request.
func requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &responseRecorder{ResponseWriter: w, statusCode: http.StatusOK}

		next.ServeHTTP(rec, r)

		slog.Info("request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rec.statusCode,
			"duration", time.Since(start),
		)
	})
}

// securityHeaders sets standard security response headers on every response.
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:")
		next.ServeHTTP(w, r)
	})
}

// maxBodySize is the maximum allowed request body size in bytes.
const maxBodySize = 1 << 20 // 1 MB

// maxRequestBody limits the size of incoming request bodies to prevent
// memory exhaustion from oversized payloads.
func maxRequestBody(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)
		next.ServeHTTP(w, r)
	})
}

// roleErrorResponse is the JSON structure returned when role authorization fails.
type roleErrorResponse struct {
	Error        string `json:"error"`
	RequiredRole string `json:"required_role"`
}

// requireRole returns middleware that enforces role-based access control.
// It checks the authenticated user's role against the allowed roles.
// Returns 401 if no user is present, 403 if the user's role is not allowed.
func requireRole(roles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user := UserFromContext(r.Context())
			if user == nil {
				Error(w, http.StatusUnauthorized, "authentication required")
				return
			}
			for _, role := range roles {
				if user.Role == role {
					next.ServeHTTP(w, r)
					return
				}
			}
			lowest := roles[0]
			JSON(w, http.StatusForbidden, roleErrorResponse{
				Error:        "insufficient permissions",
				RequiredRole: lowest,
			})
		})
	}
}

// rateLimiter implements a simple per-IP token bucket rate limiter.
type rateLimiter struct {
	mu      sync.Mutex
	clients map[string]*bucket
	rpm     int
}

type bucket struct {
	tokens    int
	lastReset time.Time
}

func newRateLimiter(rpm int) *rateLimiter {
	rl := &rateLimiter{clients: make(map[string]*bucket), rpm: rpm}
	go rl.cleanup()
	return rl
}

// allowResult holds the result of a rate limit check.
type allowResult struct {
	allowed   bool
	remaining int
	resetAt   time.Time
}

func (rl *rateLimiter) allow(ip string) allowResult {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	b, ok := rl.clients[ip]
	if !ok || time.Since(b.lastReset) > time.Minute {
		rl.clients[ip] = &bucket{tokens: rl.rpm - 1, lastReset: time.Now()}
		return allowResult{allowed: true, remaining: rl.rpm - 1, resetAt: time.Now().Add(time.Minute)}
	}

	if b.tokens <= 0 {
		return allowResult{allowed: false, remaining: 0, resetAt: b.lastReset.Add(time.Minute)}
	}
	b.tokens--
	return allowResult{allowed: true, remaining: b.tokens, resetAt: b.lastReset.Add(time.Minute)}
}

// cleanup periodically removes stale entries to prevent unbounded growth.
func (rl *rateLimiter) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	for range ticker.C {
		rl.mu.Lock()
		for ip, b := range rl.clients {
			if time.Since(b.lastReset) > 2*time.Minute {
				delete(rl.clients, ip)
			}
		}
		rl.mu.Unlock()
	}
}

// rateLimit wraps a handler with per-IP rate limiting and standard headers.
func rateLimit(rpm int) func(http.Handler) http.Handler {
	rl := newRateLimiter(rpm)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Use RemoteAddr as the primary key. X-Forwarded-For is only used
			// when the request comes through a trusted proxy (RemoteAddr is loopback).
			// This prevents attackers from spoofing X-Forwarded-For to bypass limits.
			ip, _, _ := net.SplitHostPort(r.RemoteAddr)
			if ip == "" {
				ip = r.RemoteAddr
			}
			if isLoopbackIP(ip) {
				if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
					// Take the leftmost (client) IP from the chain.
					if idx := strings.Index(fwd, ","); idx != -1 {
						ip = strings.TrimSpace(fwd[:idx])
					} else {
						ip = strings.TrimSpace(fwd)
					}
				}
			}
			result := rl.allow(ip)
			w.Header().Set("X-RateLimit-Limit", strconv.Itoa(rpm))
			w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(result.remaining))
			w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(result.resetAt.Unix(), 10))
			if !result.allowed {
				retryAfter := int(time.Until(result.resetAt).Seconds())
				if retryAfter < 1 {
					retryAfter = 1
				}
				w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
				rateLimitRejectionsTotal.Inc()
				Error(w, http.StatusTooManyRequests, "too many requests")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// recoverer catches panics in downstream handlers and returns a 500 response
// instead of crashing the server.
func recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				slog.Error("panic recovered",
					"panic", rec,
					"stack", string(debug.Stack()),
				)
				Error(w, http.StatusInternalServerError, "internal server error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}
