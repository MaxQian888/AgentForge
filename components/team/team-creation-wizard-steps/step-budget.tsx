"use client";

import { useTranslations } from "next-intl";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";

export interface StepBudgetData {
  maxBudgetPerAgent: number;
  totalTeamBudget: number;
  autoStopOnExceed: boolean;
}

interface StepBudgetProps {
  data: StepBudgetData;
  onChange: (data: StepBudgetData) => void;
}

export function StepBudget({ data, onChange }: StepBudgetProps) {
  const t = useTranslations("teams");

  return (
    <div className="flex flex-col gap-5">
      <div className="flex flex-col gap-2">
        <Label htmlFor="budget-per-agent">{t("wizard.budgetPerAgent")}</Label>
        <div className="relative">
          <span className="absolute left-3 top-1/2 -translate-y-1/2 text-sm text-muted-foreground">
            $
          </span>
          <Input
            id="budget-per-agent"
            type="number"
            min={0}
            step={0.01}
            value={data.maxBudgetPerAgent || ""}
            onChange={(e) =>
              onChange({
                ...data,
                maxBudgetPerAgent: parseFloat(e.target.value) || 0,
              })
            }
            placeholder="0.00"
            className="pl-7"
          />
        </div>
      </div>

      <div className="flex flex-col gap-2">
        <Label htmlFor="budget-total">{t("wizard.budgetTotal")}</Label>
        <div className="relative">
          <span className="absolute left-3 top-1/2 -translate-y-1/2 text-sm text-muted-foreground">
            $
          </span>
          <Input
            id="budget-total"
            type="number"
            min={0}
            step={0.01}
            value={data.totalTeamBudget || ""}
            onChange={(e) =>
              onChange({
                ...data,
                totalTeamBudget: parseFloat(e.target.value) || 0,
              })
            }
            placeholder="0.00"
            className="pl-7"
          />
        </div>
      </div>

      <div className="flex items-center justify-between gap-4">
        <div className="flex flex-col gap-0.5">
          <Label htmlFor="auto-stop">{t("wizard.autoStop")}</Label>
          <p className="text-xs text-muted-foreground">
            {t("wizard.autoStopDesc")}
          </p>
        </div>
        <Switch
          id="auto-stop"
          checked={data.autoStopOnExceed}
          onCheckedChange={(checked) =>
            onChange({ ...data, autoStopOnExceed: !!checked })
          }
        />
      </div>
    </div>
  );
}
