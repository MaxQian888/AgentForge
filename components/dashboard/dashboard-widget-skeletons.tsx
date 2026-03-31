"use client";

import { Skeleton } from "@/components/ui/skeleton";

function WidgetSkeletonCard() {
  return (
    <div
      data-testid="dashboard-widget-skeleton"
      className="rounded-lg border bg-card p-4"
    >
      <div className="flex items-center justify-between">
        <Skeleton className="h-5 w-32" />
        <Skeleton className="size-4 rounded" />
      </div>
      <div className="mt-4 space-y-3">
        <Skeleton className="h-4 w-full" />
        <Skeleton className="h-4 w-5/6" />
        <Skeleton className="h-24 w-full rounded-md" />
      </div>
    </div>
  );
}

export function DashboardWidgetsSkeleton() {
  return (
    <div className="grid gap-4 lg:grid-cols-2">
      {Array.from({ length: 4 }).map((_, index) => (
        <WidgetSkeletonCard key={index} />
      ))}
    </div>
  );
}
