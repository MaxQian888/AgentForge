"use client";

import { Skeleton } from "@/components/ui/skeleton";
import { cn } from "@/lib/utils";

interface WorkspaceSkeletonProps {
  showSidebar?: boolean;
  showRail?: boolean;
  className?: string;
}

export function WorkspaceSkeleton({
  showSidebar = true,
  showRail = false,
  className,
}: WorkspaceSkeletonProps) {
  return (
    <div
      className={cn("flex h-full w-full gap-0", className)}
    >
      {showSidebar && (
        <div className="w-[260px] shrink-0 border-r p-4 space-y-3">
          <Skeleton className="h-8 w-full" />
          <div className="space-y-2">
            {Array.from({ length: 6 }).map((_, i) => (
              <Skeleton key={i} className="h-8 w-full rounded-md" />
            ))}
          </div>
        </div>
      )}
      <div className="flex-1 p-6 space-y-4">
        <div className="flex items-center gap-4">
          <Skeleton className="h-8 w-48" />
          <div className="flex-1" />
          <Skeleton className="h-8 w-24" />
        </div>
        <div className="space-y-3">
          {Array.from({ length: 4 }).map((_, i) => (
            <Skeleton key={i} className="h-20 w-full rounded-lg" />
          ))}
        </div>
      </div>
      {showRail && (
        <div className="w-[320px] shrink-0 border-l p-4 space-y-3">
          <Skeleton className="h-6 w-32" />
          <Skeleton className="h-32 w-full rounded-lg" />
          <Skeleton className="h-6 w-24" />
          <Skeleton className="h-20 w-full rounded-lg" />
        </div>
      )}
    </div>
  );
}
