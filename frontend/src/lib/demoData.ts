import { CheckResult, Incident, Monitor, Webhook } from "./types";

function makeChecks(monitorId: string, baseLatency: number, hasDip = false): CheckResult[] {
  const now = Date.now();
  return Array.from({ length: 30 }).map((_, i) => {
    const idxFromOldest = i;
    const isDown = hasDip && idxFromOldest === 18;
    const latency = isDown
      ? 0
      : Math.max(4, Math.round(baseLatency + Math.sin(i * 0.7) * (baseLatency * 0.25)));
    return {
      id: i + 1,
      monitor_id: monitorId,
      status: isDown ? "down" : "up",
      latency_ms: latency,
      status_code: isDown ? 503 : 200,
      error: isDown ? "HTTP 503 Service Unavailable" : "",
      checked_at: new Date(now - (29 - i) * 30_000).toISOString(),
    };
  });
}

const nowIso = new Date().toISOString();

export const INITIAL_DEMO_MONITORS: Monitor[] = [
  {
    id: "demo-http-1",
    name: "Production API Gateway",
    type: "http",
    target: "https://api.netpulse.local/health",
    port: 0,
    interval_seconds: 30,
    timeout_seconds: 5,
    expected_status: 200,
    http_method: "GET",
    ssl_expiry_warn_days: 14,
    max_retries: 2,
    maintenance: false,
    enabled: true,
    status: "up",
    last_latency_ms: 38,
    last_checked_at: nowIso,
    ssl_days_remaining: 64,
    uptime_24h: 99.8,
    uptime_7d: 99.9,
    uptime_30d: 99.95,
    avg_latency_24h: 41,
    p95_latency_24h: 58,
    recent_checks: makeChecks("demo-http-1", 38, false),
    created_at: nowIso,
    updated_at: nowIso,
  },
  {
    id: "demo-icmp-1",
    name: "Cloudflare DNS (ICMP)",
    type: "icmp",
    target: "1.1.1.1",
    port: 0,
    interval_seconds: 30,
    timeout_seconds: 5,
    expected_status: 0,
    ssl_expiry_warn_days: 14,
    max_retries: 2,
    maintenance: false,
    enabled: true,
    status: "up",
    last_latency_ms: 11,
    last_checked_at: nowIso,
    uptime_24h: 100,
    uptime_7d: 100,
    uptime_30d: 99.99,
    avg_latency_24h: 12,
    p95_latency_24h: 17,
    recent_checks: makeChecks("demo-icmp-1", 11, false),
    created_at: nowIso,
    updated_at: nowIso,
  },
  {
    id: "demo-tcp-1",
    name: "PostgreSQL Primary (TCP)",
    type: "tcp",
    target: "db.internal",
    port: 5432,
    interval_seconds: 30,
    timeout_seconds: 5,
    expected_status: 0,
    ssl_expiry_warn_days: 14,
    max_retries: 3,
    maintenance: false,
    enabled: true,
    status: "up",
    last_latency_ms: 6,
    last_checked_at: nowIso,
    uptime_24h: 99.4,
    uptime_7d: 99.7,
    uptime_30d: 99.8,
    avg_latency_24h: 7,
    p95_latency_24h: 12,
    recent_checks: makeChecks("demo-tcp-1", 6, true),
    created_at: nowIso,
    updated_at: nowIso,
  },
  {
    id: "demo-ssl-1",
    name: "Auth Portal SSL",
    type: "ssl",
    target: "auth.example.com",
    port: 443,
    interval_seconds: 60,
    timeout_seconds: 5,
    expected_status: 0,
    ssl_expiry_warn_days: 14,
    max_retries: 1,
    maintenance: false,
    enabled: true,
    status: "degraded",
    last_latency_ms: 52,
    last_checked_at: nowIso,
    last_error: "SSL certificate expires in 9 days",
    ssl_days_remaining: 9,
    uptime_24h: 100,
    uptime_7d: 100,
    uptime_30d: 100,
    avg_latency_24h: 49,
    p95_latency_24h: 64,
    recent_checks: makeChecks("demo-ssl-1", 50, false).map((c, idx) =>
      idx === 29 ? { ...c, status: "degraded", error: "SSL certificate expires in 9 days" } : c
    ),
    created_at: nowIso,
    updated_at: nowIso,
  },
];

export const INITIAL_DEMO_INCIDENTS: Incident[] = [
  {
    id: 1,
    monitor_id: "demo-tcp-1",
    monitor_name: "PostgreSQL Primary (TCP)",
    cause: "dial tcp db.internal:5432: i/o timeout",
    started_at: new Date(Date.now() - 12 * 60_000).toISOString(),
    resolved_at: new Date(Date.now() - 11 * 60_000).toISOString(),
    duration_seconds: 42,
  },
];

export const INITIAL_DEMO_WEBHOOKS: Webhook[] = [
  {
    id: "demo-wh-1",
    name: "Ops Discord",
    type: "discord",
    url: "https://discord.com/api/webhooks/demo",
    enabled: true,
    created_at: nowIso,
  },
];
