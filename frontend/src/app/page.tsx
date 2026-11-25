"use client";

import React, { useCallback, useEffect, useMemo, useRef, useState } from "react";
import Link from "next/link";
import {
  Activity,
  AlertTriangle,
  Archive,
  Bell,
  CheckCircle2,
  Clock,
  Edit3,
  ExternalLink,
  Globe,
  KeyRound,
  Pause,
  Play,
  Plus,
  RefreshCw,
  Search,
  Server,
  ShieldCheck,
  Trash2,
  Wifi,
  WifiOff,
  X,
  XCircle,
} from "lucide-react";
import { api, getWebSocketURL } from "@/lib/api";
import {
  CheckResult,
  Incident,
  Monitor,
  MonitorStatus,
  Webhook,
  WSMessage,
} from "@/lib/types";
import { LatencyChart } from "@/components/LatencyChart";
import { MonitorModal } from "@/components/MonitorModal";
import { WebhookModal } from "@/components/WebhookModal";
import { AuthModal, BackupModal } from "@/components/BackupAuthModals";

interface ToastItem {
  id: string;
  title: string;
  message: string;
  variant: "ok" | "err" | "warn";
}

function statusColor(status: MonitorStatus) {
  switch (status) {
    case "up":
      return "text-emerald-400 bg-emerald-500/15 border-emerald-500/30";
    case "down":
      return "text-rose-400 bg-rose-500/15 border-rose-500/30";
    case "degraded":
      return "text-amber-400 bg-amber-500/15 border-amber-500/30";
    default:
      return "text-slate-400 bg-slate-500/15 border-slate-500/30";
  }
}

function statusDot(status: MonitorStatus) {
  switch (status) {
    case "up":
      return "bg-emerald-400 shadow-[0_0_8px_rgba(52,211,153,0.8)]";
    case "down":
      return "bg-rose-500 shadow-[0_0_8px_rgba(244,63,94,0.8)]";
    case "degraded":
      return "bg-amber-400 shadow-[0_0_8px_rgba(251,191,36,0.8)]";
    default:
      return "bg-slate-500";
  }
}

function formatDuration(sec: number) {
  if (sec < 60) return `${sec}s`;
  const m = Math.floor(sec / 60);
  const s = sec % 60;
  if (m < 60) return `${m}m ${s}s`;
  const h = Math.floor(m / 60);
  return `${h}h ${m % 60}m`;
}

export default function DashboardPage() {
  const [monitors, setMonitors] = useState<Monitor[]>([]);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const selectedIdRef = useRef<string | null>(null);
  const [checks, setChecks] = useState<CheckResult[]>([]);
  const [incidents, setIncidents] = useState<Incident[]>([]);
  const [webhooks, setWebhooks] = useState<Webhook[]>([]);
  const [wsConnected, setWsConnected] = useState(false);
  const [loading, setLoading] = useState(true);
  const [apiError, setApiError] = useState<string | null>(null);

  const [filter, setFilter] = useState<"all" | MonitorStatus>("all");
  const [search, setSearch] = useState("");
  const [detailTab, setDetailTab] = useState<"checks" | "incidents">("checks");

  const [monitorModalOpen, setMonitorModalOpen] = useState(false);
  const [editingMonitor, setEditingMonitor] = useState<Monitor | null>(null);
  const [webhookModalOpen, setWebhookModalOpen] = useState(false);
  const [backupModalOpen, setBackupModalOpen] = useState(false);
  const [authModalOpen, setAuthModalOpen] = useState(false);
  const [authRequired, setAuthRequired] = useState(false);
  const [toasts, setToasts] = useState<ToastItem[]>([]);

  const pushToast = useCallback(
    (title: string, message: string, variant: "ok" | "err" | "warn" = "warn") => {
      const id = `${Date.now()}-${Math.random().toString(36).slice(2, 7)}`;
      setToasts((prev) => [...prev.slice(-3), { id, title, message, variant }]);
      setTimeout(() => {
        setToasts((prev) => prev.filter((t) => t.id !== id));
      }, 6000);
    },
    []
  );

  const loadInitial = useCallback(async () => {
    try {
      const [authRes, mList, incList] = await Promise.all([
        api.authStatus().catch(() => ({ auth_required: false })),
        api.listMonitors(),
        api.listIncidents(undefined, 30),
      ]);
      setAuthRequired(authRes.auth_required);
      setMonitors(mList);
      setIncidents(incList);
      setSelectedId((prev) => prev || (mList[0]?.id ?? null));
      api.listWebhooks().then(setWebhooks).catch(() => {});
      setApiError(null);
    } catch (err) {
      setApiError(
        err instanceof Error
          ? err.message
          : "Unable to reach the NetPulse backend API."
      );
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    loadInitial();
  }, [loadInitial]);

  // Auto-retry loading if backend was offline at startup and comes back online
  useEffect(() => {
    if (!apiError) return;
    if (wsConnected) {
      loadInitial();
      return;
    }
    const timer = setInterval(loadInitial, 5000);
    return () => clearInterval(timer);
  }, [apiError, wsConnected, loadInitial]);

  useEffect(() => {
    selectedIdRef.current = selectedId;
    if (!selectedId) {
      setChecks([]);
      return;
    }
    api
      .listChecks(selectedId, 60)
      .then(setChecks)
      .catch(() => {});
  }, [selectedId]);

  // Real-time WebSocket subscription
  useEffect(() => {
    let ws: WebSocket | null = null;
    let reconnectTimer: ReturnType<typeof setTimeout>;

    const connect = () => {
      try {
        ws = new WebSocket(getWebSocketURL());
        ws.onopen = () => setWsConnected(true);
        ws.onclose = () => {
          setWsConnected(false);
          reconnectTimer = setTimeout(connect, 3000);
        };
        ws.onerror = () => {
          ws?.close();
        };
        ws.onmessage = (ev) => {
          try {
            const msg: WSMessage = JSON.parse(ev.data);
            if (msg.event === "check_result") {
              const payload = msg.data as {
                monitor: Monitor;
                check: CheckResult;
              };
              setMonitors((prev) =>
                prev.map((m) =>
                  m.id === payload.monitor.id ? payload.monitor : m
                )
              );
              if (selectedIdRef.current === payload.monitor.id) {
                setChecks((prev) =>
                  prev.some((c) => c.id === payload.check.id)
                    ? prev
                    : [payload.check, ...prev].slice(0, 60)
                );
              }
            } else if (
              msg.event === "monitor_created" ||
              msg.event === "monitor_updated"
            ) {
              const updated = msg.data as Monitor;
              setMonitors((prev) => {
                const exists = prev.some((m) => m.id === updated.id);
                if (exists) {
                  return prev.map((m) => (m.id === updated.id ? updated : m));
                }
                return [...prev, updated];
              });
            } else if (msg.event === "monitor_deleted") {
              const { id } = msg.data as { id: string };
              setMonitors((prev) => prev.filter((m) => m.id !== id));
              setSelectedId((curr) => (curr === id ? null : curr));
            } else if (msg.event === "incident_opened") {
              const inc = msg.data as Incident;
              pushToast(
                `${inc.monitor_name} is DOWN`,
                inc.cause || "Health check failed",
                "err"
              );
              api.listIncidents(undefined, 30).then(setIncidents).catch(() => {});
            } else if (msg.event === "incident_resolved") {
              const inc = msg.data as Incident;
              pushToast(
                `${inc.monitor_name} Recovered`,
                `Downtime duration: ${formatDuration(inc.duration_seconds)}`,
                "ok"
              );
              api.listIncidents(undefined, 30).then(setIncidents).catch(() => {});
            }
          } catch {
            // Ignore malformed WS message
          }
        };
      } catch {
        reconnectTimer = setTimeout(connect, 3000);
      }
    };

    connect();
    return () => {
      clearTimeout(reconnectTimer);
      ws?.close();
    };
  }, []);

  const selectedMonitor = useMemo(
    () => monitors.find((m) => m.id === selectedId) ?? monitors[0] ?? null,
    [monitors, selectedId]
  );

  const filteredMonitors = useMemo(() => {
    return monitors.filter((m) => {
      if (filter !== "all" && m.status !== filter) return false;
      if (
        search.trim() &&
        !m.name.toLowerCase().includes(search.toLowerCase()) &&
        !m.target.toLowerCase().includes(search.toLowerCase())
      ) {
        return false;
      }
      return true;
    });
  }, [monitors, filter, search]);

  const stats = useMemo(() => {
    const total = monitors.length;
    const up = monitors.filter((m) => m.status === "up").length;
    const down = monitors.filter((m) => m.status === "down").length;
    const degraded = monitors.filter((m) => m.status === "degraded").length;
    const activeLatencies = monitors
      .filter((m) => m.last_latency_ms > 0)
      .map((m) => m.last_latency_ms);
    const avgLatency =
      activeLatencies.length > 0
        ? Math.round(
            activeLatencies.reduce((a, b) => a + b, 0) / activeLatencies.length
          )
        : 0;
    return { total, up, down, degraded, avgLatency };
  }, [monitors]);

  const handleSaveMonitor = async (
    data: Partial<Monitor>,
    existingId?: string
  ) => {
    if (existingId) {
      await api.updateMonitor(existingId, data);
    } else {
      const created = await api.createMonitor(data);
      setSelectedId(created.id);
    }
    const fresh = await api.listMonitors();
    setMonitors(fresh);
  };

  const handleToggleEnabled = async (m: Monitor) => {
    await api.updateMonitor(m.id, { ...m, enabled: !m.enabled });
    const fresh = await api.listMonitors();
    setMonitors(fresh);
  };

  const handleDeleteMonitor = async (id: string) => {
    await api.deleteMonitor(id);
    const fresh = await api.listMonitors();
    setMonitors(fresh);
    if (selectedId === id) {
      setSelectedId(fresh[0]?.id ?? null);
    }
  };

  const monitorIncidents = useMemo(() => {
    if (!selectedMonitor) return incidents;
    return incidents.filter((inc) => inc.monitor_id === selectedMonitor.id);
  }, [incidents, selectedMonitor]);

  return (
    <div className="min-h-screen bg-slate-950 text-slate-100">
      {/* Top Navbar */}
      <header className="sticky top-0 z-30 border-b border-slate-800/80 bg-slate-950/90 backdrop-blur-md">
        <div className="mx-auto flex max-w-7xl items-center justify-between px-4 py-3.5 sm:px-6">
          <div className="flex items-center gap-3">
            <div className="flex h-9 w-9 items-center justify-center rounded-xl bg-emerald-500/15 ring-1 ring-emerald-500/30">
              <Activity className="h-5 w-5 text-emerald-400" />
            </div>
            <div>
              <h1 className="text-base font-bold tracking-tight text-slate-100 sm:text-lg">
                NetPulse
              </h1>
              <p className="text-xs text-slate-400">
                Self-Hosted Network & Service Monitor
              </p>
            </div>
          </div>

          <div className="flex items-center gap-2.5">
            <span
              className={`inline-flex items-center gap-1.5 rounded-full border px-2.5 py-1 text-xs font-medium ${
                wsConnected
                  ? "border-emerald-500/30 bg-emerald-500/10 text-emerald-300"
                  : "border-rose-500/30 bg-rose-500/10 text-rose-300"
              }`}
            >
              {wsConnected ? (
                <Wifi className="h-3.5 w-3.5" />
              ) : (
                <WifiOff className="h-3.5 w-3.5" />
              )}
              <span className="hidden sm:inline">
                {wsConnected ? "Live WS" : "Reconnecting"}
              </span>
            </span>

            <Link
              href="/status"
              className="inline-flex items-center gap-1.5 rounded-lg border border-slate-700 bg-slate-900 px-3 py-1.5 text-xs font-medium text-slate-200 hover:bg-slate-800 sm:text-sm"
            >
              <ExternalLink className="h-3.5 w-3.5 text-emerald-400" />
              <span className="hidden md:inline">Status Page</span>
            </Link>

            <button
              onClick={() => setBackupModalOpen(true)}
              className="inline-flex items-center gap-1.5 rounded-lg border border-slate-700 bg-slate-900 px-3 py-1.5 text-xs font-medium text-slate-200 hover:bg-slate-800 sm:text-sm"
              title="Backup & Restore JSON"
            >
              <Archive className="h-4 w-4 text-sky-400" />
              <span className="hidden md:inline">Backup</span>
            </button>

            {authRequired && (
              <button
                onClick={() => setAuthModalOpen(true)}
                className="inline-flex items-center gap-1.5 rounded-lg border border-slate-700 bg-slate-900 px-3 py-1.5 text-xs font-medium text-slate-200 hover:bg-slate-800 sm:text-sm"
                title="API Auth Token"
              >
                <KeyRound className="h-4 w-4 text-amber-400" />
              </button>
            )}

            <button
              onClick={() => setWebhookModalOpen(true)}
              className="inline-flex items-center gap-1.5 rounded-lg border border-slate-700 bg-slate-900 px-3 py-1.5 text-xs font-medium text-slate-200 hover:bg-slate-800 sm:text-sm"
            >
              <Bell className="h-4 w-4 text-amber-400" />
              <span className="hidden sm:inline">Alerts</span>
              {webhooks.length > 0 && (
                <span className="rounded-full bg-slate-800 px-1.5 py-0.5 text-[10px] text-slate-300">
                  {webhooks.length}
                </span>
              )}
            </button>

            <button
              onClick={() => {
                setEditingMonitor(null);
                setMonitorModalOpen(true);
              }}
              className="inline-flex items-center gap-1.5 rounded-lg bg-emerald-600 px-3.5 py-1.5 text-xs font-semibold text-white shadow-sm hover:bg-emerald-500 sm:text-sm"
            >
              <Plus className="h-4 w-4" />
              New Monitor
            </button>
          </div>
        </div>
      </header>

      <main className="mx-auto max-w-7xl space-y-6 px-4 py-6 sm:px-6">
        {apiError && (
          <div className="flex flex-wrap items-center justify-between gap-3 rounded-2xl border border-amber-500/30 bg-amber-500/10 px-4 py-3 text-sm text-amber-200">
            <div className="flex items-center gap-2.5">
              <AlertTriangle className="h-4 w-4 shrink-0 text-amber-400" />
              <span>
                Backend API unreachable ({apiError}). Waiting for daemon on{" "}
                <code className="rounded bg-slate-900 px-1.5 py-0.5 text-xs">
                  {process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080"}
                </code>
                ...
              </span>
            </div>
            <button
              onClick={loadInitial}
              className="inline-flex items-center gap-1.5 rounded-lg border border-amber-500/40 bg-amber-500/20 px-3 py-1 text-xs font-semibold text-amber-100 hover:bg-amber-500/30"
            >
              <RefreshCw className="h-3.5 w-3.5" />
              Retry
            </button>
          </div>
        )}

        {/* KPI Summary Cards */}
        <section className="grid grid-cols-2 gap-3 sm:grid-cols-4 sm:gap-4">
          <div className="rounded-2xl border border-slate-800/80 bg-slate-900/60 p-4">
            <div className="flex items-center justify-between text-xs text-slate-400">
              <span>Total Monitors</span>
              <Server className="h-4 w-4 text-slate-500" />
            </div>
            <div className="mt-2 text-2xl font-bold text-slate-100">
              {stats.total}
            </div>
          </div>

          <div className="rounded-2xl border border-slate-800/80 bg-slate-900/60 p-4">
            <div className="flex items-center justify-between text-xs text-slate-400">
              <span>Operational</span>
              <CheckCircle2 className="h-4 w-4 text-emerald-400" />
            </div>
            <div className="mt-2 text-2xl font-bold text-emerald-400">
              {stats.up}
            </div>
          </div>

          <div className="rounded-2xl border border-slate-800/80 bg-slate-900/60 p-4">
            <div className="flex items-center justify-between text-xs text-slate-400">
              <span>Down / Degraded</span>
              <AlertTriangle className="h-4 w-4 text-rose-400" />
            </div>
            <div className="mt-2 flex items-baseline gap-2 text-2xl font-bold">
              <span className="text-rose-400">{stats.down}</span>
              {stats.degraded > 0 && (
                <span className="text-sm font-medium text-amber-400">
                  + {stats.degraded} warn
                </span>
              )}
            </div>
          </div>

          <div className="rounded-2xl border border-slate-800/80 bg-slate-900/60 p-4">
            <div className="flex items-center justify-between text-xs text-slate-400">
              <span>Avg Latency</span>
              <Clock className="h-4 w-4 text-sky-400" />
            </div>
            <div className="mt-2 text-2xl font-bold text-sky-400">
              {stats.avgLatency} <span className="text-sm font-normal">ms</span>
            </div>
          </div>
        </section>

        {/* Main Split Layout */}
        <section className="grid grid-cols-1 gap-6 lg:grid-cols-12">
          {/* Left Column: Monitors List */}
          <div className="space-y-3 lg:col-span-5">
            <div className="flex flex-col gap-2 sm:flex-row sm:items-center">
              <div className="relative flex-1">
                <Search className="pointer-events-none absolute left-3 top-2.5 h-4 w-4 text-slate-500" />
                <input
                  type="text"
                  value={search}
                  onChange={(e) => setSearch(e.target.value)}
                  placeholder="Search monitors..."
                  className="w-full rounded-xl border border-slate-800 bg-slate-900/80 py-2 pl-9 pr-3 text-sm text-slate-100 placeholder:text-slate-500 focus:border-emerald-500 focus:outline-none"
                />
              </div>

              <div className="flex rounded-xl border border-slate-800 bg-slate-900/80 p-1 text-xs">
                {(["all", "up", "down", "degraded"] as const).map((f) => (
                  <button
                    key={f}
                    onClick={() => setFilter(f)}
                    className={`rounded-lg px-2.5 py-1 font-medium capitalize transition ${
                      filter === f
                        ? "bg-slate-800 text-slate-100"
                        : "text-slate-400 hover:text-slate-200"
                    }`}
                  >
                    {f}
                  </button>
                ))}
              </div>
            </div>

            {loading ? (
              <div className="rounded-2xl border border-slate-800 bg-slate-900/50 p-8 text-center text-sm text-slate-500">
                Loading monitors...
              </div>
            ) : filteredMonitors.length === 0 ? (
              <div className="rounded-2xl border border-slate-800 bg-slate-900/50 p-8 text-center text-sm text-slate-500">
                No monitors match your filter.
              </div>
            ) : (
              <div className="space-y-2.5">
                {filteredMonitors.map((m) => {
                  const isSelected = selectedMonitor?.id === m.id;
                  const bars = m.recent_checks || [];
                  return (
                    <div
                      key={m.id}
                      onClick={() => setSelectedId(m.id)}
                      className={`cursor-pointer rounded-2xl border p-4 transition ${
                        isSelected
                          ? "border-emerald-500/50 bg-slate-900 shadow-lg shadow-emerald-950/20"
                          : "border-slate-800/80 bg-slate-900/50 hover:border-slate-700 hover:bg-slate-900/80"
                      }`}
                    >
                      <div className="flex items-start justify-between gap-2">
                        <div className="flex items-center gap-2.5">
                          <span
                            className={`h-2.5 w-2.5 shrink-0 rounded-full ${statusDot(
                              m.status
                            )}`}
                          />
                          <div>
                            <div className="flex items-center gap-2">
                              <span className="font-semibold text-slate-100">
                                {m.name}
                              </span>
                              {m.maintenance && (
                                <span className="rounded bg-amber-500/20 px-1.5 py-0.5 text-[10px] font-semibold text-amber-300">
                                  MAINT
                                </span>
                              )}
                              {!m.enabled && (
                                <span className="rounded bg-slate-800 px-1.5 py-0.5 text-[10px] text-slate-400">
                                  PAUSED
                                </span>
                              )}
                            </div>
                            <div className="mt-0.5 text-xs text-slate-400">
                              {m.target}
                              {m.port > 0 ? `:${m.port}` : ""}
                            </div>
                          </div>
                        </div>

                        <div className="text-right">
                          <span className="inline-block rounded-md border border-slate-700 bg-slate-950 px-2 py-0.5 text-[10px] font-semibold uppercase text-slate-300">
                            {m.type}
                          </span>
                          <div className="mt-1 text-xs font-medium text-slate-300">
                            {m.last_latency_ms} ms
                          </div>
                        </div>
                      </div>

                      {/* Heartbeat mini-bar */}
                      <div className="mt-3 flex items-center justify-between gap-3">
                        <div className="flex h-3 flex-1 items-center gap-0.5">
                          {Array.from({ length: 30 }).map((_, i) => {
                            const offset = 30 - bars.length;
                            const chk = i >= offset ? bars[i - offset] : null;
                            const bg = !chk
                              ? "bg-slate-800"
                              : chk.status === "up"
                                ? "bg-emerald-500"
                                : chk.status === "degraded"
                                  ? "bg-amber-400"
                                  : "bg-rose-500";
                            return (
                              <span
                                key={i}
                                title={
                                  chk
                                    ? `${chk.status.toUpperCase()} • ${chk.latency_ms}ms`
                                    : "No data"
                                }
                                className={`h-full flex-1 rounded-sm ${bg}`}
                              />
                            );
                          })}
                        </div>
                        <span className="text-xs font-semibold text-emerald-400">
                          {m.uptime_24h.toFixed(1)}%
                        </span>
                      </div>
                    </div>
                  );
                })}
              </div>
            )}
          </div>

          {/* Right Column: Selected Monitor Detail */}
          <div className="lg:col-span-7">
            {!selectedMonitor ? (
              <div className="flex h-80 flex-col items-center justify-center rounded-2xl border border-slate-800 bg-slate-900/40 p-8 text-center">
                <Globe className="mb-3 h-10 w-10 text-slate-600" />
                <p className="text-sm text-slate-400">
                  Select a monitor or create a new one to inspect real-time
                  telemetry.
                </p>
              </div>
            ) : (
              <div className="space-y-5 rounded-2xl border border-slate-800/80 bg-slate-900/60 p-5 sm:p-6">
                {/* Header */}
                <div className="flex flex-wrap items-start justify-between gap-4 border-b border-slate-800 pb-4">
                  <div>
                    <div className="flex items-center gap-2.5">
                      <h2 className="text-xl font-bold text-slate-100">
                        {selectedMonitor.name}
                      </h2>
                      <span
                        className={`rounded-full border px-2.5 py-0.5 text-xs font-semibold uppercase ${statusColor(
                          selectedMonitor.status
                        )}`}
                      >
                        {selectedMonitor.status}
                      </span>
                    </div>
                    <p className="mt-1 text-sm text-slate-400">
                      {selectedMonitor.type.toUpperCase()} •{" "}
                      {selectedMonitor.target}
                      {selectedMonitor.port > 0
                        ? `:${selectedMonitor.port}`
                        : ""}{" "}
                      • every {selectedMonitor.interval_seconds}s
                    </p>
                    {selectedMonitor.last_error && (
                      <p className="mt-2 inline-flex items-center gap-1.5 rounded-lg border border-rose-500/30 bg-rose-500/10 px-2.5 py-1 text-xs text-rose-300">
                        <XCircle className="h-3.5 w-3.5 shrink-0" />
                        {selectedMonitor.last_error}
                      </p>
                    )}
                  </div>

                  <div className="flex flex-wrap items-center gap-2">
                    <button
                      onClick={() => api.triggerCheck(selectedMonitor.id)}
                      className="inline-flex items-center gap-1.5 rounded-lg border border-slate-700 bg-slate-950 px-3 py-1.5 text-xs font-medium text-slate-200 hover:bg-slate-800"
                    >
                      <RefreshCw className="h-3.5 w-3.5" />
                      Check Now
                    </button>
                    <button
                      onClick={() => handleToggleEnabled(selectedMonitor)}
                      className="inline-flex items-center gap-1.5 rounded-lg border border-slate-700 bg-slate-950 px-3 py-1.5 text-xs font-medium text-slate-200 hover:bg-slate-800"
                    >
                      {selectedMonitor.enabled ? (
                        <>
                          <Pause className="h-3.5 w-3.5" /> Pause
                        </>
                      ) : (
                        <>
                          <Play className="h-3.5 w-3.5" /> Resume
                        </>
                      )}
                    </button>
                    <button
                      onClick={() => {
                        setEditingMonitor(selectedMonitor);
                        setMonitorModalOpen(true);
                      }}
                      className="inline-flex items-center gap-1.5 rounded-lg border border-slate-700 bg-slate-950 px-3 py-1.5 text-xs font-medium text-slate-200 hover:bg-slate-800"
                    >
                      <Edit3 className="h-3.5 w-3.5" />
                      Edit
                    </button>
                    <button
                      onClick={() => handleDeleteMonitor(selectedMonitor.id)}
                      className="inline-flex items-center gap-1.5 rounded-lg border border-rose-500/30 bg-rose-500/10 px-3 py-1.5 text-xs font-medium text-rose-300 hover:bg-rose-500/20"
                    >
                      <Trash2 className="h-3.5 w-3.5" />
                    </button>
                  </div>
                </div>

                {/* Metric Strip */}
                <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
                  <div className="rounded-xl border border-slate-800 bg-slate-950/60 p-3">
                    <div className="text-[11px] text-slate-400">
                      Current Latency
                    </div>
                    <div className="mt-1 text-lg font-bold text-slate-100">
                      {selectedMonitor.last_latency_ms} ms
                    </div>
                  </div>
                  <div className="rounded-xl border border-slate-800 bg-slate-950/60 p-3">
                    <div className="text-[11px] text-slate-400">
                      Avg Latency (24h)
                    </div>
                    <div className="mt-1 text-lg font-bold text-slate-100">
                      {Math.round(selectedMonitor.avg_latency_24h || 0)} ms
                    </div>
                  </div>
                  <div className="rounded-xl border border-slate-800 bg-slate-950/60 p-3">
                    <div className="text-[11px] text-slate-400">
                      Uptime (24h)
                    </div>
                    <div className="mt-1 text-lg font-bold text-emerald-400">
                      {selectedMonitor.uptime_24h.toFixed(2)}%
                    </div>
                  </div>
                  <div className="rounded-xl border border-slate-800 bg-slate-950/60 p-3">
                    <div className="flex items-center gap-1 text-[11px] text-slate-400">
                      <ShieldCheck className="h-3.5 w-3.5 text-emerald-400" />
                      SSL Expiry
                    </div>
                    <div className="mt-1 text-lg font-bold text-slate-100">
                      {selectedMonitor.ssl_days_remaining !== undefined
                        ? `${selectedMonitor.ssl_days_remaining}d`
                        : "N/A"}
                    </div>
                  </div>
                </div>

                {/* Real-time Latency Chart */}
                <LatencyChart checks={checks} />

                {/* Checks / Incidents Tabs */}
                <div>
                  <div className="mb-3 flex gap-2 border-b border-slate-800 pb-2">
                    <button
                      onClick={() => setDetailTab("checks")}
                      className={`rounded-lg px-3 py-1.5 text-xs font-semibold ${
                        detailTab === "checks"
                          ? "bg-slate-800 text-slate-100"
                          : "text-slate-400 hover:text-slate-200"
                      }`}
                    >
                      Recent Checks ({checks.length})
                    </button>
                    <button
                      onClick={() => setDetailTab("incidents")}
                      className={`rounded-lg px-3 py-1.5 text-xs font-semibold ${
                        detailTab === "incidents"
                          ? "bg-slate-800 text-slate-100"
                          : "text-slate-400 hover:text-slate-200"
                      }`}
                    >
                      Incidents ({monitorIncidents.length})
                    </button>
                  </div>

                  {detailTab === "checks" ? (
                    <div className="max-h-60 overflow-y-auto rounded-xl border border-slate-800 bg-slate-950/50">
                      <table className="w-full text-left text-xs">
                        <thead className="sticky top-0 bg-slate-900 text-slate-400">
                          <tr>
                            <th className="px-3 py-2 font-medium">Status</th>
                            <th className="px-3 py-2 font-medium">Latency</th>
                            <th className="px-3 py-2 font-medium">Details</th>
                            <th className="px-3 py-2 font-medium text-right">
                              Checked At
                            </th>
                          </tr>
                        </thead>
                        <tbody className="divide-y divide-slate-800/70">
                          {checks.slice(0, 20).map((c) => (
                            <tr key={c.id} className="hover:bg-slate-900/40">
                              <td className="px-3 py-2">
                                <span
                                  className={`inline-block rounded px-1.5 py-0.5 text-[10px] font-semibold uppercase ${statusColor(
                                    c.status
                                  )}`}
                                >
                                  {c.status}
                                </span>
                              </td>
                              <td className="px-3 py-2 text-slate-300">
                                {c.latency_ms} ms
                              </td>
                              <td className="max-w-[200px] truncate px-3 py-2 text-slate-400">
                                {c.error ||
                                  (c.status_code
                                    ? `HTTP ${c.status_code}`
                                    : "OK")}
                              </td>
                              <td className="px-3 py-2 text-right text-slate-500">
                                {new Date(c.checked_at).toLocaleTimeString()}
                              </td>
                            </tr>
                          ))}
                        </tbody>
                      </table>
                    </div>
                  ) : (
                    <div className="max-h-60 space-y-2 overflow-y-auto">
                      {monitorIncidents.length === 0 ? (
                        <div className="rounded-xl border border-slate-800 bg-slate-950/50 p-4 text-center text-xs text-slate-500">
                          No downtime incidents recorded for this monitor.
                        </div>
                      ) : (
                        monitorIncidents.map((inc) => (
                          <div
                            key={inc.id}
                            className="flex items-center justify-between rounded-xl border border-slate-800 bg-slate-950/60 px-3.5 py-2.5 text-xs"
                          >
                            <div>
                              <div className="font-medium text-slate-200">
                                {inc.cause}
                              </div>
                              <div className="mt-0.5 text-slate-500">
                                Started:{" "}
                                {new Date(inc.started_at).toLocaleString()}
                              </div>
                            </div>
                            <div className="text-right">
                              {inc.resolved_at ? (
                                <span className="rounded bg-emerald-500/15 px-2 py-0.5 text-emerald-300">
                                  Resolved ({formatDuration(inc.duration_seconds)})
                                </span>
                              ) : (
                                <span className="rounded bg-rose-500/15 px-2 py-0.5 text-rose-300">
                                  Ongoing
                                </span>
                              )}
                            </div>
                          </div>
                        ))
                      )}
                    </div>
                  )}
                </div>
              </div>
            )}
          </div>
        </section>
      </main>

      <MonitorModal
        open={monitorModalOpen}
        initial={editingMonitor}
        onClose={() => setMonitorModalOpen(false)}
        onSave={handleSaveMonitor}
      />

      <WebhookModal
        open={webhookModalOpen}
        webhooks={webhooks}
        onClose={() => setWebhookModalOpen(false)}
        onChanged={async () => {
          const w = await api.listWebhooks();
          setWebhooks(w);
        }}
      />

      <BackupModal
        open={backupModalOpen}
        onClose={() => setBackupModalOpen(false)}
        onImported={loadInitial}
      />

      <AuthModal
        open={authModalOpen}
        onClose={() => setAuthModalOpen(false)}
        onSaved={loadInitial}
      />

      {/* Real-time Toast Notifications */}
      {toasts.length > 0 && (
        <div className="fixed bottom-4 right-4 z-50 flex w-80 flex-col gap-2">
          {toasts.map((t) => (
            <div
              key={t.id}
              className={`flex items-start justify-between gap-2 rounded-xl border p-3.5 text-xs shadow-2xl backdrop-blur-md ${
                t.variant === "ok"
                  ? "border-emerald-500/40 bg-slate-900/95 text-emerald-200"
                  : t.variant === "err"
                    ? "border-rose-500/40 bg-slate-900/95 text-rose-200"
                    : "border-amber-500/40 bg-slate-900/95 text-amber-200"
              }`}
            >
              <div>
                <div className="font-semibold">{t.title}</div>
                <div className="mt-0.5 text-slate-300">{t.message}</div>
              </div>
              <button
                onClick={() =>
                  setToasts((prev) => prev.filter((item) => item.id !== t.id))
                }
                className="rounded p-0.5 text-slate-400 hover:text-slate-100"
              >
                <X className="h-3.5 w-3.5" />
              </button>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
