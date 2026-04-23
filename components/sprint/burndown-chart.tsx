"use client";

import { useMemo, useState } from "react";
import { useTranslations } from "next-intl";
import type { SprintBurndownPoint } from "@/lib/stores/sprint-store";

interface BurndownChartProps {
  burndown: SprintBurndownPoint[];
  plannedTasks: number;
}

const PADDING = { top: 16, right: 16, bottom: 32, left: 40 };

export function BurndownChart({ burndown, plannedTasks }: BurndownChartProps) {
  const t = useTranslations("sprints");
  const [hovered, setHovered] = useState<number | null>(null);

  const points = burndown;
  const count = points.length;
  const maxTasks = Math.max(plannedTasks, ...points.map((p) => p.remainingTasks), 0);
  const yTicks = useMemo(() => {
    const step = Math.max(1, Math.ceil(maxTasks / 4));
    const ticks: number[] = [];
    for (let v = 0; v <= maxTasks; v += step) ticks.push(v);
    if (ticks[ticks.length - 1] !== maxTasks && maxTasks > 0) ticks.push(maxTasks);
    return ticks;
  }, [maxTasks]);
  const xLabels = useMemo(() => {
    if (count <= 3) return points.map((p, i) => ({ i, label: p.date.slice(5) }));
    const mid = Math.floor(count / 2);
    return [
      { i: 0, label: points[0].date.slice(5) },
      { i: mid, label: points[mid].date.slice(5) },
      { i: count - 1, label: points[count - 1].date.slice(5) },
    ];
  }, [count, points]);

  if (count === 0) {
    return (
      <div className="flex h-[200px] items-center justify-center text-sm text-muted-foreground">
        {t("burndown.empty")}
      </div>
    );
  }

  const viewWidth = 480;
  const viewHeight = 200;
  const chartW = viewWidth - PADDING.left - PADDING.right;
  const chartH = viewHeight - PADDING.top - PADDING.bottom;

  const xScale = (i: number) => PADDING.left + (count > 1 ? (i / (count - 1)) * chartW : chartW / 2);
  const yScale = (v: number) => PADDING.top + (maxTasks > 0 ? (1 - v / maxTasks) * chartH : chartH);

  // Ideal line: from plannedTasks at start to 0 at end
  const idealLine = `M ${xScale(0)},${yScale(plannedTasks)} L ${xScale(count - 1)},${yScale(0)}`;

  // Actual remaining line
  const actualLine = points
    .map((p, i) => `${i === 0 ? "M" : "L"} ${xScale(i)},${yScale(p.remainingTasks)}`)
    .join(" ");

  const hoveredPoint = hovered !== null ? points[hovered] : null;

  return (
    <div className="relative w-full">
      <svg
        viewBox={`0 0 ${viewWidth} ${viewHeight}`}
        className="w-full"
        style={{ height: 200 }}
        onMouseLeave={() => setHovered(null)}
      >
        {/* Grid lines */}
        {yTicks.map((v) => (
          <line
            key={v}
            x1={PADDING.left}
            y1={yScale(v)}
            x2={viewWidth - PADDING.right}
            y2={yScale(v)}
            className="stroke-border/40"
            strokeWidth={0.5}
          />
        ))}

        {/* Y-axis labels */}
        {yTicks.map((v) => (
          <text
            key={`y-${v}`}
            x={PADDING.left - 6}
            y={yScale(v) + 4}
            textAnchor="end"
            className="fill-muted-foreground"
            fontSize={10}
          >
            {v}
          </text>
        ))}

        {/* X-axis labels */}
        {xLabels.map(({ i, label }) => (
          <text
            key={`x-${i}`}
            x={xScale(i)}
            y={viewHeight - 6}
            textAnchor="middle"
            className="fill-muted-foreground"
            fontSize={10}
          >
            {label}
          </text>
        ))}

        {/* Ideal line */}
        <path d={idealLine} fill="none" className="stroke-muted-foreground/50" strokeWidth={1.5} strokeDasharray="6 3" />

        {/* Actual line */}
        <path d={actualLine} fill="none" className="stroke-primary" strokeWidth={2} />

        {/* Hover dots */}
        {points.map((p, i) => (
          <circle
            key={p.date}
            cx={xScale(i)}
            cy={yScale(p.remainingTasks)}
            r={hovered === i ? 5 : 3}
            className={hovered === i ? "fill-primary" : "fill-primary/60"}
            onMouseEnter={() => setHovered(i)}
          />
        ))}
      </svg>

      {/* Tooltip */}
      {hoveredPoint && hovered !== null && (
        <div
          className="pointer-events-none absolute rounded border border-border bg-popover px-2 py-1 text-xs shadow-sm"
          style={{
            left: `${(xScale(hovered) / viewWidth) * 100}%`,
            top: 4,
            transform: "translateX(-50%)",
          }}
        >
          <div className="font-medium">{hoveredPoint.date}</div>
          <div>{t("burndown.remaining", { remainingTasks: hoveredPoint.remainingTasks })}</div>
          <div>{t("burndown.completed", { completedTasks: hoveredPoint.completedTasks })}</div>
        </div>
      )}
    </div>
  );
}
