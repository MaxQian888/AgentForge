"use client";

import { useTranslations } from "next-intl";
import { cn } from "@/lib/utils";

export type StrategyType = "sequential" | "parallel" | "hybrid";

interface StepStrategyProps {
  strategy: StrategyType;
  onChange: (strategy: StrategyType) => void;
}

const STRATEGIES: StrategyType[] = ["sequential", "parallel", "hybrid"];

export function StepStrategy({ strategy, onChange }: StepStrategyProps) {
  const t = useTranslations("teams");

  return (
    <div className="flex flex-col gap-3">
      <p className="text-sm text-muted-foreground">{t("wizard.strategyHint")}</p>
      <div className="flex flex-col gap-2">
        {STRATEGIES.map((s) => (
          <button
            key={s}
            type="button"
            onClick={() => onChange(s)}
            className={cn(
              "flex flex-col gap-1 rounded-md border p-3 text-left transition-colors",
              strategy === s
                ? "border-primary bg-primary/5"
                : "border-border hover:border-muted-foreground/40"
            )}
          >
            <span className="flex items-center gap-2">
              <span
                className={cn(
                  "flex size-4 items-center justify-center rounded-full border-2",
                  strategy === s
                    ? "border-primary"
                    : "border-muted-foreground/40"
                )}
              >
                {strategy === s && (
                  <span className="size-2 rounded-full bg-primary" />
                )}
              </span>
              <span className="text-sm font-medium">
                {t(`wizard.strategy.${s}`)}
              </span>
            </span>
            <span className="ml-6 text-xs text-muted-foreground">
              {t(`wizard.strategy.${s}Desc`)}
            </span>
          </button>
        ))}
      </div>
    </div>
  );
}
