"use client";

import React, { useState } from "react";
import { Bell, Send, Trash2, X } from "lucide-react";
import { Webhook, WebhookType } from "@/lib/types";
import { api } from "@/lib/api";

interface WebhookModalProps {
  open: boolean;
  webhooks: Webhook[];
  onClose: () => void;
  onChanged: () => Promise<void>;
}

export function WebhookModal({
  open,
  webhooks,
  onClose,
  onChanged,
}: WebhookModalProps) {
  const [name, setName] = useState("");
  const [type, setType] = useState<WebhookType>("discord");
  const [url, setUrl] = useState("");
  const [botToken, setBotToken] = useState("");
  const [chatID, setChatID] = useState("");
  const [saving, setSaving] = useState(false);
  const [feedback, setFeedback] = useState<{ type: "ok" | "err"; msg: string } | null>(null);

  if (!open) return null;

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault();
    setSaving(true);
    setFeedback(null);
    try {
      await api.createWebhook({
        name: name.trim(),
        type,
        url: url.trim(),
        bot_token: botToken.trim(),
        chat_id: chatID.trim(),
        enabled: true,
      });
      setName("");
      setUrl("");
      setBotToken("");
      setChatID("");
      await onChanged();
      setFeedback({ type: "ok", msg: "Webhook added." });
    } catch (err) {
      setFeedback({
        type: "err",
        msg: err instanceof Error ? err.message : "Failed to add webhook",
      });
    } finally {
      setSaving(false);
    }
  };

  const handleTest = async (id: string) => {
    setFeedback(null);
    try {
      await api.testWebhook(id);
      setFeedback({ type: "ok", msg: "Test alert sent successfully!" });
    } catch (err) {
      setFeedback({
        type: "err",
        msg: err instanceof Error ? err.message : "Test failed",
      });
    }
  };

  const handleDelete = async (id: string) => {
    await api.deleteWebhook(id);
    await onChanged();
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/70 p-4 backdrop-blur-sm">
      <div className="w-full max-w-xl rounded-2xl border border-slate-800 bg-slate-900 shadow-2xl">
        <div className="flex items-center justify-between border-b border-slate-800 px-6 py-4">
          <div className="flex items-center gap-2">
            <Bell className="h-5 w-5 text-emerald-400" />
            <h2 className="text-lg font-semibold text-slate-100">
              Alert Webhooks (Discord / Telegram)
            </h2>
          </div>
          <button
            onClick={onClose}
            className="rounded-lg p-1 text-slate-400 hover:bg-slate-800 hover:text-slate-200"
          >
            <X className="h-5 w-5" />
          </button>
        </div>

        <div className="space-y-6 p-6">
          {feedback && (
            <div
              className={`rounded-lg border px-3 py-2 text-sm ${
                feedback.type === "ok"
                  ? "border-emerald-500/40 bg-emerald-500/10 text-emerald-300"
                  : "border-rose-500/40 bg-rose-500/10 text-rose-300"
              }`}
            >
              {feedback.msg}
            </div>
          )}

          {/* Existing Webhooks */}
          <div className="space-y-2">
            <h3 className="text-xs font-semibold uppercase tracking-wider text-slate-400">
              Configured Channels ({webhooks.length})
            </h3>
            {webhooks.length === 0 ? (
              <p className="rounded-xl border border-slate-800 bg-slate-950/60 p-4 text-sm text-slate-500">
                No webhooks configured yet. Add Discord or Telegram below.
              </p>
            ) : (
              <div className="max-h-48 space-y-2 overflow-y-auto">
                {webhooks.map((wh) => (
                  <div
                    key={wh.id}
                    className="flex items-center justify-between rounded-xl border border-slate-800 bg-slate-950/70 px-4 py-3"
                  >
                    <div>
                      <div className="flex items-center gap-2">
                        <span className="font-medium text-slate-100">
                          {wh.name}
                        </span>
                        <span className="rounded bg-slate-800 px-2 py-0.5 text-[11px] font-medium uppercase text-slate-300">
                          {wh.type}
                        </span>
                      </div>
                      <div className="mt-0.5 text-xs text-slate-500">
                        {wh.type === "telegram"
                          ? `Chat ID: ${wh.chat_id}`
                          : wh.url}
                      </div>
                    </div>
                    <div className="flex items-center gap-2">
                      <button
                        onClick={() => handleTest(wh.id)}
                        className="inline-flex items-center gap-1 rounded-lg border border-slate-700 px-2.5 py-1.5 text-xs text-slate-300 hover:bg-slate-800"
                      >
                        <Send className="h-3.5 w-3.5" />
                        Test
                      </button>
                      <button
                        onClick={() => handleDelete(wh.id)}
                        className="rounded-lg p-1.5 text-slate-400 hover:bg-rose-500/10 hover:text-rose-400"
                        title="Delete webhook"
                      >
                        <Trash2 className="h-4 w-4" />
                      </button>
                    </div>
                  </div>
                ))}
              </div>
            )}
          </div>

          {/* Add Webhook Form */}
          <form
            onSubmit={handleCreate}
            className="space-y-3 rounded-xl border border-slate-800 bg-slate-950/50 p-4"
          >
            <h3 className="text-xs font-semibold uppercase tracking-wider text-slate-400">
              Add New Channel
            </h3>
            <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
              <input
                type="text"
                required
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder="Channel Name (e.g. Ops Discord)"
                className="rounded-lg border border-slate-700 bg-slate-950 px-3 py-2 text-sm text-slate-100 focus:border-emerald-500 focus:outline-none"
              />
              <select
                value={type}
                onChange={(e) => setType(e.target.value as WebhookType)}
                className="rounded-lg border border-slate-700 bg-slate-950 px-3 py-2 text-sm text-slate-100 focus:border-emerald-500 focus:outline-none"
              >
                <option value="discord">Discord Webhook</option>
                <option value="telegram">Telegram Bot</option>
                <option value="generic">Generic JSON Webhook</option>
              </select>
            </div>

            {type === "telegram" ? (
              <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
                <input
                  type="text"
                  required
                  value={botToken}
                  onChange={(e) => setBotToken(e.target.value)}
                  placeholder="Bot Token (123456:ABC...)"
                  className="rounded-lg border border-slate-700 bg-slate-950 px-3 py-2 text-sm text-slate-100 focus:border-emerald-500 focus:outline-none"
                />
                <input
                  type="text"
                  required
                  value={chatID}
                  onChange={(e) => setChatID(e.target.value)}
                  placeholder="Chat ID (-100...)"
                  className="rounded-lg border border-slate-700 bg-slate-950 px-3 py-2 text-sm text-slate-100 focus:border-emerald-500 focus:outline-none"
                />
              </div>
            ) : (
              <input
                type="url"
                required
                value={url}
                onChange={(e) => setUrl(e.target.value)}
                placeholder={
                  type === "discord"
                    ? "https://discord.com/api/webhooks/..."
                    : "https://hooks.example.com/alert"
                }
                className="w-full rounded-lg border border-slate-700 bg-slate-950 px-3 py-2 text-sm text-slate-100 focus:border-emerald-500 focus:outline-none"
              />
            )}

            <div className="flex justify-end">
              <button
                type="submit"
                disabled={saving}
                className="rounded-lg bg-emerald-600 px-4 py-2 text-sm font-medium text-white hover:bg-emerald-500 disabled:opacity-50"
              >
                {saving ? "Adding..." : "Add Webhook"}
              </button>
            </div>
          </form>
        </div>
      </div>
    </div>
  );
}
