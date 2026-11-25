import { CheckResult, Incident, Monitor, Webhook } from "./types";

const API_BASE =
  process.env.NEXT_PUBLIC_API_URL?.replace(/\/$/, "") || "http://localhost:8080";

export function getWebSocketURL(): string {
  if (process.env.NEXT_PUBLIC_WS_URL) {
    return process.env.NEXT_PUBLIC_WS_URL;
  }
  const url = new URL(API_BASE);
  url.protocol = url.protocol === "https:" ? "wss:" : "ws:";
  url.pathname = "/api/ws";
  return url.toString();
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`${API_BASE}${path}`, {
    ...init,
    headers: {
      "Content-Type": "application/json",
      ...(init?.headers || {}),
    },
  });
  if (!res.ok) {
    const errBody = await res.json().catch(() => ({}));
    throw new Error(errBody.error || `HTTP ${res.status}`);
  }
  if (res.status === 204) {
    return undefined as T;
  }
  return res.json();
}

export const api = {
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
};
