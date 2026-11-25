"use client";

import React, { useEffect, useState } from "react";
import { X } from "lucide-react";
import { Monitor, MonitorType } from "@/lib/types";

interface MonitorModalProps {
  open: boolean;
  initial?: Monitor | null;
  onClose: () => void;
  onSave: (data: Partial<Monitor>, existingId?: string) => Promise<void>;
}

export function MonitorModal({
  open,
  initial,
  onClose,
  onSave,
}: MonitorModalProps) {
  const [name, setName] = useState("");
  const [type, setType] = useState<MonitorType>("http");
  const [target, setTarget] = useState("");
  const [port, setPort] = useState(443);
  const [intervalSeconds, setIntervalSeconds] = useState(30);
  const [timeoutSeconds, setTimeoutSeconds] = useState(5);
  const [expectedStatus, setExpectedStatus] = useState(200);
  const [keyword, setKeyword] = useState("");
  const [sslWarnDays, setSslWarnDays] = useState(14);
  const [enabled, setEnabled] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => {
    if (initial) {
      setName(initial.name);
      setType(initial.type);
      setTarget(initial.target);
      setPort(initial.port || (initial.type === "ssl" ? 443 : 80));
      setIntervalSeconds(initial.interval_seconds || 30);
      setTimeoutSeconds(initial.timeout_seconds || 5);
      setExpectedStatus(initial.expected_status ?? 200);
      setKeyword(initial.keyword || "");
      setSslWarnDays(initial.ssl_expiry_warn_days || 14);
      setEnabled(initial.enabled);
    } else {
      setName("");
      setType("http");
      setTarget("");
      setPort(443);
      setIntervalSeconds(30);
      setTimeoutSeconds(5);
      setExpectedStatus(200);
      setKeyword("");
      setSslWarnDays(14);
      setEnabled(true);
    }
    setError("");
  }, [initial, open]);

  if (!open) return null;

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setSaving(true);
    setError("");
    try {
      await onSave(
        {
          name: name.trim(),
          type,
          target: target.trim(),
          port: Number(port) || 0,
          interval_seconds: Number(intervalSeconds) || 30,
          timeout_seconds: Number(timeoutSeconds) || 5,
          expected_status: Number(expectedStatus) || 0,
          keyword: keyword.trim(),
          ssl_expiry_warn_days: Number(sslWarnDays) || 14,
          enabled,
        },
        initial?.id
      );
      onClose();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to save monitor");
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/70 p-4 backdrop-blur-sm">
      <div className="w-full max-w-lg rounded-2xl border border-slate-800 bg-slate-900 shadow-2xl">
        <div className="flex items-center justify-between border-b border-slate-800 px-6 py-4">
          <h2 className="text-lg font-semibold text-slate-100">
            {initial ? "Edit Monitor" : "Add New Monitor"}
          </h2>
          <button
            onClick={onClose}
            className="rounded-lg p-1 text-slate-400 hover:bg-slate-800 hover:text-slate-200"
          >
            <X className="h-5 w-5" />
          </button>
        </div>

        <form onSubmit={handleSubmit} className="space-y-4 p-6">
          {error && (
            <div className="rounded-lg border border-rose-500/40 bg-rose-500/10 px-3 py-2 text-sm text-rose-300">
              {error}
            </div>
          )}

          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
            <div>
              <label className="mb-1 block text-xs font-medium text-slate-400">
                Monitor Name
              </label>
              <input
                type="text"
                required
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder="e.g. Production API"
                className="w-full rounded-lg border border-slate-700 bg-slate-950 px-3 py-2 text-sm text-slate-100 focus:border-emerald-500 focus:outline-none"
              />
            </div>

            <div>
              <label className="mb-1 block text-xs font-medium text-slate-400">
                Check Type
              </label>
              <select
                value={type}
                onChange={(e) => {
                  const next = e.target.value as MonitorType;
                  setType(next);
                  if (next === "ssl") setPort(443);
                  if (next === "tcp" && port === 443) setPort(5432);
                }}
                className="w-full rounded-lg border border-slate-700 bg-slate-950 px-3 py-2 text-sm text-slate-100 focus:border-emerald-500 focus:outline-none"
              >
                <option value="http">HTTP / HTTPS</option>
                <option value="icmp">Ping (ICMP)</option>
                <option value="tcp">TCP Port</option>
                <option value="ssl">SSL Certificate</option>
              </select>
            </div>
          </div>

          <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
            <div className={type === "tcp" || type === "ssl" ? "sm:col-span-2" : "sm:col-span-3"}>
              <label className="mb-1 block text-xs font-medium text-slate-400">
                {type === "http" ? "URL" : "Hostname / IP"}
              </label>
              <input
                type="text"
                required
                value={target}
                onChange={(e) => setTarget(e.target.value)}
                placeholder={
                  type === "http"
                    ? "https://api.example.com/health"
                    : "1.1.1.1 or example.com"
                }
                className="w-full rounded-lg border border-slate-700 bg-slate-950 px-3 py-2 text-sm text-slate-100 focus:border-emerald-500 focus:outline-none"
              />
            </div>

            {(type === "tcp" || type === "ssl") && (
              <div>
                <label className="mb-1 block text-xs font-medium text-slate-400">
                  Port
                </label>
                <input
                  type="number"
                  min={1}
                  max={65535}
                  value={port}
                  onChange={(e) => setPort(Number(e.target.value))}
                  className="w-full rounded-lg border border-slate-700 bg-slate-950 px-3 py-2 text-sm text-slate-100 focus:border-emerald-500 focus:outline-none"
                />
              </div>
            )}
          </div>

          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="mb-1 block text-xs font-medium text-slate-400">
                Interval (seconds)
              </label>
              <input
                type="number"
                min={5}
                value={intervalSeconds}
                onChange={(e) => setIntervalSeconds(Number(e.target.value))}
                className="w-full rounded-lg border border-slate-700 bg-slate-950 px-3 py-2 text-sm text-slate-100 focus:border-emerald-500 focus:outline-none"
              />
            </div>

            <div>
              <label className="mb-1 block text-xs font-medium text-slate-400">
                Timeout (seconds)
              </label>
              <input
                type="number"
                min={1}
                max={60}
                value={timeoutSeconds}
                onChange={(e) => setTimeoutSeconds(Number(e.target.value))}
                className="w-full rounded-lg border border-slate-700 bg-slate-950 px-3 py-2 text-sm text-slate-100 focus:border-emerald-500 focus:outline-none"
              />
            </div>
          </div>

          {type === "http" && (
            <div className="grid grid-cols-2 gap-4">
              <div>
                <label className="mb-1 block text-xs font-medium text-slate-400">
                  Expected Status (0 = 2xx/3xx)
                </label>
                <input
                  type="number"
                  min={0}
                  max={599}
                  value={expectedStatus}
                  onChange={(e) => setExpectedStatus(Number(e.target.value))}
                  className="w-full rounded-lg border border-slate-700 bg-slate-950 px-3 py-2 text-sm text-slate-100 focus:border-emerald-500 focus:outline-none"
                />
              </div>

              <div>
                <label className="mb-1 block text-xs font-medium text-slate-400">
                  Body Keyword (optional)
                </label>
                <input
                  type="text"
                  value={keyword}
                  onChange={(e) => setKeyword(e.target.value)}
                  placeholder="ok"
                  className="w-full rounded-lg border border-slate-700 bg-slate-950 px-3 py-2 text-sm text-slate-100 focus:border-emerald-500 focus:outline-none"
                />
              </div>
            </div>
          )}

          {(type === "ssl" || type === "http") && (
            <div>
              <label className="mb-1 block text-xs font-medium text-slate-400">
                SSL Expiry Warning Threshold (days)
              </label>
              <input
                type="number"
                min={1}
                max={365}
                value={sslWarnDays}
                onChange={(e) => setSslWarnDays(Number(e.target.value))}
                className="w-full rounded-lg border border-slate-700 bg-slate-950 px-3 py-2 text-sm text-slate-100 focus:border-emerald-500 focus:outline-none"
              />
            </div>
          )}

          <label className="flex items-center gap-2 text-sm text-slate-300">
            <input
              type="checkbox"
              checked={enabled}
              onChange={(e) => setEnabled(e.target.checked)}
              className="h-4 w-4 rounded border-slate-700 bg-slate-950 accent-emerald-500"
            />
            Enable monitor immediately
          </label>

          <div className="flex justify-end gap-3 border-t border-slate-800 pt-4">
            <button
              type="button"
              onClick={onClose}
              className="rounded-lg border border-slate-700 px-4 py-2 text-sm font-medium text-slate-300 hover:bg-slate-800"
            >
              Cancel
            </button>
            <button
              type="submit"
              disabled={saving}
              className="rounded-lg bg-emerald-600 px-4 py-2 text-sm font-medium text-white hover:bg-emerald-500 disabled:opacity-50"
            >
              {saving ? "Saving..." : initial ? "Save Changes" : "Create Monitor"}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
