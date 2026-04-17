"use client";

import { useTranslations } from "next-intl";
import { AlertTriangle, OctagonAlert } from "lucide-react";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";

export type OverspendingSeverity = "warning" | "critical";

export interface OverspendingAlert {
  /** Stable identifier (e.g. sprint or project id). */
  id: string;
  /** Human-readable scope, e.g. sprint name or "Project". */
  scope: string;
  severity: OverspendingSeverity;
  spentUsd: number;
  budgetUsd: number;
}

interface OverspendingAlertBannerProps {
  alerts: OverspendingAlert[];
  onAction?: (alert: OverspendingAlert) => void;
  className?: string;
}

/**
 * Derives overspending alerts from a list of scoped budget/spend pairs.
 * Critical when spend >= 100% of budget; warning when spend >= 80%.
 */
export function deriveOverspendingAlerts(
  items: Array<{
    id: string;
    scope: string;
    spentUsd: number;
    budgetUsd: number;
  }>,
): OverspendingAlert[] {
  const alerts: OverspendingAlert[] = [];
  for (const item of items) {
    if (item.budgetUsd <= 0) continue;
    const ratio = item.spentUsd / item.budgetUsd;
    if (ratio >= 1) {
      alerts.push({ ...item, severity: "critical" });
    } else if (ratio >= 0.8) {
      alerts.push({ ...item, severity: "warning" });
    }
  }
  return alerts;
}

export function OverspendingAlertBanner({
  alerts,
  onAction,
  className,
}: OverspendingAlertBannerProps) {
  const t = useTranslations("cost");
  if (alerts.length === 0) return null;

  return (
    <div className={cn("space-y-2", className)} data-testid="overspending-alerts">
      {alerts.map((alert) => {
        const isCritical = alert.severity === "critical";
        const percent = Math.round((alert.spentUsd / alert.budgetUsd) * 100);
        const Icon = isCritical ? OctagonAlert : AlertTriangle;
        return (
          <div
            key={alert.id}
            role="alert"
            className={cn(
              "flex items-center gap-3 rounded-lg border px-4 py-3 text-sm",
              isCritical
                ? "border-red-300 bg-red-50 text-red-900 dark:border-red-900/50 dark:bg-red-950/30 dark:text-red-200"
                : "border-amber-300 bg-amber-50 text-amber-900 dark:border-amber-900/50 dark:bg-amber-950/30 dark:text-amber-200",
            )}
          >
            <Icon className="size-4 shrink-0" aria-hidden />
            <div className="flex-1">
              <p className="font-medium">
                {isCritical
                  ? t("alertCriticalTitle", { scope: alert.scope })
                  : t("alertWarningTitle", { scope: alert.scope })}
              </p>
              <p className="text-xs opacity-90">
                {t("alertDetail", {
                  spent: `$${alert.spentUsd.toFixed(2)}`,
                  budget: `$${alert.budgetUsd.toFixed(2)}`,
                  percent: percent.toString(),
                })}
              </p>
            </div>
            {onAction ? (
              <Button
                type="button"
                size="sm"
                variant={isCritical ? "destructive" : "outline"}
                onClick={() => onAction(alert)}
              >
                {t("alertAdjustBudget")}
              </Button>
            ) : null}
          </div>
        );
      })}
    </div>
  );
}
