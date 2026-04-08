"use client";

import { useMemo } from "react";
import { useTranslations } from "next-intl";
import Link from "next/link";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { getSettingsFallbackState } from "@/lib/settings/project-settings-workspace";
import type { ProjectSectionProps } from "./types";

export function SectionAdvanced({ draft, project }: ProjectSectionProps) {
  const t = useTranslations("settings");

  const runtimeOptions = project.codingAgentCatalog?.runtimes ?? [];
  const selectedRuntime =
    runtimeOptions.find((o) => o.runtime === draft.settings.codingAgent.runtime) ?? runtimeOptions[0];
  const fallbackState = getSettingsFallbackState(project);
  const hasFallbackDefaults = fallbackState.budgetGovernance || fallbackState.reviewPolicy || fallbackState.webhook;

  const webhookSummary = useMemo(() => {
    if (!draft.settings.webhook.active) return t("webhookInactiveSummary");
    if (!draft.settings.webhook.url.trim()) return t("webhookMissingUrlSummary");
    if (draft.settings.webhook.events.length === 0) return t("webhookMissingEventsSummary");
    return t("webhookReadySummary");
  }, [draft.settings.webhook.active, draft.settings.webhook.events.length, draft.settings.webhook.url, t]);

  return (
    <div className="space-y-6">
      <div>
        <h2 className="text-lg font-semibold">{t("operatorDiagnostics")}</h2>
        <p className="text-sm text-muted-foreground">{t("operatorDiagnosticsDesc")}</p>
      </div>

      {hasFallbackDefaults && (
        <div className="rounded-md border border-amber-500/40 bg-amber-500/10 p-3 text-sm">
          {t("fallbackGovernanceDefaults")}
        </div>
      )}

      <Card>
        <CardContent className="space-y-4 pt-6">
          <div className="grid gap-3 md:grid-cols-3">
            <div className="rounded-md border p-3 text-sm">
              <p className="font-medium">{t("runtimeSummaryTitle")}</p>
              <p className="mt-1 text-muted-foreground">
                {(selectedRuntime?.label ?? draft.settings.codingAgent.runtime) || t("noRuntimeInfo")}
              </p>
              <p className="mt-2">
                {selectedRuntime?.available ? t("runtimeReadySummary") : t("runtimeBlockedSummary")}
              </p>
            </div>
            <div className="rounded-md border p-3 text-sm">
              <p className="font-medium">{t("reviewSummaryTitle")}</p>
              <p className="mt-1 text-muted-foreground">
                {draft.settings.reviewPolicy.requireManualApproval
                  ? t("reviewManualApprovalEnabled")
                  : t("reviewManualApprovalDisabled")}
              </p>
              <p className="mt-2">
                {t("reviewRiskSummary", {
                  risk: draft.settings.reviewPolicy.minRiskLevelForBlock || t("disabled"),
                })}
              </p>
            </div>
            <div className="rounded-md border p-3 text-sm">
              <p className="font-medium">{t("webhookSummaryTitle")}</p>
              <p className="mt-1 text-muted-foreground">{webhookSummary}</p>
              <p className="mt-2">
                {t("webhookEventCountSummary", { count: draft.settings.webhook.events.length })}
              </p>
            </div>
          </div>

          <div className="grid gap-3 md:grid-cols-2">
            {runtimeOptions.map((o) => (
              <div key={o.runtime} className="flex items-center justify-between rounded-md border p-3">
                <span className="text-sm font-medium">{o.label}</span>
                <Badge variant={o.available ? "default" : "secondary"}>
                  {o.available ? t("runtimeReady") : t("runtimeUnavailable")}
                </Badge>
              </div>
            ))}
            {runtimeOptions.length === 0 && (
              <p className="text-sm text-muted-foreground">{t("noRuntimeInfo")}</p>
            )}
          </div>

          <div className="flex gap-3">
            <Button asChild size="sm" variant="outline">
              <Link href="/agents">{t("viewAgentPool")}</Link>
            </Button>
            <Button asChild size="sm" variant="outline">
              <Link href="/reviews">{t("viewReviewBacklog")}</Link>
            </Button>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
