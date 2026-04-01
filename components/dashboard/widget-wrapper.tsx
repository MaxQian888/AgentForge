"use client";

import { ReactNode } from "react";
import { useTranslations } from "next-intl";
import { Button } from "@/components/ui/button";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";

export function WidgetWrapper({
  title,
  children,
  state = "ready",
  message,
  onRefresh,
  onConfigure,
  onRemove,
  onRetry,
  autoRefresh,
}: {
  title: string;
  children?: ReactNode;
  state?: "ready" | "saving" | "error" | "empty";
  message?: string;
  onRefresh?: () => void;
  onConfigure?: () => void;
  onRemove?: () => void;
  onRetry?: () => void;
  autoRefresh?: {
    interval: "30s" | "60s" | "300s" | "off";
    paused: boolean;
    lastUpdatedLabel?: string;
    onPauseToggle: () => void;
    onIntervalChange: (interval: "30s" | "60s" | "300s" | "off") => void;
  };
}) {
  const t = useTranslations("dashboard");

  return (
    <div className="rounded-lg border bg-card p-4 shadow-sm">
      <div className="mb-3 flex items-start justify-between gap-3">
        <div className="space-y-1">
          <div className="text-sm font-medium">{title}</div>
          {autoRefresh?.lastUpdatedLabel ? (
            <div className="text-xs text-muted-foreground">
              {autoRefresh.lastUpdatedLabel}
            </div>
          ) : null}
        </div>
        <div className="flex flex-wrap justify-end gap-2">
          {autoRefresh ? (
            <>
              <Select
                value={autoRefresh.interval}
                onValueChange={(value) =>
                  autoRefresh.onIntervalChange(
                    value as "30s" | "60s" | "300s" | "off"
                  )
                }
              >
                <SelectTrigger className="h-8 px-2 text-xs" aria-label={t("widget.autoRefresh.label")}>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="30s">{t("widget.autoRefresh.interval.30s")}</SelectItem>
                  <SelectItem value="60s">{t("widget.autoRefresh.interval.60s")}</SelectItem>
                  <SelectItem value="300s">{t("widget.autoRefresh.interval.300s")}</SelectItem>
                  <SelectItem value="off">{t("widget.autoRefresh.interval.off")}</SelectItem>
                </SelectContent>
              </Select>
              <Button
                type="button"
                size="sm"
                variant="outline"
                onClick={autoRefresh.onPauseToggle}
              >
                {autoRefresh.paused
                  ? t("widget.autoRefresh.resume")
                  : t("widget.autoRefresh.pause")}
              </Button>
            </>
          ) : null}
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
