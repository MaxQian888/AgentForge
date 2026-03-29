"use client";

import { useTranslations } from "next-intl";
import { Badge } from "@/components/ui/badge";
import { ConfirmDialog } from "@/components/shared/confirm-dialog";
import type { DispatchPreflightSummary } from "@/lib/stores/agent-store";

interface DispatchPreflightDialogProps {
  open: boolean;
  taskTitle: string;
  memberName: string;
  summary: DispatchPreflightSummary | null;
  onConfirm: () => void;
  onCancel: () => void;
}

export function DispatchPreflightDialog({
  open,
  taskTitle,
  memberName,
  summary,
  onConfirm,
  onCancel,
}: DispatchPreflightDialogProps) {
  const t = useTranslations("tasks");

  return (
    <ConfirmDialog
      open={open}
      title={t("detail.dispatchPreflightTitle", { name: memberName })}
      description={
        <div className="space-y-3 text-left">
          <p>{t("detail.dispatchPreflightDescription", { task: taskTitle })}</p>
          <div className="flex flex-wrap gap-2">
            <Badge variant="secondary">
              {t(`detail.dispatchHint.${summary?.dispatchOutcomeHint ?? "started"}`)}
            </Badge>
            {summary?.admissionLikely ? (
              <Badge variant="secondary">{t("detail.dispatchLikely")}</Badge>
            ) : (
              <Badge variant="outline">{t("detail.dispatchUncertain")}</Badge>
            )}
          </div>
          <div className="grid gap-1 text-sm text-muted-foreground">
            {summary?.budgetBlocked ? (
              <div>
                {t("detail.dispatchBudgetBlocked", {
                  scope: t(`detail.dispatchScope.${summary.budgetBlocked.scope || "project"}`),
                  message: summary.budgetBlocked.message,
                })}
              </div>
            ) : null}
            {summary?.budgetWarning ? (
              <div>
                {t("detail.dispatchBudgetWarning", {
                  scope: t(`detail.dispatchScope.${summary.budgetWarning.scope || "project"}`),
                  message: summary.budgetWarning.message,
                })}
              </div>
            ) : null}
            {typeof summary?.poolActive === "number" ||
            typeof summary?.poolAvailable === "number" ||
            typeof summary?.poolQueued === "number" ? (
              <div>
                {t("detail.dispatchPoolSnapshot", {
                  active: summary?.poolActive ?? 0,
                  available: summary?.poolAvailable ?? 0,
                  queued: summary?.poolQueued ?? 0,
                })}
              </div>
            ) : null}
          </div>
        </div>
      }
      confirmLabel={t("detail.dispatchPreflightConfirm")}
      onConfirm={onConfirm}
      onCancel={onCancel}
    />
  );
}
