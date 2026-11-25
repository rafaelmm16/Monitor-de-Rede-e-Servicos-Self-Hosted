import {
  CheckResult,
  ConfigBackup,
  Incident,
  Monitor,
  PublicStatusResponse,
  Webhook,
} from "./types";

const API_BASE = process.env.NEXT_PUBLIC_API_URL?.replace(/\/$/, "") || "";
const AUTH_STORAGE_KEY = "netpulse_auth_token";

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
  authStatus: () => request<{ auth_required: boolean }>("/api/auth/status"),
  publicStatus: () => request<PublicStatusResponse>("/api/status"),
  listMonitors: () => request<Monitor[]>("/api/monitors"),
  createMonitor: (data: Partial<Monitor>) =>
    request<Monitor>("/api/monitors", {
      method: "POST",
      body: JSON.stringify(data),
    }),
  updateMonitor: (id: string, data: Partial<Monitor>) =>
    request<Monitor>(`/api/monitors/${id}`, {
      method: "PUT",
      body: JSON.stringify(data),
    }),
  deleteMonitor: (id: string) =>
    request<void>(`/api/monitors/${id}`, { method: "DELETE" }),
  triggerCheck: (id: string) =>
    request<{ status: string }>(`/api/monitors/${id}/check`, {
      method: "POST",
    }),
  listChecks: (id: string, limit = 60) =>
    request<CheckResult[]>(`/api/monitors/${id}/checks?limit=${limit}`),
  listIncidents: (monitorId?: string, limit = 30) =>
    request<Incident[]>(
      monitorId
        ? `/api/monitors/${monitorId}/incidents?limit=${limit}`
        : `/api/incidents?limit=${limit}`
    ),
  listWebhooks: () => request<Webhook[]>("/api/webhooks"),
  createWebhook: (data: Partial<Webhook>) =>
    request<Webhook>("/api/webhooks", {
      method: "POST",
      body: JSON.stringify(data),
    }),
  deleteWebhook: (id: string) =>
    request<void>(`/api/webhooks/${id}`, { method: "DELETE" }),
  testWebhook: (id: string) =>
    request<{ status: string }>(`/api/webhooks/${id}/test`, {
      method: "POST",
    }),
  exportConfig: () => request<ConfigBackup>("/api/config/export"),
  importConfig: (data: ConfigBackup) =>
    request<{ imported_monitors: number; imported_webhooks: number }>(
      "/api/config/import",
      {
        method: "POST",
        body: JSON.stringify(data),
      }
    ),
};
