"use client";

import { ReactNode } from "react";
import { Button } from "@/components/ui/button";

export function WidgetWrapper({
  title,
  children,
  onRefresh,
}: {
  title: string;
  children: ReactNode;
  onRefresh?: () => void;
}) {
  return (
    <div className="rounded-lg border bg-card p-4 shadow-sm">
      <div className="mb-3 flex items-center justify-between gap-3">
        <div className="text-sm font-medium">{title}</div>
        {onRefresh ? (
          <Button type="button" size="sm" variant="outline" onClick={onRefresh}>
            Refresh
          </Button>
        ) : null}
      </div>
      {children}
    </div>
  );
}
