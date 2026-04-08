"use client";

import { useTranslations } from "next-intl";
import { Card, CardContent } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { FieldError } from "@/components/shared/field-error";
import type { ProjectSectionProps } from "./types";

export function SectionBudget({ draft, patchDraft, validationErrors, clearValidationError }: ProjectSectionProps) {
  const t = useTranslations("settings");

  const patchBudget = (field: string, value: number | boolean) => {
    patchDraft((d) => ({
      ...d,
      settings: {
        ...d.settings,
        budgetGovernance: { ...d.settings.budgetGovernance, [field]: value },
      },
    }));
  };

  return (
    <div className="space-y-6">
      <div>
        <h2 className="text-lg font-semibold">{t("budgetGovernance")}</h2>
        <p className="text-sm text-muted-foreground">{t("budgetGovernanceDesc")}</p>
      </div>

      <Card>
        <CardContent className="space-y-4 pt-6">
          <div className="grid gap-4 md:grid-cols-2">
            <div className="flex flex-col gap-2">
              <Label htmlFor="settings-max-task-budget">{t("maxTaskBudget")}</Label>
              <Input
                id="settings-max-task-budget"
                type="number"
                min={0}
                step={0.01}
                aria-invalid={Boolean(validationErrors.maxTaskBudgetUsd)}
                value={draft.settings.budgetGovernance.maxTaskBudgetUsd}
                onChange={(e) => {
                  patchBudget("maxTaskBudgetUsd", Number(e.target.value));
                  clearValidationError("maxTaskBudgetUsd");
                }}
              />
              <FieldError message={validationErrors.maxTaskBudgetUsd} />
            </div>
            <div className="flex flex-col gap-2">
              <Label htmlFor="settings-max-daily-spend">{t("maxDailySpend")}</Label>
              <Input
                id="settings-max-daily-spend"
                type="number"
                min={0}
                step={0.01}
                aria-invalid={Boolean(validationErrors.maxDailySpendUsd)}
                value={draft.settings.budgetGovernance.maxDailySpendUsd}
                onChange={(e) => {
                  patchBudget("maxDailySpendUsd", Number(e.target.value));
                  clearValidationError("maxDailySpendUsd");
                }}
              />
              <FieldError message={validationErrors.maxDailySpendUsd} />
            </div>
          </div>
          <div className="grid gap-4 md:grid-cols-2">
            <div className="flex flex-col gap-2">
              <Label htmlFor="settings-alert-threshold">{t("alertThreshold")}</Label>
              <Input
                id="settings-alert-threshold"
                type="number"
                min={0}
                max={100}
                aria-invalid={Boolean(validationErrors.alertThresholdPercent)}
                value={draft.settings.budgetGovernance.alertThresholdPercent}
                onChange={(e) => {
                  patchBudget("alertThresholdPercent", Number(e.target.value));
                  clearValidationError("alertThresholdPercent");
                }}
              />
              <FieldError message={validationErrors.alertThresholdPercent} />
            </div>
            <div className="flex flex-col gap-2">
              <Label>{t("autoStopOnExceed")}</Label>
              <Select
                value={draft.settings.budgetGovernance.autoStopOnExceed ? "yes" : "no"}
                onValueChange={(v) => patchBudget("autoStopOnExceed", v === "yes")}
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="yes">{t("enabled")}</SelectItem>
                  <SelectItem value="no">{t("disabled")}</SelectItem>
                </SelectContent>
              </Select>
            </div>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
