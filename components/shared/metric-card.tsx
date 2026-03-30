"use client";

import type { LucideIcon } from "lucide-react";
import { ArrowDown, ArrowRight, ArrowUp } from "lucide-react";
import { cn } from "@/lib/utils";

interface Trend {
  value: number;
  direction: "up" | "down" | "flat";
}

interface MetricCardProps {
  label: string;
  value: string | number;
  icon?: LucideIcon;
  trend?: Trend;
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

export function MetricCard({
  label,
  value,
  icon: Icon,
  trend,
  href,
  className,
}: MetricCardProps) {
  const Wrapper = href ? "a" : "div";
  const wrapperProps = href ? { href } : {};

  return (
    <Wrapper
      {...wrapperProps}
      className={cn(
        "rounded-lg border bg-card p-4 transition-colors",
        href && "cursor-pointer hover:bg-accent/50",
        className
      )}
    >
      <div className="flex items-center justify-between">
        <span className="text-[13px] text-muted-foreground">{label}</span>
        {Icon && <Icon className="size-4 text-muted-foreground" />}
      </div>
      <div className="mt-1 flex items-baseline gap-2">
        <span className="text-xl font-semibold tracking-tight">{value}</span>
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
    </Wrapper>
  );
}
