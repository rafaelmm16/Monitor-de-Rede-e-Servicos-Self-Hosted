package storage

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/rafael/monitor-de-rede-backend/internal/models"
	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

func New(dbPath string) (*Store, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)

	if _, err := db.Exec(`PRAGMA journal_mode=WAL; PRAGMA foreign_keys=ON;`); err != nil {
		_ = db.Close()
		return nil, err
	}

	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS monitors (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		type TEXT NOT NULL,
		target TEXT NOT NULL,
		port INTEGER NOT NULL DEFAULT 0,
		interval_seconds INTEGER NOT NULL DEFAULT 30,
		timeout_seconds INTEGER NOT NULL DEFAULT 5,
		expected_status INTEGER NOT NULL DEFAULT 200,
		keyword TEXT NOT NULL DEFAULT '',
		ssl_expiry_warn_days INTEGER NOT NULL DEFAULT 14,
		max_retries INTEGER NOT NULL DEFAULT 1,
		maintenance INTEGER NOT NULL DEFAULT 0,
		enabled INTEGER NOT NULL DEFAULT 1,
		status TEXT NOT NULL DEFAULT 'unknown',
		last_latency_ms INTEGER NOT NULL DEFAULT 0,
		last_checked_at DATETIME,
		last_error TEXT NOT NULL DEFAULT '',
		ssl_expires_at DATETIME,
		ssl_days_remaining INTEGER,
		created_at DATETIME NOT NULL,
		updated_at DATETIME NOT NULL
	);

	CREATE TABLE IF NOT EXISTS check_results (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		monitor_id TEXT NOT NULL,
		status TEXT NOT NULL,
		latency_ms INTEGER NOT NULL,
		status_code INTEGER NOT NULL DEFAULT 0,
		error TEXT NOT NULL DEFAULT '',
		ssl_expires_at DATETIME,
		ssl_days_remaining INTEGER,
		checked_at DATETIME NOT NULL,
		FOREIGN KEY (monitor_id) REFERENCES monitors(id) ON DELETE CASCADE
	);
	CREATE INDEX IF NOT EXISTS idx_check_results_monitor_time ON check_results(monitor_id, checked_at DESC);

	CREATE TABLE IF NOT EXISTS incidents (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		monitor_id TEXT NOT NULL,
		monitor_name TEXT NOT NULL,
		cause TEXT NOT NULL,
		started_at DATETIME NOT NULL,
		resolved_at DATETIME,
		duration_seconds INTEGER NOT NULL DEFAULT 0,
		FOREIGN KEY (monitor_id) REFERENCES monitors(id) ON DELETE CASCADE
	);
	CREATE INDEX IF NOT EXISTS idx_incidents_monitor_time ON incidents(monitor_id, started_at DESC);

	CREATE TABLE IF NOT EXISTS webhooks (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		type TEXT NOT NULL,
		url TEXT NOT NULL DEFAULT '',
		bot_token TEXT NOT NULL DEFAULT '',
		chat_id TEXT NOT NULL DEFAULT '',
		enabled INTEGER NOT NULL DEFAULT 1,
		created_at DATETIME NOT NULL
	);
	`
	_, err := s.db.Exec(schema)
	if err != nil {
		return err
	}
	// Idempotent column migrations for existing databases
	_, _ = s.db.Exec(`ALTER TABLE monitors ADD COLUMN max_retries INTEGER NOT NULL DEFAULT 1`)
	_, _ = s.db.Exec(`ALTER TABLE monitors ADD COLUMN maintenance INTEGER NOT NULL DEFAULT 0`)
	return nil
}

func (s *Store) CreateMonitor(m *models.Monitor) error {
	if m.ID == "" {
		m.ID = uuid.NewString()
	}
	now := time.Now().UTC()
	m.CreatedAt = now
	m.UpdatedAt = now
	if m.Status == "" {
		m.Status = models.StatusUnknown
	}
	if m.IntervalSeconds < 5 {
		m.IntervalSeconds = 30
	}
	if m.TimeoutSeconds < 1 {
		m.TimeoutSeconds = 5
	}
	if m.SSLExpiryWarnDays <= 0 {
		m.SSLExpiryWarnDays = 14
	}
	if m.MaxRetries < 1 {
		m.MaxRetries = 1
	}

	_, err := s.db.Exec(`
		INSERT INTO monitors (
			id, name, type, target, port, interval_seconds, timeout_seconds,
			expected_status, keyword, ssl_expiry_warn_days, max_retries, maintenance,
			enabled, status, last_latency_ms, last_error, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		m.ID, m.Name, m.Type, m.Target, m.Port, m.IntervalSeconds, m.TimeoutSeconds,
		m.ExpectedStatus, m.Keyword, m.SSLExpiryWarnDays, m.MaxRetries, boolToInt(m.Maintenance),
		boolToInt(m.Enabled), m.Status, m.LastLatencyMs, m.LastError, m.CreatedAt, m.UpdatedAt,
	)
	return err
}

func (s *Store) UpdateMonitor(m *models.Monitor) error {
	m.UpdatedAt = time.Now().UTC()
	if m.IntervalSeconds < 5 {
		m.IntervalSeconds = 30
	}
	if m.TimeoutSeconds < 1 {
		m.TimeoutSeconds = 5
	}
	if m.SSLExpiryWarnDays <= 0 {
		m.SSLExpiryWarnDays = 14
	}
	if m.MaxRetries < 1 {
		m.MaxRetries = 1
	}

	res, err := s.db.Exec(`
		UPDATE monitors SET
			name = ?, type = ?, target = ?, port = ?, interval_seconds = ?,
			timeout_seconds = ?, expected_status = ?, keyword = ?,
			ssl_expiry_warn_days = ?, max_retries = ?, maintenance = ?,
			enabled = ?, updated_at = ?
		WHERE id = ?`,
		m.Name, m.Type, m.Target, m.Port, m.IntervalSeconds,
		m.TimeoutSeconds, m.ExpectedStatus, m.Keyword,
		m.SSLExpiryWarnDays, m.MaxRetries, boolToInt(m.Maintenance),
		boolToInt(m.Enabled), m.UpdatedAt, m.ID,
	)
	if err != nil {
		return err
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *Store) DeleteMonitor(id string) error {
	res, err := s.db.Exec(`DELETE FROM monitors WHERE id = ?`, id)
	if err != nil {
		return err
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *Store) GetMonitor(id string) (*models.Monitor, error) {
	row := s.db.QueryRow(`
		SELECT id, name, type, target, port, interval_seconds, timeout_seconds,
		       expected_status, keyword, ssl_expiry_warn_days, max_retries, maintenance,
		       enabled, status, last_latency_ms, last_checked_at, last_error,
		       ssl_expires_at, ssl_days_remaining, created_at, updated_at
		FROM monitors WHERE id = ?`, id)

	m, err := scanMonitor(row)
	if err != nil {
		return nil, err
	}
	s.enrichMonitorStats(m)
	return m, nil
}

func (s *Store) ListMonitors() ([]models.Monitor, error) {
	rows, err := s.db.Query(`
		SELECT id, name, type, target, port, interval_seconds, timeout_seconds,
		       expected_status, keyword, ssl_expiry_warn_days, max_retries, maintenance,
		       enabled, status, last_latency_ms, last_checked_at, last_error,
		       ssl_expires_at, ssl_days_remaining, created_at, updated_at
		FROM monitors ORDER BY created_at ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []models.Monitor
	for rows.Next() {
		m, err := scanMonitor(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, *m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	for i := range list {
		s.enrichMonitorStats(&list[i])
	}
	if list == nil {
		list = []models.Monitor{}
	}
	return list, nil
}

func (s *Store) RecordCheck(res *models.CheckResult) error {
	r, err := s.db.Exec(`
		INSERT INTO check_results (
			monitor_id, status, latency_ms, status_code, error,
			ssl_expires_at, ssl_days_remaining, checked_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		res.MonitorID, res.Status, res.LatencyMs, res.StatusCode, res.Error,
		res.SSLExpiresAt, res.SSLDaysRemaining, res.CheckedAt,
	)
	if err != nil {
		return err
	}
	res.ID, _ = r.LastInsertId()

	_, err = s.db.Exec(`
		UPDATE monitors SET
			status = ?, last_latency_ms = ?, last_checked_at = ?, last_error = ?,
			ssl_expires_at = COALESCE(?, ssl_expires_at),
			ssl_days_remaining = COALESCE(?, ssl_days_remaining),
			updated_at = ?
		WHERE id = ?`,
		res.Status, res.LatencyMs, res.CheckedAt, res.Error,
		res.SSLExpiresAt, res.SSLDaysRemaining, time.Now().UTC(), res.MonitorID,
	)
	if err != nil {
		return err
	}

	// Prune old check_results beyond 2000 rows per monitor to keep SQLite compact
	_, _ = s.db.Exec(`
		DELETE FROM check_results
		WHERE monitor_id = ? AND id NOT IN (
			SELECT id FROM check_results WHERE monitor_id = ? ORDER BY id DESC LIMIT 2000
		)`, res.MonitorID, res.MonitorID)

	return nil
}

func (s *Store) ListChecks(monitorID string, limit int) ([]models.CheckResult, error) {
	if limit <= 0 || limit > 500 {
		limit = 60
	}
	rows, err := s.db.Query(`
		SELECT id, monitor_id, status, latency_ms, status_code, error,
		       ssl_expires_at, ssl_days_remaining, checked_at
		FROM check_results
		WHERE monitor_id = ?
		ORDER BY id DESC
		LIMIT ?`, monitorID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var checks []models.CheckResult
	for rows.Next() {
		var c models.CheckResult
		var sslExp sql.NullTime
		var sslDays sql.NullInt64
		if err := rows.Scan(
			&c.ID, &c.MonitorID, &c.Status, &c.LatencyMs, &c.StatusCode,
			&c.Error, &sslExp, &sslDays, &c.CheckedAt,
		); err != nil {
			return nil, err
		}
		if sslExp.Valid {
			t := sslExp.Time
			c.SSLExpiresAt = &t
		}
		if sslDays.Valid {
			d := int(sslDays.Int64)
			c.SSLDaysRemaining = &d
		}
		checks = append(checks, c)
	}
	if checks == nil {
		checks = []models.CheckResult{}
	}
	return checks, rows.Err()
}

func (s *Store) OpenIncident(monitorID, monitorName, cause string, startedAt time.Time) (*models.Incident, error) {
	// Avoid opening duplicate open incident for same monitor
	var existingID int64
	err := s.db.QueryRow(`SELECT id FROM incidents WHERE monitor_id = ? AND resolved_at IS NULL LIMIT 1`, monitorID).Scan(&existingID)
	if err == nil {
		return nil, nil
	}

	r, err := s.db.Exec(`
		INSERT INTO incidents (monitor_id, monitor_name, cause, started_at)
		VALUES (?, ?, ?, ?)`,
		monitorID, monitorName, cause, startedAt,
	)
	if err != nil {
		return nil, err
	}
	id, _ := r.LastInsertId()
	return &models.Incident{
		ID:          id,
		MonitorID:   monitorID,
		MonitorName: monitorName,
		Cause:       cause,
		StartedAt:   startedAt,
	}, nil
}

func (s *Store) ResolveOpenIncidents(monitorID string, resolvedAt time.Time) (*models.Incident, error) {
	row := s.db.QueryRow(`
		SELECT id, monitor_id, monitor_name, cause, started_at
		FROM incidents
		WHERE monitor_id = ? AND resolved_at IS NULL
		ORDER BY id DESC LIMIT 1`, monitorID)

	var inc models.Incident
	if err := row.Scan(&inc.ID, &inc.MonitorID, &inc.MonitorName, &inc.Cause, &inc.StartedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	dur := int64(resolvedAt.Sub(inc.StartedAt).Seconds())
	if dur < 0 {
		dur = 0
	}
	inc.ResolvedAt = &resolvedAt
	inc.DurationSeconds = dur

	_, err := s.db.Exec(`
		UPDATE incidents SET resolved_at = ?, duration_seconds = ?
		WHERE monitor_id = ? AND resolved_at IS NULL`,
		resolvedAt, dur, monitorID,
	)
	if err != nil {
		return nil, err
	}
	return &inc, nil
}

func (s *Store) ListIncidents(monitorID string, limit int) ([]models.Incident, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}

	var (
		rows *sql.Rows
		err  error
	)
	if monitorID != "" {
		rows, err = s.db.Query(`
			SELECT id, monitor_id, monitor_name, cause, started_at, resolved_at, duration_seconds
			FROM incidents WHERE monitor_id = ?
			ORDER BY id DESC LIMIT ?`, monitorID, limit)
	} else {
		rows, err = s.db.Query(`
			SELECT id, monitor_id, monitor_name, cause, started_at, resolved_at, duration_seconds
			FROM incidents
			ORDER BY id DESC LIMIT ?`, limit)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []models.Incident
	for rows.Next() {
		var inc models.Incident
		var resolved sql.NullTime
		if err := rows.Scan(
			&inc.ID, &inc.MonitorID, &inc.MonitorName, &inc.Cause,
			&inc.StartedAt, &resolved, &inc.DurationSeconds,
		); err != nil {
			return nil, err
		}
		if resolved.Valid {
			t := resolved.Time
			inc.ResolvedAt = &t
		}
		list = append(list, inc)
	}
	if list == nil {
		list = []models.Incident{}
	}
	return list, rows.Err()
}

func (s *Store) CreateWebhook(w *models.Webhook) error {
	if w.ID == "" {
		w.ID = uuid.NewString()
	}
	w.CreatedAt = time.Now().UTC()
	_, err := s.db.Exec(`
		INSERT INTO webhooks (id, name, type, url, bot_token, chat_id, enabled, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		w.ID, w.Name, w.Type, w.URL, w.BotToken, w.ChatID, boolToInt(w.Enabled), w.CreatedAt,
	)
	return err
}

func (s *Store) ListWebhooks() ([]models.Webhook, error) {
	rows, err := s.db.Query(`
		SELECT id, name, type, url, bot_token, chat_id, enabled, created_at
		FROM webhooks ORDER BY created_at ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []models.Webhook
	for rows.Next() {
		var w models.Webhook
		var enabled int
		if err := rows.Scan(&w.ID, &w.Name, &w.Type, &w.URL, &w.BotToken, &w.ChatID, &enabled, &w.CreatedAt); err != nil {
			return nil, err
		}
		w.Enabled = enabled == 1
		list = append(list, w)
	}
	if list == nil {
		list = []models.Webhook{}
	}
	return list, rows.Err()
}

func (s *Store) GetWebhook(id string) (*models.Webhook, error) {
	var w models.Webhook
	var enabled int
	err := s.db.QueryRow(`
		SELECT id, name, type, url, bot_token, chat_id, enabled, created_at
		FROM webhooks WHERE id = ?`, id).
		Scan(&w.ID, &w.Name, &w.Type, &w.URL, &w.BotToken, &w.ChatID, &enabled, &w.CreatedAt)
	if err != nil {
		return nil, err
	}
	w.Enabled = enabled == 1
	return &w, nil
}

func (s *Store) DeleteWebhook(id string) error {
	res, err := s.db.Exec(`DELETE FROM webhooks WHERE id = ?`, id)
	if err != nil {
		return err
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *Store) enrichMonitorStats(m *models.Monitor) {
	since := time.Now().UTC().Add(-24 * time.Hour)
	var total, upCount int
	var avgLat sql.NullFloat64

	err := s.db.QueryRow(`
		SELECT
			COUNT(*),
			COALESCE(SUM(CASE WHEN status IN ('up', 'degraded') THEN 1 ELSE 0 END), 0),
			AVG(CASE WHEN latency_ms > 0 THEN latency_ms ELSE NULL END)
		FROM check_results
		WHERE monitor_id = ? AND checked_at >= ?`,
		m.ID, since,
	).Scan(&total, &upCount, &avgLat)
	if err == nil && total > 0 {
		m.Uptime24h = float64(upCount) * 100.0 / float64(total)
		if avgLat.Valid {
			m.AvgLatency24h = avgLat.Float64
		}
	} else {
		m.Uptime24h = 100.0
	}

	recent, err := s.ListChecks(m.ID, 30)
	if err == nil {
		// Reverse so oldest -> newest for heartbeat bar
		for i, j := 0, len(recent)-1; i < j; i, j = i+1, j-1 {
			recent[i], recent[j] = recent[j], recent[i]
		}
		m.RecentChecks = recent
	}
}

type scanner interface {
	Scan(dest ...any) error
}

func scanMonitor(row scanner) (*models.Monitor, error) {
	var m models.Monitor
	var enabled, maintenance int
	var lastChecked, sslExpires sql.NullTime
	var sslDays sql.NullInt64

	err := row.Scan(
		&m.ID, &m.Name, &m.Type, &m.Target, &m.Port,
		&m.IntervalSeconds, &m.TimeoutSeconds, &m.ExpectedStatus,
		&m.Keyword, &m.SSLExpiryWarnDays, &m.MaxRetries, &maintenance,
		&enabled, &m.Status, &m.LastLatencyMs, &lastChecked, &m.LastError,
		&sslExpires, &sslDays, &m.CreatedAt, &m.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	m.Enabled = enabled == 1
	m.Maintenance = maintenance == 1
	if lastChecked.Valid {
		t := lastChecked.Time
		m.LastCheckedAt = &t
	}
	if sslExpires.Valid {
		t := sslExpires.Time
		m.SSLExpiresAt = &t
	}
	if sslDays.Valid {
		d := int(sslDays.Int64)
		m.SSLDaysRemaining = &d
	}
	return &m, nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func (s *Store) SeedDefaultsIfEmpty() error {
	var count int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM monitors`).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	defaults := []models.Monitor{
		{
			Name:              "Cloudflare DNS (ICMP)",
			Type:              models.MonitorICMP,
			Target:            "1.1.1.1",
			IntervalSeconds:   30,
			TimeoutSeconds:    5,
			SSLExpiryWarnDays: 14,
			Enabled:           true,
		},
		{
			Name:              "Google HTTPS",
			Type:              models.MonitorHTTP,
			Target:            "https://www.google.com",
			IntervalSeconds:   30,
			TimeoutSeconds:    5,
			ExpectedStatus:    200,
			SSLExpiryWarnDays: 14,
			Enabled:           true,
		},
		{
			Name:              "GitHub SSL Certificate",
			Type:              models.MonitorSSL,
			Target:            "github.com",
			Port:              443,
			IntervalSeconds:   60,
			TimeoutSeconds:    5,
			SSLExpiryWarnDays: 14,
			Enabled:           true,
		},
		{
			Name:            "Cloudflare DNS TCP:53",
			Type:            models.MonitorTCP,
			Target:          "1.1.1.1",
			Port:            53,
			IntervalSeconds: 30,
			TimeoutSeconds:  5,
			Enabled:         true,
		},
	}

	for i := range defaults {
		if err := s.CreateMonitor(&defaults[i]); err != nil {
			return fmt.Errorf("seed monitor %s: %w", defaults[i].Name, err)
		}
	}
	return nil
}
