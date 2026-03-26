"use client";

export function EditorLoadingSkeleton() {
  return (
    <div className="flex min-h-[360px] flex-col gap-4 rounded-xl border border-border/60 bg-card/60 p-6">
      <div className="h-6 w-40 animate-pulse rounded bg-muted" />
      <div className="h-4 w-64 animate-pulse rounded bg-muted" />
      <div className="h-4 w-full animate-pulse rounded bg-muted" />
      <div className="h-4 w-[92%] animate-pulse rounded bg-muted" />
      <div className="h-4 w-[88%] animate-pulse rounded bg-muted" />
      <div className="mt-6 h-32 animate-pulse rounded-xl bg-muted/80" />
    </div>
  );
}
