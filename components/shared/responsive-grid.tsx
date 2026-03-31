"use client";

import type * as React from "react";
import { useBreakpoint } from "@/hooks/use-breakpoint";
import type { Breakpoint } from "@/lib/responsive";
import { cn } from "@/lib/utils";

type ResponsiveColumns = number | Partial<Record<Breakpoint, number>>;

function resolveColumns(columns: ResponsiveColumns, breakpoint: Breakpoint) {
  if (typeof columns === "number") {
    return columns;
  }

  return columns[breakpoint] ?? columns.desktop ?? columns.tablet ?? columns.mobile ?? 1;
}

export interface ResponsiveGridProps
  extends React.HTMLAttributes<HTMLDivElement> {
  columns: ResponsiveColumns;
  gap?: number | string;
}

export function ResponsiveGrid({
  columns,
  gap = 16,
  className,
  style,
  children,
  ...props
}: ResponsiveGridProps) {
  const { breakpoint } = useBreakpoint();
  const resolvedColumns = resolveColumns(columns, breakpoint);

  return (
    <div
      data-breakpoint={breakpoint}
      className={cn("grid", className)}
      style={{
        gap: typeof gap === "number" ? `${gap}px` : gap,
        gridTemplateColumns: `repeat(${resolvedColumns}, minmax(0, 1fr))`,
        ...style,
      }}
      {...props}
    >
      {children}
    </div>
  );
}
