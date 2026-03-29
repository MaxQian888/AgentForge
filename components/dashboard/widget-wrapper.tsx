"use client";

import { ReactNode } from "react";
import { useTranslations } from "next-intl";
import { Button } from "@/components/ui/button";

export function WidgetWrapper({
  title,
  children,
  state = "ready",
  message,
  onRefresh,
  onConfigure,
  onRemove,
  onRetry,
}: {
  title: string;
  children?: ReactNode;
  state?: "ready" | "saving" | "error" | "empty";
  message?: string;
  onRefresh?: () => void;
  onConfigure?: () => void;
  onRemove?: () => void;
  onRetry?: () => void;
}) {
  const t = useTranslations("dashboard");

  return (
    <div className="rounded-lg border bg-card p-4 shadow-sm">
      <div className="mb-3 flex items-start justify-between gap-3">
        <div className="text-sm font-medium">{title}</div>
        <div className="flex flex-wrap justify-end gap-2">
          {onRefresh ? (
            <Button type="button" size="sm" variant="outline" onClick={onRefresh}>
              {t("widget.refresh")}
            </Button>
          ) : null}
          {onConfigure ? (
            <Button
              type="button"
              size="sm"
              variant="outline"
              onClick={onConfigure}
            >
              {t("widget.configure")}
            </Button>
          ) : null}
          {onRemove ? (
            <Button type="button" size="sm" variant="outline" onClick={onRemove}>
              {t("widget.remove")}
            </Button>
          ) : null}
        </div>
      </div>
      {state === "error" ? (
        <div className="space-y-3">
          <div className="text-sm text-destructive">
            {message ?? t("widget.errorFallback")}
          </div>
          {onRetry ? (
            <Button type="button" size="sm" variant="outline" onClick={onRetry}>
              {t("widget.retry")}
            </Button>
          ) : null}
        </div>
      ) : state === "empty" ? (
        <div className="text-sm text-muted-foreground">
          {message ?? t("widget.emptyFallback")}
        </div>
      ) : (
        <>
          {children}
          {state === "saving" ? (
            <div className="mt-3 text-xs text-muted-foreground">
              {message}
            </div>
          ) : null}
        </>
      )}
    </div>
  );
}
