export type MonitorType = "http" | "icmp" | "tcp" | "ssl";
export type MonitorStatus = "unknown" | "up" | "down" | "degraded";
export type WebhookType = "discord" | "telegram" | "generic";

export interface CheckResult {
  id: number;
  monitor_id: string;
  status: MonitorStatus;
  latency_ms: number;
  status_code?: number;
  error?: string;
  ssl_expires_at?: string;
  ssl_days_remaining?: number;
  checked_at: string;
}

export interface Monitor {
  id: string;
  name: string;
  type: MonitorType;
  target: string;
  port: number;
  interval_seconds: number;
  timeout_seconds: number;
  expected_status: number;
  keyword?: string;
  ssl_expiry_warn_days: number;
  max_retries: number;
  maintenance: boolean;
  enabled: boolean;
  status: MonitorStatus;
  last_latency_ms: number;
  last_checked_at?: string;
  last_error?: string;
  ssl_expires_at?: string;
  ssl_days_remaining?: number;
  uptime_24h: number;
  avg_latency_24h: number;
  recent_checks?: CheckResult[];
  created_at: string;
  updated_at: string;
}

export interface Incident {
  id: number;
  monitor_id: string;
  monitor_name: string;
  cause: string;
  started_at: string;
  resolved_at?: string;
  duration_seconds: number;
}

export interface Webhook {
  id: string;
  name: string;
  type: WebhookType;
  url?: string;
  bot_token?: string;
  chat_id?: string;
  enabled: boolean;
  created_at: string;
}

export interface WSMessage<T = unknown> {
  event: string;
  data: T;
}

export interface ConfigBackup {
  version: string;
  exported_at: string;
  monitors: Monitor[];
  webhooks: Webhook[];
}

export interface PublicStatusResponse {
  generated_at: string;
  monitors: Monitor[];
  incidents: Incident[];
}
