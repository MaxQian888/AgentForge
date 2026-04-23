"use client";

import { Skeleton } from "@/components/ui/skeleton";

export function EditorLoadingSkeleton() {
  return (
    <div className="flex min-h-[360px] flex-col gap-4 rounded-xl border border-border/60 bg-card/60 p-6">
      <Skeleton className="h-6 w-40 bg-muted" />
      <Skeleton className="h-4 w-64 bg-muted" />
      <Skeleton className="h-4 w-full bg-muted" />
      <Skeleton className="h-4 w-[92%] bg-muted" />
      <Skeleton className="h-4 w-[88%] bg-muted" />
      <Skeleton className="mt-6 h-32 rounded-xl bg-muted/80" />
    </div>
  );
}
