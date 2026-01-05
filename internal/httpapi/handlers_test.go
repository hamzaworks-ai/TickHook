package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"log/slog"

	"github.com/cr0hn/tickhook/internal/config"
	"github.com/cr0hn/tickhook/internal/model"
	"github.com/cr0hn/tickhook/internal/store"
)

// mockStore implements store.Store for testing
type mockStore struct {
	jobs map[string]*model.Job
}

func newMockStore() *mockStore {
	return &mockStore{jobs: make(map[string]*model.Job)}
}

func (m *mockStore) CreateJob(ctx context.Context, job *model.Job) error {
	m.jobs[job.ID] = job
	return nil
}

func (m *mockStore) GetJob(ctx context.Context, jobID string) (*model.Job, error) {
	job, ok := m.jobs[jobID]
	if !ok {
		return nil, store.ErrJobNotFound
	}
	return job, nil
}

func (m *mockStore) UpdateJob(ctx context.Context, job *model.Job) error {
	m.jobs[job.ID] = job
	return nil
}

func (m *mockStore) DeleteJob(ctx context.Context, jobID string) error {
	delete(m.jobs, jobID)
	return nil
}

func (m *mockStore) GetDueJobs(ctx context.Context, nowMs int64, limit int) ([]string, error) {
	return nil, nil
}

func (m *mockStore) RemoveFromSchedule(ctx context.Context, jobID string) error {
	return nil
}

func (m *mockStore) AddToSchedule(ctx context.Context, jobID string, dueAtMs int64) error {
	return nil
}

func (m *mockStore) Close() error {
	return nil
}

func (m *mockStore) Ping(ctx context.Context) error {
	return nil
}

func TestServerHeader(t *testing.T) {
	cfg := &config.Config{
		AuthToken:        "test-token",
		DefaultTimeoutMs: 5000,
	}
	logger := slog.Default()
	mockStore := newMockStore()

	server := NewServer(cfg, mockStore, logger)

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	server.server.Handler.ServeHTTP(w, req)

	if w.Header().Get("Server") != "TickHook/1.0" {
		t.Errorf("Server header = %q, want %q", w.Header().Get("Server"), "TickHook/1.0")
	}
}

func TestAuthMiddleware_NoToken(t *testing.T) {
	cfg := &config.Config{
		AuthToken:        "test-token",
		DefaultTimeoutMs: 5000,
	}
	logger := slog.Default()
	mockStore := newMockStore()

	server := NewServer(cfg, mockStore, logger)

	req := httptest.NewRequest("GET", "/v1/jobs/test-id", nil)
	w := httptest.NewRecorder()

	server.server.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestAuthMiddleware_InvalidToken(t *testing.T) {
	cfg := &config.Config{
		AuthToken:        "test-token",
		DefaultTimeoutMs: 5000,
	}
	logger := slog.Default()
	mockStore := newMockStore()

	server := NewServer(cfg, mockStore, logger)

	req := httptest.NewRequest("GET", "/v1/jobs/test-id", nil)
	req.Header.Set("Authorization", "Bearer wrong-token")
	w := httptest.NewRecorder()

	server.server.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestAuthMiddleware_ValidToken(t *testing.T) {
	cfg := &config.Config{
		AuthToken:        "test-token",
		DefaultTimeoutMs: 5000,
	}
	logger := slog.Default()
	mockStore := newMockStore()

	server := NewServer(cfg, mockStore, logger)

	req := httptest.NewRequest("GET", "/v1/jobs/nonexistent", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	w := httptest.NewRecorder()

	server.server.Handler.ServeHTTP(w, req)

	// Should get 404 (not found) instead of 401 (unauthorized)
	if w.Code != http.StatusNotFound {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestHealthEndpoint_NoAuth(t *testing.T) {
	cfg := &config.Config{
		AuthToken:        "test-token",
		DefaultTimeoutMs: 5000,
	}
	logger := slog.Default()
	mockStore := newMockStore()

	server := NewServer(cfg, mockStore, logger)

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	server.server.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestCreateOneShotJob(t *testing.T) {
	cfg := &config.Config{
		AuthToken:        "test-token",
		DefaultTimeoutMs: 5000,
	}
	logger := slog.Default()
	mockStore := newMockStore()

	server := NewServer(cfg, mockStore, logger)

	body := `{
		"execute_at": "2026-01-15T10:00:00Z",
		"webhook": {
			"url": "https://example.com/hook",
			"method": "POST"
		}
	}`

	req := httptest.NewRequest("POST", "/v1/jobs/one-shot", bytes.NewBufferString(body))
	req.Header.Set("Authorization", "Bearer test-token")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.server.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("Status = %d, want %d, body: %s", w.Code, http.StatusCreated, w.Body.String())
	}

	var resp model.CreateJobResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if resp.JobID == "" {
		t.Error("JobID should not be empty")
	}

	// Verify job was stored
	if len(mockStore.jobs) != 1 {
		t.Errorf("Store should have 1 job, got %d", len(mockStore.jobs))
	}
}

func TestCreateOneShotJob_InvalidBody(t *testing.T) {
	cfg := &config.Config{
		AuthToken:        "test-token",
		DefaultTimeoutMs: 5000,
	}
	logger := slog.Default()
	mockStore := newMockStore()

	server := NewServer(cfg, mockStore, logger)

	body := `{"invalid json`

	req := httptest.NewRequest("POST", "/v1/jobs/one-shot", bytes.NewBufferString(body))
	req.Header.Set("Authorization", "Bearer test-token")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.server.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestCreateOneShotJob_MissingURL(t *testing.T) {
	cfg := &config.Config{
		AuthToken:        "test-token",
		DefaultTimeoutMs: 5000,
	}
	logger := slog.Default()
	mockStore := newMockStore()

	server := NewServer(cfg, mockStore, logger)

	body := `{
		"execute_at": "2026-01-15T10:00:00Z",
		"webhook": {
			"method": "POST"
		}
	}`

	req := httptest.NewRequest("POST", "/v1/jobs/one-shot", bytes.NewBufferString(body))
	req.Header.Set("Authorization", "Bearer test-token")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.server.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestCreateRecurringJob(t *testing.T) {
	cfg := &config.Config{
		AuthToken:        "test-token",
		DefaultTimeoutMs: 5000,
	}
	logger := slog.Default()
	mockStore := newMockStore()

	server := NewServer(cfg, mockStore, logger)

	body := `{
		"start_at": "2026-01-15T10:00:00Z",
		"interval_ms": 3600000,
		"webhook": {
			"url": "https://example.com/hook",
			"method": "POST"
		}
	}`

	req := httptest.NewRequest("POST", "/v1/jobs/recurring", bytes.NewBufferString(body))
	req.Header.Set("Authorization", "Bearer test-token")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.server.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("Status = %d, want %d, body: %s", w.Code, http.StatusCreated, w.Body.String())
	}

	// Verify job type
	for _, job := range mockStore.jobs {
		if job.Type != model.JobTypeRecurring {
			t.Errorf("Job type = %v, want %v", job.Type, model.JobTypeRecurring)
		}
		if job.IntervalMs != 3600000 {
			t.Errorf("IntervalMs = %d, want 3600000", job.IntervalMs)
		}
	}
}

func TestDeleteJob(t *testing.T) {
	cfg := &config.Config{
		AuthToken:        "test-token",
		DefaultTimeoutMs: 5000,
	}
	logger := slog.Default()
	mockStore := newMockStore()

	// Pre-create a job
	mockStore.jobs["test-job-id"] = &model.Job{
		ID:   "test-job-id",
		Type: model.JobTypeOneShot,
	}

	server := NewServer(cfg, mockStore, logger)

	req := httptest.NewRequest("DELETE", "/v1/jobs/test-job-id", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	w := httptest.NewRecorder()

	server.server.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp model.DeleteJobResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if !resp.Deleted {
		t.Error("Deleted should be true")
	}

	// Verify job was deleted
	if len(mockStore.jobs) != 0 {
		t.Errorf("Store should have 0 jobs, got %d", len(mockStore.jobs))
	}
}

func TestDeleteJob_NotFound(t *testing.T) {
	cfg := &config.Config{
		AuthToken:        "test-token",
		DefaultTimeoutMs: 5000,
	}
	logger := slog.Default()
	mockStore := newMockStore()

	server := NewServer(cfg, mockStore, logger)

	req := httptest.NewRequest("DELETE", "/v1/jobs/nonexistent", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	w := httptest.NewRecorder()

	server.server.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestGetJob(t *testing.T) {
	cfg := &config.Config{
		AuthToken:        "test-token",
		DefaultTimeoutMs: 5000,
	}
	logger := slog.Default()
	mockStore := newMockStore()

	// Pre-create a job
	mockStore.jobs["test-job-id"] = &model.Job{
		ID:          "test-job-id",
		Type:        model.JobTypeOneShot,
		URL:         "https://example.com/hook",
		Method:      "POST",
		MaxAttempts: 3,
	}

	server := NewServer(cfg, mockStore, logger)

	req := httptest.NewRequest("GET", "/v1/jobs/test-job-id", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	w := httptest.NewRecorder()

	server.server.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusOK)
	}

	var job model.Job
	if err := json.Unmarshal(w.Body.Bytes(), &job); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if job.ID != "test-job-id" {
		t.Errorf("Job ID = %q, want %q", job.ID, "test-job-id")
	}
}
