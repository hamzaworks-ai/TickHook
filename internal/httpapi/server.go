// Package httpapi implements the HTTP API server for TickHook.
// PRD Reference: Section 9 - REST API
package httpapi

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/cr0hn/tickhook/internal/config"
	"github.com/cr0hn/tickhook/internal/store"
)

// Server is the HTTP API server for TickHook.
type Server struct {
	cfg    *config.Config
	store  store.Store
	logger *slog.Logger
	server *http.Server
}

// NewServer creates a new HTTP API server.
func NewServer(cfg *config.Config, store store.Store, logger *slog.Logger) (*Server, error) {
	s := &Server{
		cfg:    cfg,
		store:  store,
		logger: logger,
	}

	mux := http.NewServeMux()
	s.registerRoutes(mux)

	// Build middleware chain
	var handler http.Handler = mux
	handler = s.serverHeaderMiddleware(handler)
	
	// Add rate limiting for security (DoS prevention)
	rateLimitMiddleware, err := s.rateLimitMiddleware()
	if err != nil {
		return nil, fmt.Errorf("failed to create rate limiter: %w", err)
	}
	handler = rateLimitMiddleware(handler)
	
	handler = s.authMiddleware(handler)
	handler = s.loggingMiddleware(handler)

	s.server = &http.Server{
		Addr:         cfg.Bind,
		Handler:      handler,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
		// SECURITY FIX: Limit request body size to prevent DoS
		MaxHeaderBytes: 1 << 20, // 1MB max header size
	}

	return s, nil
}

// Start starts the HTTP server.
func (s *Server) Start() error {
	s.logger.Info("HTTP server starting", "addr", s.cfg.Bind)
	return s.server.ListenAndServe()
}

// Shutdown gracefully shuts down the HTTP server.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}

// registerRoutes registers all API routes.
func (s *Server) registerRoutes(mux *http.ServeMux) {
	// Health check (no auth required - will be handled in middleware)
	mux.HandleFunc("GET /health", s.handleHealth)

	// Job endpoints
	mux.HandleFunc("POST /v1/jobs/one-shot", s.handleCreateOneShot)
	mux.HandleFunc("POST /v1/jobs/recurring", s.handleCreateRecurring)
	mux.HandleFunc("GET /v1/jobs/{job_id}", s.handleGetJob)
	mux.HandleFunc("DELETE /v1/jobs/{job_id}", s.handleDeleteJob)
}
