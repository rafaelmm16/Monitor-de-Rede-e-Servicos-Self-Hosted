import {
  INITIAL_DEMO_INCIDENTS,
  INITIAL_DEMO_MONITORS,
  INITIAL_DEMO_WEBHOOKS,
} from "./demoData";
import {
  CheckResult,
  ConfigBackup,
  Incident,
  Monitor,
  PublicStatusResponse,
  Webhook,
} from "./types";

export const IS_DEMO_MODE = process.env.NEXT_PUBLIC_DEMO_MODE === "true";

const API_BASE = process.env.NEXT_PUBLIC_API_URL?.replace(/\/$/, "") || "";
const AUTH_STORAGE_KEY = "netpulse_auth_token";

let demoMonitors: Monitor[] = structuredClone(INITIAL_DEMO_MONITORS);
let demoIncidents: Incident[] = structuredClone(INITIAL_DEMO_INCIDENTS);
let demoWebhooks: Webhook[] = structuredClone(INITIAL_DEMO_WEBHOOKS);

export function getAuthToken(): string {
  if (typeof window === "undefined") return "";
  return window.localStorage.getItem(AUTH_STORAGE_KEY) || "";
}

export function setAuthToken(token: string) {
  if (typeof window === "undefined") return;
  if (token) {
    window.localStorage.setItem(AUTH_STORAGE_KEY, token);
  } else {
    window.localStorage.removeItem(AUTH_STORAGE_KEY);
  }
}

export function getWebSocketURL(): string {
  if (process.env.NEXT_PUBLIC_WS_URL) {
    return process.env.NEXT_PUBLIC_WS_URL;
  }
  if (API_BASE) {
    const url = new URL(API_BASE);
    url.protocol = url.protocol === "https:" ? "wss:" : "ws:";
    url.pathname = "/api/ws";
    return url.toString();
  }
  if (typeof window !== "undefined") {
    const proto = window.location.protocol === "https:" ? "wss:" : "ws:";
    return `${proto}//${window.location.hostname}:8080/api/ws`;
  }
  return "ws://localhost:8080/api/ws";
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const token = getAuthToken();
  const res = await fetch(`${API_BASE}${path}`, {
    ...init,
    headers: {
      "Content-Type": "application/json",
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
      ...(init?.headers || {}),
    },
  });
  if (!res.ok) {
    const errBody = await res.json().catch(() => ({}));
    const err = new Error(errBody.error || `HTTP ${res.status}`) as Error & {
      status?: number;
    };
    err.status = res.status;
    throw err;
  }
  if (res.status === 204) {
    return undefined as T;
  }
  return res.json();
}

export const api = {
  authStatus: async () => {
    if (IS_DEMO_MODE) return { auth_required: false };
    return request<{ auth_required: boolean }>("/api/auth/status");
  },

  publicStatus: async (): Promise<PublicStatusResponse> => {
    if (IS_DEMO_MODE) {
      return {
        generated_at: new Date().toISOString(),
        monitors: demoMonitors,
        incidents: demoIncidents,
      };
    }
    return request<PublicStatusResponse>("/api/status");
  },

  listMonitors: async () => {
    if (IS_DEMO_MODE) return structuredClone(demoMonitors);
    return request<Monitor[]>("/api/monitors");
  },

  createMonitor: async (data: Partial<Monitor>) => {
    if (IS_DEMO_MODE) {
      const now = new Date().toISOString();
      const created: Monitor = {
        id: `demo-${Date.now()}`,
        name: data.name || "New Monitor",
        type: data.type || "http",
        target: data.target || "https://example.com",
        port: data.port || 0,
        interval_seconds: data.interval_seconds || 30,
        timeout_seconds: data.timeout_seconds || 5,
        expected_status: data.expected_status ?? 200,
        http_method: data.http_method || "GET",
        http_headers: data.http_headers || "",
        keyword: data.keyword || "",
        ssl_expiry_warn_days: data.ssl_expiry_warn_days || 14,
        max_retries: data.max_retries || 1,
        maintenance: Boolean(data.maintenance),
        enabled: data.enabled ?? true,
        status: "up",
        last_latency_ms: 24,
        last_checked_at: now,
        uptime_24h: 100,
        uptime_7d: 100,
        uptime_30d: 100,
        avg_latency_24h: 24,
        p95_latency_24h: 32,
        recent_checks: [
          {
            id: 1,
            monitor_id: "",
            status: "up",
            latency_ms: 24,
            status_code: 200,
            checked_at: now,
          },
        ],
        created_at: now,
        updated_at: now,
      };
      demoMonitors = [...demoMonitors, created];
      return created;
    }
    return request<Monitor>("/api/monitors", {
      method: "POST",
      body: JSON.stringify(data),
    });
  },

  updateMonitor: async (id: string, data: Partial<Monitor>) => {
    if (IS_DEMO_MODE) {
      demoMonitors = demoMonitors.map((m) =>
        m.id === id ? { ...m, ...data, updated_at: new Date().toISOString() } : m
      );
      return demoMonitors.find((m) => m.id === id)!;
    }
    return request<Monitor>(`/api/monitors/${id}`, {
      method: "PUT",
      body: JSON.stringify(data),
    });
  },

  deleteMonitor: async (id: string) => {
    if (IS_DEMO_MODE) {
      demoMonitors = demoMonitors.filter((m) => m.id !== id);
      return;
    }
    return request<void>(`/api/monitors/${id}`, { method: "DELETE" });
  },

  triggerCheck: async (id: string) => {
    if (IS_DEMO_MODE) {
      const m = demoMonitors.find((item) => item.id === id);
      if (m) {
        const nextLat = Math.max(5, Math.round(m.last_latency_ms + (Math.random() * 14 - 7)));
        const chk: CheckResult = {
          id: Date.now(),
          monitor_id: id,
          status: m.status === "unknown" ? "up" : m.status,
          latency_ms: nextLat,
          status_code: 200,
          checked_at: new Date().toISOString(),
        };
        m.last_latency_ms = nextLat;
        m.last_checked_at = chk.checked_at;
        m.recent_checks = [...(m.recent_checks || []), chk].slice(-30);
      }
      return { status: "triggered" };
    }
    return request<{ status: string }>(`/api/monitors/${id}/check`, {
      method: "POST",
    });
  },

  listChecks: async (id: string, limit = 60) => {
    if (IS_DEMO_MODE) {
      const m = demoMonitors.find((item) => item.id === id);
      return [...(m?.recent_checks || [])].reverse().slice(0, limit);
    }
    return request<CheckResult[]>(`/api/monitors/${id}/checks?limit=${limit}`);
  },

  listIncidents: async (monitorId?: string, limit = 30) => {
    if (IS_DEMO_MODE) {
      const filtered = monitorId
        ? demoIncidents.filter((i) => i.monitor_id === monitorId)
        : demoIncidents;
      return filtered.slice(0, limit);
    }
    return request<Incident[]>(
      monitorId
        ? `/api/monitors/${monitorId}/incidents?limit=${limit}`
        : `/api/incidents?limit=${limit}`
    );
  },

  listWebhooks: async () => {
    if (IS_DEMO_MODE) return structuredClone(demoWebhooks);
    return request<Webhook[]>("/api/webhooks");
  },

  createWebhook: async (data: Partial<Webhook>) => {
    if (IS_DEMO_MODE) {
      const created: Webhook = {
        id: `demo-wh-${Date.now()}`,
        name: data.name || "Webhook",
        type: data.type || "discord",
        url: data.url,
        bot_token: data.bot_token,
        chat_id: data.chat_id,
        enabled: data.enabled ?? true,
        created_at: new Date().toISOString(),
      };
      demoWebhooks = [...demoWebhooks, created];
      return created;
    }
    return request<Webhook>("/api/webhooks", {
      method: "POST",
      body: JSON.stringify(data),
    });
  },

  deleteWebhook: async (id: string) => {
    if (IS_DEMO_MODE) {
      demoWebhooks = demoWebhooks.filter((w) => w.id !== id);
      return;
    }
    return request<void>(`/api/webhooks/${id}`, { method: "DELETE" });
  },

  testWebhook: async (id: string) => {
    if (IS_DEMO_MODE) return { status: "sent (demo)" };
    return request<{ status: string }>(`/api/webhooks/${id}/test`, {
      method: "POST",
    });
  },

  exportConfig: async (): Promise<ConfigBackup> => {
    if (IS_DEMO_MODE) {
      return {
        version: "1.0",
        exported_at: new Date().toISOString(),
        monitors: demoMonitors,
        webhooks: demoWebhooks,
      };
    }
    return request<ConfigBackup>("/api/config/export");
  },

  importConfig: async (data: ConfigBackup) => {
    if (IS_DEMO_MODE) {
      return {
        imported_monitors: data.monitors?.length || 0,
        imported_webhooks: data.webhooks?.length || 0,
      };
    }
    return request<{ imported_monitors: number; imported_webhooks: number }>(
      "/api/config/import",
      {
        method: "POST",
        body: JSON.stringify(data),
      }
    );
  },
};
