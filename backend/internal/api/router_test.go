package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/rafael/monitor-de-rede-backend/internal/api"
	"github.com/rafael/monitor-de-rede-backend/internal/models"
	"github.com/rafael/monitor-de-rede-backend/internal/scheduler"
	"github.com/rafael/monitor-de-rede-backend/internal/storage"
	"github.com/rafael/monitor-de-rede-backend/internal/ws"
)

func TestMonitorEndpoints(t *testing.T) {
	store, err := storage.New(filepath.Join(t.TempDir(), "api.db"))
	if err != nil {
		t.Fatalf("store.New: %v", err)
	}
	defer store.Close()

	hub := ws.NewHub()
	sched := scheduler.New(store, hub)
	handler := api.NewServer(store, sched, hub)

	// Health
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 on /api/health, got %d", rec.Code)
	}

	// Create monitor
	body, _ := json.Marshal(models.Monitor{
		Name:    "Test TCP",
		Type:    models.MonitorTCP,
		Target:  "127.0.0.1",
		Port:    8080,
		Enabled: false,
	})
	req = httptest.NewRequest(http.MethodPost, "/api/monitors", bytes.NewReader(body))
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 on POST /api/monitors, got %d: %s", rec.Code, rec.Body.String())
	}

	var created models.Monitor
	_ = json.Unmarshal(rec.Body.Bytes(), &created)
	if created.ID == "" {
		t.Fatalf("expected non-empty monitor ID")
	}

	// List monitors
	req = httptest.NewRequest(http.MethodGet, "/api/monitors", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 on GET /api/monitors, got %d", rec.Code)
	}

	// Delete monitor
	req = httptest.NewRequest(http.MethodDelete, "/api/monitors/"+created.ID, nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204 on DELETE /api/monitors, got %d", rec.Code)
	}
}

func TestMetricsStatusBackupAndAuth(t *testing.T) {
	store, err := storage.New(":memory:")
	if err != nil {
		t.Fatalf("store.New: %v", err)
	}
	defer store.Close()

	hub := ws.NewHub()
	sched := scheduler.New(store, hub)
	handler := api.NewServer(store, sched, hub)

	t.Setenv("NETPULSE_AUTH_TOKEN", "secret-token")

	// Public endpoints work without token
	for _, path := range []string{"/api/health", "/api/auth/status", "/api/status", "/metrics"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 on public %s, got %d", path, rec.Code)
		}
	}

	// Mutating endpoint rejected without token
	body, _ := json.Marshal(models.Monitor{
		Name:       "Backup Test",
		Type:       models.MonitorTCP,
		Target:     "127.0.0.1",
		Port:       80,
		MaxRetries: 3,
		Enabled:    false,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/monitors", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without token, got %d", rec.Code)
	}

	// Mutating endpoint succeeds with Bearer token
	req = httptest.NewRequest(http.MethodPost, "/api/monitors", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer secret-token")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 with token, got %d", rec.Code)
	}

	// Export config
	req = httptest.NewRequest(http.MethodGet, "/api/config/export", nil)
	req.Header.Set("X-API-Key", "secret-token")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 on export, got %d", rec.Code)
	}

	// Import config
	req = httptest.NewRequest(http.MethodPost, "/api/config/import", bytes.NewReader(rec.Body.Bytes()))
	req.Header.Set("Authorization", "Bearer secret-token")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 on import, got %d: %s", rec.Code, rec.Body.String())
	}

	// Badge SVG endpoint (public)
	monitors, _ := store.ListMonitors()
	if len(monitors) == 0 {
		t.Fatalf("expected at least 1 monitor for badge test")
	}
	req = httptest.NewRequest(http.MethodGet, "/api/badge/"+monitors[0].ID, nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "image/svg+xml" {
		t.Fatalf("expected 200 image/svg+xml on badge, got %d (%s)", rec.Code, rec.Header().Get("Content-Type"))
	}
}
