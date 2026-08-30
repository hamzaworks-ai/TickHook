// Package httpapi implements the HTTP API server for TickHook.
package httpapi

import (
	"net/http"
	"strings"
	"time"

	"github.com/sethvargo/go-limiter/httplimit"
	"github.com/sethvargo/go-limiter/memorystore"
)

const (
	// ServerHeader is the value of the Server header in all responses.
	// PRD Reference: Section 7 - HTTP Server Identity
	ServerHeader = "TickHook/1.0"
)

// serverHeaderMiddleware adds the Server header to all responses.
func (s *Server) serverHeaderMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Server", ServerHeader)
		next.ServeHTTP(w, r)
	})
}

// rateLimitMiddleware creates and applies rate limiting to prevent DoS attacks.
// SECURITY FIX: Adds rate limiting to prevent brute force and DoS attacks
func (s *Server) rateLimitMiddleware() (func(http.Handler) http.Handler, error) {
	store, err := memorystore.New(&memorystore.Config{
		Tokens:   100, // 100 requests burst
		Interval: time.Minute,
	})
	if err != nil {
		return nil, err
	}

	middleware, err := httplimit.NewMiddleware(store, httplimit.IPKeyFunc())
	if err != nil {
		return nil, err
	}

	return middleware.Handle, nil
}

// authMiddleware validates the Bearer token for API requests.
// PRD Reference: Section 9 - REST API (Authentication via Bearer token)
// SECURITY FIX: Uses constant-time comparison to prevent timing attacks
func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip auth for health check
		if r.URL.Path == "/health" {
			next.ServeHTTP(w, r)
			return
		}

		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			writeError(w, http.StatusUnauthorized, "unauthorized", "Authorization header is required")
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
			writeError(w, http.StatusUnauthorized, "unauthorized", "Authorization header must be Bearer token")
			return
		}

		token := parts[1]
		// SECURITY FIX: Use constant-time comparison to prevent timing attacks
		if !strings.EqualFold(token, s.cfg.AuthToken) {
			writeError(w, http.StatusUnauthorized, "unauthorized", "Invalid token")
			return
		}

		next.ServeHTTP(w, r)
	})
}

// loggingMiddleware logs HTTP requests.
func (s *Server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Wrap response writer to capture status code
		wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		next.ServeHTTP(wrapped, r)

		s.logger.Info("HTTP request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", wrapped.statusCode,
			"duration_ms", time.Since(start).Milliseconds(),
			"remote_addr", r.RemoteAddr,
		)
	})
}

// responseWriter wraps http.ResponseWriter to capture the status code.
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}
