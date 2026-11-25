"use client";

import React, { useState } from "react";
import { Download, KeyRound, Upload, X } from "lucide-react";
import { api, getAuthToken, setAuthToken } from "@/lib/api";
import { ConfigBackup } from "@/lib/types";

interface BackupModalProps {
  open: boolean;
  onClose: () => void;
  onImported: () => Promise<void>;
}

export function BackupModal({ open, onClose, onImported }: BackupModalProps) {
  const [status, setStatus] = useState<{ type: "ok" | "err"; msg: string } | null>(null);
  const [busy, setBusy] = useState(false);

  if (!open) return null;

  const handleExport = async () => {
    setBusy(true);
    setStatus(null);
    try {
      const data = await api.exportConfig();
      const blob = new Blob([JSON.stringify(data, null, 2)], {
        type: "application/json",
      });
      const url = URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = url;
      a.download = `netpulse-backup-${new Date().toISOString().slice(0, 10)}.json`;
      a.click();
      URL.revokeObjectURL(url);
      setStatus({ type: "ok", msg: "Backup JSON downloaded." });
    } catch (err) {
      setStatus({
        type: "err",
        msg: err instanceof Error ? err.message : "Export failed",
      });
    } finally {
      setBusy(false);
    }
  };

  const handleImportFile = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;
    setBusy(true);
    setStatus(null);
    try {
      const text = await file.text();
      const parsed = JSON.parse(text) as ConfigBackup;
      const res = await api.importConfig(parsed);
      await onImported();
      setStatus({
        type: "ok",
        msg: `Imported ${res.imported_monitors} monitor(s) and ${res.imported_webhooks} webhook(s).`,
      });
    } catch (err) {
      setStatus({
        type: "err",
        msg: err instanceof Error ? err.message : "Invalid backup JSON",
      });
    } finally {
      setBusy(false);
      e.target.value = "";
    }
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/70 p-4 backdrop-blur-sm">
      <div className="w-full max-w-md rounded-2xl border border-slate-800 bg-slate-900 shadow-2xl">
        <div className="flex items-center justify-between border-b border-slate-800 px-6 py-4">
          <h2 className="text-lg font-semibold text-slate-100">
            Backup & Restore (JSON)
          </h2>
          <button
            onClick={onClose}
            className="rounded-lg p-1 text-slate-400 hover:bg-slate-800"
          >
            <X className="h-5 w-5" />
          </button>
        </div>

        <div className="space-y-4 p-6">
          {status && (
            <div
              className={`rounded-lg border px-3 py-2 text-sm ${
                status.type === "ok"
                  ? "border-emerald-500/40 bg-emerald-500/10 text-emerald-300"
                  : "border-rose-500/40 bg-rose-500/10 text-rose-300"
              }`}
            >
              {status.msg}
            </div>
          )}

          <p className="text-sm text-slate-400">
            Export all monitors and webhook channels as JSON, or import a
            previous backup file.
          </p>

          <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
            <button
              onClick={handleExport}
              disabled={busy}
              className="inline-flex items-center justify-center gap-2 rounded-xl bg-emerald-600 px-4 py-2.5 text-sm font-medium text-white hover:bg-emerald-500 disabled:opacity-50"
            >
              <Download className="h-4 w-4" />
              Export JSON
            </button>

            <label className="inline-flex cursor-pointer items-center justify-center gap-2 rounded-xl border border-slate-700 bg-slate-950 px-4 py-2.5 text-sm font-medium text-slate-200 hover:bg-slate-800">
              <Upload className="h-4 w-4" />
              Import JSON
              <input
                type="file"
                accept="application/json,.json"
                onChange={handleImportFile}
                className="hidden"
              />
            </label>
          </div>
        </div>
      </div>
    </div>
  );
}

interface AuthModalProps {
  open: boolean;
  onClose: () => void;
  onSaved: () => void;
}

export function AuthModal({ open, onClose, onSaved }: AuthModalProps) {
  const [token, setToken] = useState(() => getAuthToken());

  if (!open) return null;

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    setAuthToken(token.trim());
    onSaved();
    onClose();
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/70 p-4 backdrop-blur-sm">
      <div className="w-full max-w-md rounded-2xl border border-slate-800 bg-slate-900 shadow-2xl">
        <div className="flex items-center justify-between border-b border-slate-800 px-6 py-4">
          <div className="flex items-center gap-2">
            <KeyRound className="h-5 w-5 text-emerald-400" />
            <h2 className="text-lg font-semibold text-slate-100">
              API Authentication Token
            </h2>
          </div>
          <button
            onClick={onClose}
            className="rounded-lg p-1 text-slate-400 hover:bg-slate-800"
          >
            <X className="h-5 w-5" />
          </button>
        </div>

        <form onSubmit={handleSubmit} className="space-y-4 p-6">
          <p className="text-sm text-slate-400">
            This NetPulse instance has <code className="text-slate-200">NETPULSE_AUTH_TOKEN</code>{" "}
            enabled. Enter your token to create, edit, or delete monitors.
          </p>
          <input
            type="password"
            value={token}
            onChange={(e) => setToken(e.target.value)}
            placeholder="Enter NETPULSE_AUTH_TOKEN..."
            className="w-full rounded-lg border border-slate-700 bg-slate-950 px-3 py-2 text-sm text-slate-100 focus:border-emerald-500 focus:outline-none"
          />
          <div className="flex justify-end gap-2">
            <button
              type="button"
              onClick={onClose}
              className="rounded-lg border border-slate-700 px-4 py-2 text-sm text-slate-300 hover:bg-slate-800"
            >
              Cancel
            </button>
            <button
              type="submit"
              className="rounded-lg bg-emerald-600 px-4 py-2 text-sm font-medium text-white hover:bg-emerald-500"
            >
              Save Token
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
