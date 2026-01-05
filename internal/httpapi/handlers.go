// Package httpapi implements the HTTP API server for TickHook.
package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/google/uuid"

	"github.com/cr0hn/tickhook/internal/model"
	"github.com/cr0hn/tickhook/internal/store"
)

// handleHealth handles the health check endpoint.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if err := s.store.Ping(ctx); err != nil {
		writeError(w, http.StatusServiceUnavailable, "unhealthy", "Redis connection failed")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// handleCreateOneShot handles POST /v1/jobs/one-shot
// PRD Reference: Section 9 - REST API, Section 14 - One-Shot Jobs
func (s *Server) handleCreateOneShot(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req model.CreateOneShotRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Invalid JSON body: "+err.Error())
		return
	}

	if err := req.Validate(s.cfg.DefaultTimeoutMs); err != nil {
		writeError(w, http.StatusBadRequest, "validation_error", err.Error())
		return
	}

	jobID := uuid.New().String()
	job := model.NewOneShotJob(jobID, &req, s.cfg.DefaultTimeoutMs)

	if err := s.store.CreateJob(ctx, job); err != nil {
		s.logger.Error("Failed to create one-shot job", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to create job")
		return
	}

	s.logger.Info("Created one-shot job", "job_id", jobID, "due_at_ms", job.DueAtMs)
	writeJSON(w, http.StatusCreated, model.CreateJobResponse{JobID: jobID})
}

// handleCreateRecurring handles POST /v1/jobs/recurring
// PRD Reference: Section 9 - REST API, Section 13 - Recurring Jobs
func (s *Server) handleCreateRecurring(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req model.CreateRecurringRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Invalid JSON body: "+err.Error())
		return
	}

	if err := req.Validate(s.cfg.DefaultTimeoutMs); err != nil {
		writeError(w, http.StatusBadRequest, "validation_error", err.Error())
		return
	}

	jobID := uuid.New().String()
	job := model.NewRecurringJob(jobID, &req, s.cfg.DefaultTimeoutMs)

	if err := s.store.CreateJob(ctx, job); err != nil {
		s.logger.Error("Failed to create recurring job", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to create job")
		return
	}

	s.logger.Info("Created recurring job", "job_id", jobID, "due_at_ms", job.DueAtMs, "interval_ms", job.IntervalMs)
	writeJSON(w, http.StatusCreated, model.CreateJobResponse{JobID: jobID})
}

// handleGetJob handles GET /v1/jobs/{job_id}
// PRD Reference: Section 9 - REST API
func (s *Server) handleGetJob(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	jobID := r.PathValue("job_id")

	if jobID == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "job_id is required")
		return
	}

	job, err := s.store.GetJob(ctx, jobID)
	if err != nil {
		if errors.Is(err, store.ErrJobNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "Job not found")
			return
		}
		s.logger.Error("Failed to get job", "job_id", jobID, "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to get job")
		return
	}

	writeJSON(w, http.StatusOK, job)
}

// handleDeleteJob handles DELETE /v1/jobs/{job_id}
// PRD Reference: Section 9 - REST API
func (s *Server) handleDeleteJob(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	jobID := r.PathValue("job_id")

	if jobID == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "job_id is required")
		return
	}

	// Check if job exists first
	_, err := s.store.GetJob(ctx, jobID)
	if err != nil {
		if errors.Is(err, store.ErrJobNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "Job not found")
			return
		}
		s.logger.Error("Failed to get job for deletion", "job_id", jobID, "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to delete job")
		return
	}

	if err := s.store.DeleteJob(ctx, jobID); err != nil {
		s.logger.Error("Failed to delete job", "job_id", jobID, "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to delete job")
		return
	}

	s.logger.Info("Deleted job", "job_id", jobID)
	writeJSON(w, http.StatusOK, model.DeleteJobResponse{Deleted: true})
}

// writeJSON writes a JSON response.
func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

// writeError writes an error response.
func writeError(w http.ResponseWriter, status int, errCode, message string) {
	writeJSON(w, status, model.ErrorResponse{
		Error:   errCode,
		Message: message,
	})
}
