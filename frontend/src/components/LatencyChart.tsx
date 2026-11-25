"use client";

import React, { useMemo, useState } from "react";
import { CheckResult } from "@/lib/types";

interface LatencyChartProps {
  checks: CheckResult[];
}

export function LatencyChart({ checks }: LatencyChartProps) {
  const [hoveredIdx, setHoveredIdx] = useState<number | null>(null);

  const ordered = useMemo(() => {
    return [...checks].reverse();
  }, [checks]);

  if (ordered.length === 0) {
    return (
      <div className="flex h-48 items-center justify-center rounded-xl border border-slate-800/80 bg-slate-900/40 text-sm text-slate-500">
        Waiting for first health check data...
      </div>
    );
  }

  const width = 680;
  const height = 180;
  const padX = 12;
  const padY = 18;
  const innerW = width - padX * 2;
  const innerH = height - padY * 2;

  const maxLatency = Math.max(
    10,
    ...ordered.map((c) => c.latency_ms || 0)
  );

  const points = ordered.map((c, idx) => {
    const x =
      ordered.length === 1
        ? padX + innerW / 2
        : padX + (idx / (ordered.length - 1)) * innerW;
    const y = padY + innerH - ((c.latency_ms || 0) / maxLatency) * innerH;
    return { x, y, check: c };
  });

  const linePath = points
    .map((p, i) => `${i === 0 ? "M" : "L"} ${p.x.toFixed(1)} ${p.y.toFixed(1)}`)
    .join(" ");

  const areaPath =
    points.length > 0
      ? `${linePath} L ${points[points.length - 1].x.toFixed(1)} ${(
          padY + innerH
        ).toFixed(1)} L ${points[0].x.toFixed(1)} ${(padY + innerH).toFixed(
          1
        )} Z`
      : "";

  const hovered = hoveredIdx !== null ? points[hoveredIdx] : null;

  return (
    <div className="relative rounded-xl border border-slate-800/80 bg-slate-900/50 p-4">
      <div className="mb-2 flex items-center justify-between text-xs text-slate-400">
        <span>Response Time (ms) — Last {ordered.length} checks</span>
        <span>Peak: {maxLatency} ms</span>
      </div>

      <div className="relative">
        <svg
          viewBox={`0 0 ${width} ${height}`}
          className="h-44 w-full overflow-visible"
          onMouseLeave={() => setHoveredIdx(null)}
        >
          <defs>
            <linearGradient id="latencyGrad" x1="0" y1="0" x2="0" y2="1">
              <stop offset="0%" stopColor="#10b981" stopOpacity="0.35" />
              <stop offset="100%" stopColor="#10b981" stopOpacity="0.0" />
            </linearGradient>
          </defs>

          {/* Horizontal grid lines */}
          {[0, 0.5, 1].map((ratio) => {
            const y = padY + innerH * ratio;
            return (
              <line
                key={ratio}
                x1={padX}
                y1={y}
                x2={width - padX}
                y2={y}
                stroke="#1e293b"
                strokeDasharray="3 3"
              />
            );
          })}

          {areaPath && <path d={areaPath} fill="url(#latencyGrad)" />}
          {linePath && (
            <path
              d={linePath}
              fill="none"
              stroke="#10b981"
              strokeWidth="2"
              strokeLinejoin="round"
              strokeLinecap="round"
            />
          )}

          {points.map((p, idx) => {
            const color =
              p.check.status === "down"
                ? "#ef4444"
                : p.check.status === "degraded"
                  ? "#f59e0b"
                  : "#10b981";
            const isHovered = hoveredIdx === idx;
            return (
              <g key={`${p.check.id}-${idx}`}>
                <circle
                  cx={p.x}
                  cy={p.y}
                  r={isHovered ? 5 : 2.5}
                  fill={color}
                  stroke="#020617"
                  strokeWidth="1.5"
                />
                <rect
                  x={p.x - innerW / Math.max(ordered.length * 2, 10)}
                  y={0}
                  width={Math.max(innerW / ordered.length, 10)}
                  height={height}
                  fill="transparent"
                  className="cursor-crosshair"
                  onMouseEnter={() => setHoveredIdx(idx)}
                />
              </g>
            );
          })}
        </svg>

        {hovered && (
          <div className="pointer-events-none absolute right-2 top-0 rounded-lg border border-slate-700 bg-slate-950/95 px-3 py-1.5 text-xs shadow-lg">
            <div className="font-semibold text-slate-100">
              {hovered.check.latency_ms} ms{" "}
              <span
                className={
                  hovered.check.status === "up"
                    ? "text-emerald-400"
                    : hovered.check.status === "degraded"
                      ? "text-amber-400"
                      : "text-rose-400"
                }
              >
                ({hovered.check.status.toUpperCase()})
              </span>
            </div>
            <div className="text-slate-400">
              {new Date(hovered.check.checked_at).toLocaleTimeString()}
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
