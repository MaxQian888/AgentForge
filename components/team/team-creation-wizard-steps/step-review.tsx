"use client";

import { useTranslations } from "next-intl";
import { Card, CardContent } from "@/components/ui/card";
import { useRoleStore } from "@/lib/stores/role-store";
import type { StepNameData } from "./step-name";
import type { StepBudgetData } from "./step-budget";
import type { StrategyType } from "./step-strategy";
import { AUTO_ASSIGN } from "./step-agents";

interface StepReviewProps {
  nameData: StepNameData;
  selectedRoleIds: string[];
  agentAssignments: Record<string, string>;
  strategy: StrategyType;
  budgetData: StepBudgetData;
}

export function StepReview({
  nameData,
  selectedRoleIds,
  agentAssignments,
  strategy,
  budgetData,
}: StepReviewProps) {
  const t = useTranslations("teams");
  const roles = useRoleStore((s) => s.roles);

  const selectedRoles = roles.filter((r) =>
    selectedRoleIds.includes(r.metadata.id)
  );

  const assignedCount = Object.values(agentAssignments).filter(
    (v) => v && v !== AUTO_ASSIGN
  ).length;
  const autoCount = selectedRoleIds.length - assignedCount;

  return (
    <div className="flex flex-col gap-4">
      <p className="text-sm text-muted-foreground">{t("wizard.reviewHint")}</p>

      <Card>
        <CardContent className="flex flex-col gap-3 p-4">
          <div>
            <p className="text-xs font-medium uppercase text-muted-foreground">
              {t("wizard.nameLabel")}
            </p>
            <p className="text-sm font-medium">{nameData.name}</p>
          </div>

          {nameData.description && (
            <div>
              <p className="text-xs font-medium uppercase text-muted-foreground">
                {t("wizard.descriptionLabel")}
              </p>
              <p className="text-sm">{nameData.description}</p>
            </div>
          )}

          {nameData.objective && (
            <div>
              <p className="text-xs font-medium uppercase text-muted-foreground">
                {t("wizard.objectiveLabel")}
              </p>
              <p className="text-sm">{nameData.objective}</p>
            </div>
          )}

          <div>
            <p className="text-xs font-medium uppercase text-muted-foreground">
              {t("wizard.rolesLabel")}
            </p>
            <p className="text-sm">
              {selectedRoles.map((r) => r.metadata.name).join(", ") ||
                t("wizard.noneSelected")}
            </p>
          </div>

          <div>
            <p className="text-xs font-medium uppercase text-muted-foreground">
              {t("wizard.agentsLabel")}
            </p>
            <p className="text-sm">
              {assignedCount > 0 &&
                t("wizard.manuallyAssigned", { count: assignedCount })}
              {assignedCount > 0 && autoCount > 0 && ", "}
              {autoCount > 0 &&
                t("wizard.autoAssigned", { count: autoCount })}
              {assignedCount === 0 && autoCount === 0 && t("wizard.noneSelected")}
            </p>
          </div>

          <div>
            <p className="text-xs font-medium uppercase text-muted-foreground">
              {t("wizard.strategyLabel")}
            </p>
            <p className="text-sm">{t(`wizard.strategy.${strategy}`)}</p>
          </div>

          <div>
            <p className="text-xs font-medium uppercase text-muted-foreground">
              {t("wizard.budgetLabel")}
            </p>
            <p className="text-sm">
              {t("wizard.budgetSummary", {
                perAgent: budgetData.maxBudgetPerAgent.toFixed(2),
                total: budgetData.totalTeamBudget.toFixed(2),
              })}
            </p>
            {budgetData.autoStopOnExceed && (
              <p className="text-xs text-muted-foreground">
                {t("wizard.autoStopEnabled")}
              </p>
            )}
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
