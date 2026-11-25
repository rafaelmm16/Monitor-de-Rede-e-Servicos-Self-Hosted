"use client";

import React, { useEffect, useState } from "react";
import Link from "next/link";
import {
  Activity,
  AlertTriangle,
  ArrowLeft,
  CheckCircle2,
  Clock,
  ShieldCheck,
  XCircle,
} from "lucide-react";
import { api } from "@/lib/api";
import { Incident, Monitor } from "@/lib/types";

export default function PublicStatusPage() {
  const [monitors, setMonitors] = useState<Monitor[]>([]);
  const [incidents, setIncidents] = useState<Incident[]>([]);
  const [generatedAt, setGeneratedAt] = useState<string>("");
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    const load = () => {
      api
        .publicStatus()
        .then((res) => {
          setMonitors(res.monitors);
          setIncidents(res.incidents);
          setGeneratedAt(res.generated_at);
        })
        .catch(() => {})
        .finally(() => setLoading(false));
    };
    load();
    const timer = setInterval(load, 15000);
    return () => clearInterval(timer);
  }, []);

  const downCount = monitors.filter((m) => m.status === "down").length;
  const degradedCount = monitors.filter((m) => m.status === "degraded").length;
  const allOperational = monitors.length > 0 && downCount === 0 && degradedCount === 0;

  return (
    <div className="min-h-screen bg-slate-950 text-slate-100">
      <header className="border-b border-slate-800/80 bg-slate-950/90">
        <div className="mx-auto flex max-w-4xl items-center justify-between px-4 py-4 sm:px-6">
          <div className="flex items-center gap-3">
            <div className="flex h-9 w-9 items-center justify-center rounded-xl bg-emerald-500/15 ring-1 ring-emerald-500/30">
              <Activity className="h-5 w-5 text-emerald-400" />
            </div>
            <div>
              <h1 className="text-lg font-bold text-slate-100">
                NetPulse Status
              </h1>
              <p className="text-xs text-slate-400">
                Public Service Health Overview
              </p>
            </div>
          </div>

          <Link
            href="/"
            className="inline-flex items-center gap-1.5 rounded-lg border border-slate-800 bg-slate-900 px-3 py-1.5 text-xs font-medium text-slate-300 hover:bg-slate-800"
          >
            <ArrowLeft className="h-3.5 w-3.5" />
            Dashboard
          </Link>
        </div>
      </header>

      <main className="mx-auto max-w-4xl space-y-6 px-4 py-8 sm:px-6">
        {/* Global Banner */}
        <div
          className={`flex items-center justify-between rounded-2xl border p-5 ${
            allOperational
              ? "border-emerald-500/30 bg-emerald-500/10"
              : downCount > 0
                ? "border-rose-500/30 bg-rose-500/10"
                : "border-amber-500/30 bg-amber-500/10"
          }`}
        >
          <div className="flex items-center gap-3">
            {allOperational ? (
              <CheckCircle2 className="h-7 w-7 text-emerald-400" />
            ) : downCount > 0 ? (
              <XCircle className="h-7 w-7 text-rose-400" />
            ) : (
              <AlertTriangle className="h-7 w-7 text-amber-400" />
            )}
            <div>
              <h2 className="text-lg font-bold">
                {loading
                  ? "Checking services..."
                  : allOperational
                    ? "All Systems Operational"
                    : downCount > 0
                      ? `${downCount} Service(s) Experiencing Outage`
                      : `${degradedCount} Service(s) Degraded`}
              </h2>
              {generatedAt && (
                <p className="text-xs text-slate-400">
                  Last updated: {new Date(generatedAt).toLocaleString()}
                </p>
              )}
            </div>
          </div>
        </div>

        {/* Service List */}
        <section className="space-y-3">
          <h3 className="text-xs font-semibold uppercase tracking-wider text-slate-400">
            Monitored Services ({monitors.length})
          </h3>
          <div className="divide-y divide-slate-800/80 overflow-hidden rounded-2xl border border-slate-800/80 bg-slate-900/50">
            {monitors.map((m) => {
              const bars = m.recent_checks || [];
              return (
                <div key={m.id} className="p-4 sm:p-5">
                  <div className="flex flex-wrap items-center justify-between gap-2">
                    <div className="flex items-center gap-2.5">
                      <span
                        className={`h-2.5 w-2.5 rounded-full ${
                          m.status === "up"
                            ? "bg-emerald-400"
                            : m.status === "degraded"
                              ? "bg-amber-400"
                              : m.status === "down"
                                ? "bg-rose-500"
                                : "bg-slate-500"
                        }`}
                      />
                      <span className="font-semibold text-slate-100">
                        {m.name}
                      </span>
                      <span className="rounded bg-slate-800 px-1.5 py-0.5 text-[10px] font-semibold uppercase text-slate-300">
                        {m.type}
                      </span>
                      {m.maintenance && (
                        <span className="rounded bg-amber-500/20 px-1.5 py-0.5 text-[10px] font-semibold text-amber-300">
                          MAINTENANCE
                        </span>
                      )}
                    </div>

                    <div className="flex items-center gap-4 text-xs">
                      {m.ssl_days_remaining !== undefined && (
                        <span className="inline-flex items-center gap-1 text-slate-400">
                          <ShieldCheck className="h-3.5 w-3.5 text-emerald-400" />
                          SSL {m.ssl_days_remaining}d
                        </span>
                      )}
                      <span className="inline-flex items-center gap-1 text-slate-400">
                        <Clock className="h-3.5 w-3.5" />
                        {m.last_latency_ms} ms
                      </span>
                      <span className="font-semibold text-emerald-400">
                        {m.uptime_24h.toFixed(2)}%
                      </span>
                    </div>
                  </div>

                  <div className="mt-3 flex h-2.5 gap-0.5">
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
                        <span key={i} className={`h-full flex-1 rounded-sm ${bg}`} />
                      );
                    })}
                  </div>
                </div>
              );
            })}
          </div>
        </section>

        {/* Recent Incidents */}
        <section className="space-y-3">
          <h3 className="text-xs font-semibold uppercase tracking-wider text-slate-400">
            Recent Incidents
          </h3>
          {incidents.length === 0 ? (
            <div className="rounded-2xl border border-slate-800 bg-slate-900/40 p-6 text-center text-sm text-slate-500">
              No recent incidents reported.
            </div>
          ) : (
            <div className="space-y-2">
              {incidents.map((inc) => (
                <div
                  key={inc.id}
                  className="flex flex-wrap items-center justify-between gap-2 rounded-xl border border-slate-800 bg-slate-900/50 px-4 py-3 text-xs"
                >
                  <div>
                    <span className="font-semibold text-slate-100">
                      {inc.monitor_name}
                    </span>
                    <span className="mx-2 text-slate-600">•</span>
                    <span className="text-slate-300">{inc.cause}</span>
                    <div className="mt-0.5 text-slate-500">
                      {new Date(inc.started_at).toLocaleString()}
                    </div>
                  </div>
                  <span
                    className={`rounded px-2 py-0.5 font-medium ${
                      inc.resolved_at
                        ? "bg-emerald-500/15 text-emerald-300"
                        : "bg-rose-500/15 text-rose-300"
                    }`}
                  >
                    {inc.resolved_at
                      ? `Resolved (${inc.duration_seconds}s)`
                      : "Ongoing"}
                  </span>
                </div>
              ))}
            </div>
          )}
        </section>
      </main>
    </div>
  );
}
