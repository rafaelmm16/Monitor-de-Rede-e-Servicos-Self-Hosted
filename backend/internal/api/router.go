package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/rafael/monitor-de-rede-backend/internal/models"
	"github.com/rafael/monitor-de-rede-backend/internal/notifier"
	"github.com/rafael/monitor-de-rede-backend/internal/scheduler"
	"github.com/rafael/monitor-de-rede-backend/internal/storage"
	"github.com/rafael/monitor-de-rede-backend/internal/ws"
)

type Server struct {
	store     *storage.Store
	scheduler *scheduler.Scheduler
	hub       *ws.Hub
}

func NewServer(store *storage.Store, sched *scheduler.Scheduler, hub *ws.Hub) http.Handler {
	s := &Server{
		store:     store,
		scheduler: sched,
		hub:       hub,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", s.handleHealth)
	mux.HandleFunc("GET /api/auth/status", s.handleAuthStatus)
	mux.HandleFunc("GET /api/status", s.handlePublicStatus)
	mux.HandleFunc("GET /metrics", s.handleMetrics)
	mux.HandleFunc("GET /api/ws", s.hub.HandleWS)

	mux.HandleFunc("GET /api/monitors", s.handleListMonitors)
	mux.HandleFunc("POST /api/monitors", s.handleCreateMonitor)
	mux.HandleFunc("GET /api/monitors/{id}", s.handleGetMonitor)
	mux.HandleFunc("PUT /api/monitors/{id}", s.handleUpdateMonitor)
	mux.HandleFunc("DELETE /api/monitors/{id}", s.handleDeleteMonitor)
	mux.HandleFunc("POST /api/monitors/{id}/check", s.handleTriggerCheck)
	mux.HandleFunc("GET /api/monitors/{id}/checks", s.handleListChecks)
	mux.HandleFunc("GET /api/monitors/{id}/incidents", s.handleListMonitorIncidents)

	mux.HandleFunc("GET /api/incidents", s.handleListAllIncidents)

	mux.HandleFunc("GET /api/webhooks", s.handleListWebhooks)
	mux.HandleFunc("POST /api/webhooks", s.handleCreateWebhook)
	mux.HandleFunc("DELETE /api/webhooks/{id}", s.handleDeleteWebhook)
	mux.HandleFunc("POST /api/webhooks/{id}/test", s.handleTestWebhook)

	mux.HandleFunc("GET /api/config/export", s.handleExportConfig)
	mux.HandleFunc("POST /api/config/import", s.handleImportConfig)

	return withCORS(withOptionalAuth(mux))
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-API-Key")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func withOptionalAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := os.Getenv("NETPULSE_AUTH_TOKEN")
		if token == "" {
			next.ServeHTTP(w, r)
			return
		}

		// Public read-only endpoints
		if r.Method == http.MethodGet &&
			(r.URL.Path == "/api/health" ||
				r.URL.Path == "/api/auth/status" ||
				r.URL.Path == "/api/status" ||
				r.URL.Path == "/metrics" ||
				r.URL.Path == "/api/ws") {
			next.ServeHTTP(w, r)
			return
		}

		// Check Bearer token or X-API-Key
		provided := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if provided == "" || provided == r.Header.Get("Authorization") {
			provided = r.Header.Get("X-API-Key")
		}
		if provided != token {
			writeError(w, http.StatusUnauthorized, "unauthorized: valid NETPULSE_AUTH_TOKEN required")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
		"time":   time.Now().UTC(),
	})
}

func (s *Server) handleAuthStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"auth_required": os.Getenv("NETPULSE_AUTH_TOKEN") != "",
	})
}

func (s *Server) handlePublicStatus(w http.ResponseWriter, r *http.Request) {
	monitors, err := s.store.ListMonitors()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	incidents, err := s.store.ListIncidents("", 15)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"generated_at": time.Now().UTC(),
		"monitors":     monitors,
		"incidents":    incidents,
	})
}

func (s *Server) handleMetrics(w http.ResponseWriter, r *http.Request) {
	monitors, err := s.store.ListMonitors()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	var b strings.Builder
	b.WriteString("# HELP netpulse_monitor_up Monitor health status (1=up, 0.5=degraded, 0=down)\n")
	b.WriteString("# TYPE netpulse_monitor_up gauge\n")
	for _, m := range monitors {
		val := 0.0
		switch m.Status {
		case models.StatusUp:
			val = 1.0
		case models.StatusDegraded:
			val = 0.5
		}
		fmt.Fprintf(&b, "netpulse_monitor_up{id=%q,name=%q,type=%q,target=%q} %g\n",
			m.ID, m.Name, m.Type, m.Target, val)
	}

	b.WriteString("# HELP netpulse_monitor_latency_ms Last recorded latency in milliseconds\n")
	b.WriteString("# TYPE netpulse_monitor_latency_ms gauge\n")
	for _, m := range monitors {
		fmt.Fprintf(&b, "netpulse_monitor_latency_ms{id=%q,name=%q,type=%q,target=%q} %d\n",
			m.ID, m.Name, m.Type, m.Target, m.LastLatencyMs)
	}

	b.WriteString("# HELP netpulse_monitor_uptime_24h_percent 24-hour uptime percentage\n")
	b.WriteString("# TYPE netpulse_monitor_uptime_24h_percent gauge\n")
	for _, m := range monitors {
		fmt.Fprintf(&b, "netpulse_monitor_uptime_24h_percent{id=%q,name=%q,type=%q,target=%q} %.2f\n",
			m.ID, m.Name, m.Type, m.Target, m.Uptime24h)
	}

	b.WriteString("# HELP netpulse_monitor_ssl_days_remaining Days remaining until SSL certificate expiration\n")
	b.WriteString("# TYPE netpulse_monitor_ssl_days_remaining gauge\n")
	for _, m := range monitors {
		if m.SSLDaysRemaining != nil {
			fmt.Fprintf(&b, "netpulse_monitor_ssl_days_remaining{id=%q,name=%q,target=%q} %d\n",
				m.ID, m.Name, m.Target, *m.SSLDaysRemaining)
		}
	}

	_, _ = w.Write([]byte(b.String()))
}

type configBackup struct {
	Version    string           `json:"version"`
	ExportedAt time.Time        `json:"exported_at"`
	Monitors   []models.Monitor `json:"monitors"`
	Webhooks   []models.Webhook `json:"webhooks"`
}

func (s *Server) handleExportConfig(w http.ResponseWriter, r *http.Request) {
	monitors, err := s.store.ListMonitors()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	webhooks, err := s.store.ListWebhooks()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, configBackup{
		Version:    "1.0",
		ExportedAt: time.Now().UTC(),
		Monitors:   monitors,
		Webhooks:   webhooks,
	})
}

func (s *Server) handleImportConfig(w http.ResponseWriter, r *http.Request) {
	var payload configBackup
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON backup payload")
		return
	}

	importedMonitors := 0
	for i := range payload.Monitors {
		m := payload.Monitors[i]
		if strings.TrimSpace(m.Name) == "" || strings.TrimSpace(m.Target) == "" {
			continue
		}
		m.ID = ""
		m.Status = models.StatusUnknown
		if err := s.store.CreateMonitor(&m); err == nil {
			s.scheduler.UpsertMonitor(m)
			s.hub.Broadcast("monitor_created", m)
			importedMonitors++
		}
	}

	importedWebhooks := 0
	for i := range payload.Webhooks {
		wh := payload.Webhooks[i]
		if strings.TrimSpace(wh.Name) == "" {
			continue
		}
		wh.ID = ""
		if err := s.store.CreateWebhook(&wh); err == nil {
			importedWebhooks++
		}
	}

	writeJSON(w, http.StatusOK, map[string]int{
		"imported_monitors": importedMonitors,
		"imported_webhooks": importedWebhooks,
	})
}

func (s *Server) handleListMonitors(w http.ResponseWriter, r *http.Request) {
	list, err := s.store.ListMonitors()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) handleCreateMonitor(w http.ResponseWriter, r *http.Request) {
	var m models.Monitor
	if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON payload")
		return
	}
	if strings.TrimSpace(m.Name) == "" || strings.TrimSpace(m.Target) == "" {
		writeError(w, http.StatusBadRequest, "name and target are required")
		return
	}
	if m.Type == "" {
		m.Type = models.MonitorHTTP
	}

	if err := s.store.CreateMonitor(&m); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.scheduler.UpsertMonitor(m)
	s.hub.Broadcast("monitor_created", m)
	writeJSON(w, http.StatusCreated, m)
}

func (s *Server) handleGetMonitor(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	m, err := s.store.GetMonitor(id)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, "monitor not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, m)
}

func (s *Server) handleUpdateMonitor(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	existing, err := s.store.GetMonitor(id)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, "monitor not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	var input models.Monitor
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON payload")
		return
	}

	input.ID = existing.ID
	if strings.TrimSpace(input.Name) == "" {
		input.Name = existing.Name
	}
	if strings.TrimSpace(input.Target) == "" {
		input.Target = existing.Target
	}
	if input.Type == "" {
		input.Type = existing.Type
	}

	if err := s.store.UpdateMonitor(&input); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	updated, err := s.store.GetMonitor(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.scheduler.UpsertMonitor(*updated)
	s.hub.Broadcast("monitor_updated", updated)
	writeJSON(w, http.StatusOK, updated)
}

func (s *Server) handleDeleteMonitor(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	s.scheduler.StopMonitor(id)
	if err := s.store.DeleteMonitor(id); errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, "monitor not found")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.hub.Broadcast("monitor_deleted", map[string]string{"id": id})
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleTriggerCheck(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.scheduler.TriggerNow(id); errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, "monitor not found")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "triggered"})
}

func (s *Server) handleListChecks(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	checks, err := s.store.ListChecks(id, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, checks)
}

func (s *Server) handleListMonitorIncidents(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	list, err := s.store.ListIncidents(id, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) handleListAllIncidents(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	list, err := s.store.ListIncidents("", limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) handleListWebhooks(w http.ResponseWriter, r *http.Request) {
	list, err := s.store.ListWebhooks()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) handleCreateWebhook(w http.ResponseWriter, r *http.Request) {
	var wh models.Webhook
	if err := json.NewDecoder(r.Body).Decode(&wh); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON payload")
		return
	}
	if strings.TrimSpace(wh.Name) == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	if wh.Type != models.WebhookDiscord && wh.Type != models.WebhookTelegram && wh.Type != models.WebhookGeneric {
		writeError(w, http.StatusBadRequest, "type must be discord, telegram, or generic")
		return
	}
	if err := s.store.CreateWebhook(&wh); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, wh)
}

func (s *Server) handleDeleteWebhook(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.store.DeleteWebhook(id); errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, "webhook not found")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleTestWebhook(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	wh, err := s.store.GetWebhook(id)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, "webhook not found")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	alert := notifier.AlertPayload{
		Monitor: models.Monitor{
			Name:          "Test Monitor",
			Type:          models.MonitorHTTP,
			Target:        "https://example.com",
			LastLatencyMs: 42,
		},
		OldStatus: models.StatusUp,
		NewStatus: models.StatusDown,
		Message:   "This is a test alert from NetPulse Monitor.",
		Timestamp: time.Now().UTC(),
	}

	if err := notifier.Send(r.Context(), *wh, alert); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "sent"})
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
