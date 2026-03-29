"use client";

import { useTranslations } from "next-intl";
import { Badge } from "@/components/ui/badge";
import type { DispatchAttemptRecord } from "@/lib/stores/agent-store";

interface DispatchHistoryPanelProps {
  attempts: DispatchAttemptRecord[];
}

export function DispatchHistoryPanel({ attempts }: DispatchHistoryPanelProps) {
  const t = useTranslations("tasks");

  return (
    <div className="rounded-lg border border-border/60 bg-muted/20 p-3 text-sm">
      <div className="font-medium">{t("detail.dispatchHistoryTitle")}</div>
      <div className="mt-1 text-muted-foreground">
        {t("detail.dispatchHistoryDescription")}
      </div>

      {attempts.length === 0 ? (
        <div className="mt-3 text-muted-foreground">
          {t("detail.dispatchHistoryEmpty")}
        </div>
      ) : (
        <div className="mt-3 space-y-2">
          {attempts.map((attempt) => (
            <div
              key={attempt.id}
              className="rounded-md border border-border/60 bg-background px-3 py-2"
            >
              <div className="flex flex-wrap items-center gap-2">
                <Badge variant="secondary">
                  {t(`detail.dispatchHint.${attempt.outcome}`)}
                </Badge>
                <Badge variant="outline">
                  {t("detail.dispatchTrigger", {
                    trigger: attempt.triggerSource,
                  })}
                </Badge>
                {attempt.guardrailType ? (
                  <Badge variant="outline">
                    {t(`detail.dispatchGuardrail.${attempt.guardrailType}`)}
                  </Badge>
                ) : null}
              </div>
              <div className="mt-2 text-xs text-muted-foreground">
                {new Date(attempt.createdAt).toLocaleString()}
              </div>
              {attempt.reason ? (
                <div className="mt-1 text-muted-foreground">{attempt.reason}</div>
              ) : null}
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
