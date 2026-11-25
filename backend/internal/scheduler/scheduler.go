package scheduler

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/rafael/monitor-de-rede-backend/internal/checker"
	"github.com/rafael/monitor-de-rede-backend/internal/models"
	"github.com/rafael/monitor-de-rede-backend/internal/notifier"
	"github.com/rafael/monitor-de-rede-backend/internal/storage"
	"github.com/rafael/monitor-de-rede-backend/internal/ws"
)

type Scheduler struct {
	store *storage.Store
	hub   *ws.Hub

	mu         sync.Mutex
	cancels    map[string]context.CancelFunc
	failStreak map[string]int
	rootCtx    context.Context
}

func New(store *storage.Store, hub *ws.Hub) *Scheduler {
	return &Scheduler{
		store:      store,
		hub:        hub,
		cancels:    make(map[string]context.CancelFunc),
		failStreak: make(map[string]int),
	}
}

func (s *Scheduler) Start(ctx context.Context) error {
	s.mu.Lock()
	s.rootCtx = ctx
	s.mu.Unlock()

	monitors, err := s.store.ListMonitors()
	if err != nil {
		return err
	}

	for _, m := range monitors {
		if m.Enabled {
			s.startMonitorLoop(m)
		}
	}
	return nil
}

func (s *Scheduler) UpsertMonitor(m models.Monitor) {
	s.StopMonitor(m.ID)
	if m.Enabled {
		s.startMonitorLoop(m)
	}
}

func (s *Scheduler) StopMonitor(id string) {
	s.mu.Lock()
	cancel, ok := s.cancels[id]
	if ok {
		delete(s.cancels, id)
	}
	s.mu.Unlock()
	if ok && cancel != nil {
		cancel()
	}
}

func (s *Scheduler) TriggerNow(id string) error {
	m, err := s.store.GetMonitor(id)
	if err != nil {
		return err
	}
	go s.executeAndProcess(*m)
	return nil
}

func (s *Scheduler) startMonitorLoop(m models.Monitor) {
	s.mu.Lock()
	if s.rootCtx == nil {
		s.mu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(s.rootCtx)
	s.cancels[m.ID] = cancel
	s.mu.Unlock()

	interval := time.Duration(m.IntervalSeconds) * time.Second
	if interval < 5*time.Second {
		interval = 30 * time.Second
	}

	go func() {
		// Immediate initial check
		s.executeAndProcess(m)

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				// Reload latest monitor config before executing
				latest, err := s.store.GetMonitor(m.ID)
				if err != nil || !latest.Enabled {
					return
				}
				s.executeAndProcess(*latest)
			}
		}
	}()
}

func (s *Scheduler) executeAndProcess(m models.Monitor) {
	ctx := s.rootCtx
	if ctx == nil {
		ctx = context.Background()
	}

	oldStatus := m.Status
	res := checker.Run(ctx, m)

	// Apply max_retries confirmation before flipping monitor to DOWN
	maxRetries := m.MaxRetries
	if maxRetries < 1 {
		maxRetries = 1
	}

	s.mu.Lock()
	if res.Status == models.StatusDown {
		s.failStreak[m.ID]++
		if s.failStreak[m.ID] < maxRetries && oldStatus != models.StatusDown {
			// Not yet confirmed down: keep previous operational status on monitor
			if oldStatus == models.StatusUnknown {
				res.Status = models.StatusUp
			} else {
				res.Status = oldStatus
			}
		}
	} else {
		s.failStreak[m.ID] = 0
	}
	s.mu.Unlock()

	if err := s.store.RecordCheck(&res); err != nil {
		log.Printf("failed to record check for %s: %v", m.ID, err)
		return
	}

	// Handle state transitions, incidents, and webhook alerts
	if oldStatus != res.Status {
		if res.Status == models.StatusDown {
			cause := res.Error
			if cause == "" {
				cause = "Health check failed"
			}
			inc, err := s.store.OpenIncident(m.ID, m.Name, cause, res.CheckedAt)
			if err == nil && inc != nil {
				s.hub.Broadcast("incident_opened", inc)
			}
		} else if (oldStatus == models.StatusDown || oldStatus == models.StatusDegraded) && res.Status == models.StatusUp {
			inc, err := s.store.ResolveOpenIncidents(m.ID, res.CheckedAt)
			if err == nil && inc != nil {
				s.hub.Broadcast("incident_resolved", inc)
			}
		}

		// Notify webhooks on meaningful transitions unless in maintenance mode
		if !m.Maintenance && !(oldStatus == models.StatusUnknown && res.Status == models.StatusUp) {
			s.dispatchAlerts(m, oldStatus, res)
		}
	}

	updated, err := s.store.GetMonitor(m.ID)
	if err == nil {
		s.hub.Broadcast("check_result", map[string]any{
			"monitor": updated,
			"check":   res,
		})
	}
}

func (s *Scheduler) dispatchAlerts(m models.Monitor, oldStatus models.MonitorStatus, res models.CheckResult) {
	webhooks, err := s.store.ListWebhooks()
	if err != nil {
		return
	}

	msg := res.Error
	if msg == "" {
		msg = fmt.Sprintf("Service recovered (latency: %d ms)", res.LatencyMs)
	}

	m.Status = res.Status
	m.LastLatencyMs = res.LatencyMs
	m.LastError = res.Error

	alert := notifier.AlertPayload{
		Monitor:   m,
		OldStatus: oldStatus,
		NewStatus: res.Status,
		Message:   msg,
		Timestamp: res.CheckedAt,
	}

	for _, wh := range webhooks {
		if !wh.Enabled {
			continue
		}
		go func(w models.Webhook) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if err := notifier.Send(ctx, w, alert); err != nil {
				log.Printf("webhook %s (%s) failed: %v", w.Name, w.Type, err)
			}
		}(wh)
	}
}
