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
