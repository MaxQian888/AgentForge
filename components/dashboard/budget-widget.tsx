"use client";

import { useTranslations } from "next-intl";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { cn } from "@/lib/utils";

interface BudgetWidgetProps {
  totalBudget: number;
  spent: number;
  remaining: number;
}

function formatUsd(value: number): string {
  return new Intl.NumberFormat("en-US", {
    style: "currency",
    currency: "USD",
  }).format(value);
}

function utilizationColor(percent: number): string {
  if (percent >= 90) return "bg-red-500";
  if (percent >= 70) return "bg-amber-500";
  return "bg-emerald-500";
}

export function BudgetWidget({ totalBudget, spent, remaining }: BudgetWidgetProps) {
  const t = useTranslations("dashboard");
  const utilization = totalBudget > 0 ? (spent / totalBudget) * 100 : 0;

  return (
    <Card>
      <CardHeader className="pb-3">
        <CardTitle className="text-sm font-medium">
          {t("budget.title")}
        </CardTitle>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="grid grid-cols-3 gap-2 text-center">
          <div>
            <p className="text-xs text-muted-foreground">{t("budget.total")}</p>
            <p className="text-sm font-semibold">{formatUsd(totalBudget)}</p>
          </div>
          <div>
            <p className="text-xs text-muted-foreground">{t("budget.spent")}</p>
            <p className="text-sm font-semibold">{formatUsd(spent)}</p>
          </div>
          <div>
            <p className="text-xs text-muted-foreground">{t("budget.remaining")}</p>
            <p className="text-sm font-semibold">{formatUsd(remaining)}</p>
          </div>
        </div>

        <div className="space-y-1">
          <div className="flex items-center justify-between text-xs text-muted-foreground">
            <span>{t("budget.utilization")}</span>
            <span>{utilization.toFixed(0)}%</span>
          </div>
          <div className="h-2 w-full rounded-full bg-muted">
            <div
              className={cn(
                "h-2 rounded-full transition-all",
                utilizationColor(utilization)
              )}
              style={{ width: `${Math.min(utilization, 100)}%` }}
            />
          </div>
        </div>
      </CardContent>
    </Card>
  );
}
