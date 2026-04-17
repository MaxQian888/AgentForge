"use client";

import { useMemo } from "react";
import { useTranslations } from "next-intl";
import { AlertTriangle, TrendingUp } from "lucide-react";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { cn } from "@/lib/utils";

export interface BudgetForecastInput {
  /** Daily cost points for the current period, ordered ascending by date. */
  dailyCosts: Array<{ date: string; costUsd: number }>;
  /** Total allocated budget for the period (USD). 0 or null => no budget set. */
  budgetUsd: number | null;
  /** Total spend so far (USD). */
  spentUsd: number;
  /** Days remaining in the period. */
  daysRemaining: number;
}

interface BudgetForecastCardProps {
  input: BudgetForecastInput;
  className?: string;
}

function computeBurnRate(
  dailyCosts: BudgetForecastInput["dailyCosts"],
): number {
  if (dailyCosts.length === 0) return 0;
  // Use last 7 days (or all, if fewer) for stability
  const window = dailyCosts.slice(-7);
  const total = window.reduce((sum, p) => sum + p.costUsd, 0);
  return total / window.length;
}

export function BudgetForecastCard({
  input,
  className,
}: BudgetForecastCardProps) {
  const t = useTranslations("cost");

  const { projectedSpend, burnRate, willExceed, exhaustionDays, hasBudget } =
    useMemo(() => {
      const burn = computeBurnRate(input.dailyCosts);
      const daysAhead = Math.max(0, input.daysRemaining);
      const projected = input.spentUsd + burn * daysAhead;
      const budget = input.budgetUsd ?? 0;
      const hasBudgetConfigured = budget > 0;
      const exceeds = hasBudgetConfigured && projected > budget;
      let exhaustion: number | null = null;
      if (hasBudgetConfigured && burn > 0) {
        const remainingBudget = budget - input.spentUsd;
        if (remainingBudget <= 0) {
          exhaustion = 0;
        } else {
          exhaustion = remainingBudget / burn;
        }
      }
      return {
        projectedSpend: projected,
        burnRate: burn,
        willExceed: exceeds,
        exhaustionDays: exhaustion,
        hasBudget: hasBudgetConfigured,
      };
    }, [input]);

  return (
    <Card className={cn(className)} data-testid="budget-forecast-card">
      <CardHeader className="flex flex-row items-center justify-between space-y-0">
        <div className="space-y-1">
          <CardTitle className="flex items-center gap-2 text-base">
            {willExceed ? (
              <AlertTriangle className="size-4 text-destructive" aria-hidden />
            ) : (
              <TrendingUp className="size-4 text-muted-foreground" aria-hidden />
            )}
            {t("forecastTitle")}
          </CardTitle>
          <CardDescription>{t("forecastDescription")}</CardDescription>
        </div>
        {willExceed ? (
          <Badge variant="destructive">{t("forecastOverBudget")}</Badge>
        ) : hasBudget ? (
          <Badge variant="secondary">{t("forecastOnTrack")}</Badge>
        ) : null}
      </CardHeader>
      <CardContent className="space-y-3">
        <div className="grid gap-3 sm:grid-cols-3">
          <div>
            <p className="text-xs text-muted-foreground">
              {t("forecastProjected")}
            </p>
            <p className="text-xl font-semibold">
              ${projectedSpend.toFixed(2)}
            </p>
          </div>
          <div>
            <p className="text-xs text-muted-foreground">
              {t("forecastBurnRate")}
            </p>
            <p className="text-xl font-semibold">
              ${burnRate.toFixed(2)}
              <span className="ml-1 text-xs font-normal text-muted-foreground">
                {t("forecastPerDay")}
              </span>
            </p>
          </div>
          <div>
            <p className="text-xs text-muted-foreground">
              {t("forecastBudget")}
            </p>
            <p className="text-xl font-semibold">
              {hasBudget ? `$${(input.budgetUsd ?? 0).toFixed(2)}` : "—"}
            </p>
          </div>
        </div>
        {willExceed && exhaustionDays !== null ? (
          <p className="text-sm text-destructive">
            {t("forecastExhaustion", {
              days: Math.max(0, Math.round(exhaustionDays)).toString(),
            })}
          </p>
        ) : null}
        <p className="text-xs text-muted-foreground">
          {t("forecastDisclaimer")}
        </p>
      </CardContent>
    </Card>
  );
}
