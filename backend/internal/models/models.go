package models

import "time"

type MonitorType string

const (
	MonitorHTTP MonitorType = "http"
	MonitorICMP MonitorType = "icmp"
	MonitorTCP  MonitorType = "tcp"
	MonitorSSL  MonitorType = "ssl"
)

type MonitorStatus string

const (
	StatusUnknown  MonitorStatus = "unknown"
	StatusUp       MonitorStatus = "up"
	StatusDown     MonitorStatus = "down"
	StatusDegraded MonitorStatus = "degraded"
)

type Monitor struct {
	ID                string        `json:"id"`
	Name              string        `json:"name"`
	Type              MonitorType   `json:"type"`
	Target            string        `json:"target"`
	Port              int           `json:"port"`
	IntervalSeconds   int           `json:"interval_seconds"`
	TimeoutSeconds    int           `json:"timeout_seconds"`
	ExpectedStatus    int           `json:"expected_status"`
	Keyword           string        `json:"keyword,omitempty"`
	SSLExpiryWarnDays int           `json:"ssl_expiry_warn_days"`
	Enabled           bool          `json:"enabled"`
	Status            MonitorStatus `json:"status"`
	LastLatencyMs     int64         `json:"last_latency_ms"`
	LastCheckedAt     *time.Time    `json:"last_checked_at,omitempty"`
	LastError         string        `json:"last_error,omitempty"`
	SSLExpiresAt      *time.Time    `json:"ssl_expires_at,omitempty"`
	SSLDaysRemaining  *int          `json:"ssl_days_remaining,omitempty"`
	Uptime24h         float64       `json:"uptime_24h"`
	AvgLatency24h     float64       `json:"avg_latency_24h"`
	RecentChecks      []CheckResult `json:"recent_checks,omitempty"`
	CreatedAt         time.Time     `json:"created_at"`
	UpdatedAt         time.Time     `json:"updated_at"`
}

type CheckResult struct {
	ID               int64         `json:"id"`
	MonitorID        string        `json:"monitor_id"`
	Status           MonitorStatus `json:"status"`
	LatencyMs        int64         `json:"latency_ms"`
	StatusCode       int           `json:"status_code,omitempty"`
	Error            string        `json:"error,omitempty"`
	SSLExpiresAt     *time.Time    `json:"ssl_expires_at,omitempty"`
	SSLDaysRemaining *int          `json:"ssl_days_remaining,omitempty"`
	CheckedAt        time.Time     `json:"checked_at"`
}

type Incident struct {
	ID              int64      `json:"id"`
	MonitorID       string     `json:"monitor_id"`
	MonitorName     string     `json:"monitor_name"`
	Cause           string     `json:"cause"`
	StartedAt       time.Time  `json:"started_at"`
	ResolvedAt      *time.Time `json:"resolved_at,omitempty"`
	DurationSeconds int64      `json:"duration_seconds"`
}

type WebhookType string

const (
	WebhookDiscord  WebhookType = "discord"
	WebhookTelegram WebhookType = "telegram"
	WebhookGeneric  WebhookType = "generic"
)

type Webhook struct {
	ID        string      `json:"id"`
	Name      string      `json:"name"`
	Type      WebhookType `json:"type"`
	URL       string      `json:"url,omitempty"`
	BotToken  string      `json:"bot_token,omitempty"`
	ChatID    string      `json:"chat_id,omitempty"`
	Enabled   bool        `json:"enabled"`
	CreatedAt time.Time   `json:"created_at"`
}

type WSMessage struct {
	Event string `json:"event"`
	Data  any    `json:"data"`
}
