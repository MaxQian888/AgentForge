"use client";

import type { LucideIcon } from "lucide-react";
import { ArrowDown, ArrowRight, ArrowUp } from "lucide-react";
import { Area, AreaChart, ResponsiveContainer } from "recharts";
import { cn } from "@/lib/utils";

interface Trend {
  value: number;
  direction: "up" | "down" | "flat";
}

interface SparklinePoint {
  label: string;
  value: number;
}

interface MetricCardProps {
  label: string;
  value: string | number;
  icon?: LucideIcon;
  trend?: Trend;
  sparkline?: SparklinePoint[];
  href?: string;
  className?: string;
}

const trendIcons = {
  up: ArrowUp,
  down: ArrowDown,
  flat: ArrowRight,
} as const;

const trendColors = {
  up: "text-emerald-600 dark:text-emerald-400",
  down: "text-red-600 dark:text-red-400",
  flat: "text-muted-foreground",
} as const;

const sparklineColors = {
  up: { stroke: "#059669", fill: "#059669" },
  down: { stroke: "#dc2626", fill: "#dc2626" },
  flat: { stroke: "#64748b", fill: "#64748b" },
} as const;

export function MetricCard({
  label,
  value,
  icon: Icon,
  trend,
  sparkline,
  href,
  className,
}: MetricCardProps) {
  const Wrapper = href ? "a" : "div";
  const wrapperProps = href ? { href } : {};
  const sparklineTone = sparklineColors[trend?.direction ?? "flat"];

  return (
    <Wrapper
      {...wrapperProps}
      className={cn(
        "rounded-lg border bg-card p-[var(--space-card-padding)] transition-colors",
        href && "cursor-pointer hover:bg-accent/50",
        className
      )}
    >
      <div className="flex items-start justify-between gap-3">
        <span className="text-fluid-caption text-muted-foreground">{label}</span>
        {Icon && <Icon className="size-4 text-muted-foreground" />}
      </div>
      <div className="mt-[var(--space-stack-sm)] flex items-end justify-between gap-3">
        <div className="min-w-0">
          <div className="flex flex-wrap items-baseline gap-2">
            <span className="text-fluid-metric font-semibold tracking-tight">
              {value}
            </span>
            {trend && (
              <span
                className={cn(
                  "flex items-center gap-0.5 text-xs font-medium",
                  trendColors[trend.direction]
                )}
              >
                {(() => {
                  const TrendIcon = trendIcons[trend.direction];
                  return <TrendIcon className="size-3" />;
                })()}
                {trend.value}%
              </span>
            )}
          </div>
        </div>
        {sparkline && sparkline.length > 0 ? (
          <div
            aria-hidden="true"
            className="h-12 w-24 shrink-0 overflow-hidden rounded-md bg-muted/30 p-1"
          >
            <ResponsiveContainer width="100%" height="100%">
              <AreaChart data={sparkline}>
                <Area
                  type="monotone"
                  dataKey="value"
                  stroke={sparklineTone.stroke}
                  fill={sparklineTone.fill}
                  fillOpacity={0.18}
                  strokeWidth={2}
                  isAnimationActive={false}
                />
              </AreaChart>
            </ResponsiveContainer>
          </div>
        ) : null}
      </div>
    </Wrapper>
  );
}
