"use client";

import { AlertCircle, RefreshCw } from "lucide-react";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";

interface ErrorBannerProps {
  message: string;
  onRetry?: () => void;
  className?: string;
}

export function ErrorBanner({ message, onRetry, className }: ErrorBannerProps) {
  return (
    <div
      className={cn(
        "flex items-center gap-3 rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm dark:border-red-900/50 dark:bg-red-950/30",
        className
      )}
    >
      <AlertCircle className="size-4 shrink-0 text-red-600 dark:text-red-400" />
      <span className="flex-1 text-red-800 dark:text-red-300">{message}</span>
      {onRetry && (
        <Button
          variant="ghost"
          size="sm"
          className="h-7 gap-1.5 text-xs text-red-700 hover:text-red-800 dark:text-red-400 dark:hover:text-red-300"
          onClick={onRetry}
        >
          <RefreshCw className="size-3" />
          Retry
        </Button>
      )}
    </div>
  );
}
