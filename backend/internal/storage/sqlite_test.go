package storage_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/rafael/monitor-de-rede-backend/internal/models"
	"github.com/rafael/monitor-de-rede-backend/internal/storage"
)

func TestStoreCRUDAndIncidents(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	store, err := storage.New(dbPath)
	if err != nil {
		t.Fatalf("store.New failed: %v", err)
	}
	defer store.Close()

	m := &models.Monitor{
		Name:            "API Gateway",
		Type:            models.MonitorHTTP,
		Target:          "https://api.example.com/health",
		IntervalSeconds: 30,
		TimeoutSeconds:  5,
		ExpectedStatus:  200,
		Enabled:         true,
	}
	if err := store.CreateMonitor(m); err != nil {
		t.Fatalf("CreateMonitor failed: %v", err)
	}

	got, err := store.GetMonitor(m.ID)
	if err != nil {
		t.Fatalf("GetMonitor failed: %v", err)
	}
	if got.Name != m.Name || got.Target != m.Target {
		t.Fatalf("unexpected monitor: %+v", got)
	}

	now := time.Now().UTC()
	if err := store.RecordCheck(&models.CheckResult{
		MonitorID: m.ID,
		Status:    models.StatusUp,
		LatencyMs: 25,
		CheckedAt: now,
	}); err != nil {
		t.Fatalf("RecordCheck failed: %v", err)
	}

	inc, err := store.OpenIncident(m.ID, m.Name, "connection refused", now)
	if err != nil || inc == nil {
		t.Fatalf("OpenIncident failed: %v", err)
	}

	resolved, err := store.ResolveOpenIncidents(m.ID, now.Add(15*time.Second))
	if err != nil || resolved == nil {
		t.Fatalf("ResolveOpenIncidents failed: %v", err)
	}
	if resolved.DurationSeconds != 15 {
		t.Fatalf("expected 15s duration, got %d", resolved.DurationSeconds)
	}

	wh := &models.Webhook{
		Name:    "Discord Ops",
		Type:    models.WebhookDiscord,
		URL:     "https://discord.com/api/webhooks/test",
		Enabled: true,
	}
	if err := store.CreateWebhook(wh); err != nil {
		t.Fatalf("CreateWebhook failed: %v", err)
	}
	webhooks, err := store.ListWebhooks()
	if err != nil || len(webhooks) != 1 {
		t.Fatalf("expected 1 webhook, got %d (err: %v)", len(webhooks), err)
	}
}
