package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
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

	return withCORS(mux)
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
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
