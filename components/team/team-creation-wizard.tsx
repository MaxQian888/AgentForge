"use client";

import { useCallback, useState } from "react";
import { useTranslations } from "next-intl";
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet";
import { Button } from "@/components/ui/button";
import { useTeamStore } from "@/lib/stores/team-store";
import { cn } from "@/lib/utils";

import { StepName, type StepNameData } from "./team-creation-wizard-steps/step-name";
import { StepRoles } from "./team-creation-wizard-steps/step-roles";
import { StepAgents } from "./team-creation-wizard-steps/step-agents";
import {
  StepStrategy,
  type StrategyType,
} from "./team-creation-wizard-steps/step-strategy";
import {
  StepBudget,
  type StepBudgetData,
} from "./team-creation-wizard-steps/step-budget";
import { StepReview } from "./team-creation-wizard-steps/step-review";

const TOTAL_STEPS = 6;

const STEP_KEYS = [
  "nameGoal",
  "selectRoles",
  "assignAgents",
  "strategy",
  "budget",
  "review",
] as const;

interface TeamCreationWizardProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

export function TeamCreationWizard({
  open,
  onOpenChange,
}: TeamCreationWizardProps) {
  const t = useTranslations("teams");
  const startTeam = useTeamStore((s) => s.startTeam);

  const [step, setStep] = useState(0);
  const [launching, setLaunching] = useState(false);

  // Step 1 data
  const [nameData, setNameData] = useState<StepNameData>({
    name: "",
    description: "",
    objective: "",
  });

  // Step 2 data
  const [selectedRoleIds, setSelectedRoleIds] = useState<string[]>([]);

  // Step 3 data
  const [agentAssignments, setAgentAssignments] = useState<
    Record<string, string>
  >({});

  // Step 4 data
  const [strategy, setStrategy] = useState<StrategyType>("sequential");

  // Step 5 data
  const [budgetData, setBudgetData] = useState<StepBudgetData>({
    maxBudgetPerAgent: 0,
    totalTeamBudget: 0,
    autoStopOnExceed: true,
  });

  // Track highest completed step for navigation
  const [highestVisited, setHighestVisited] = useState(0);

  const canProceed = useCallback(() => {
    switch (step) {
      case 0:
        return nameData.name.trim().length > 0;
      case 1:
        return selectedRoleIds.length > 0;
      default:
        return true;
    }
  }, [step, nameData.name, selectedRoleIds.length]);

  const goNext = () => {
    if (step < TOTAL_STEPS - 1 && canProceed()) {
      const next = step + 1;
      setStep(next);
      setHighestVisited((prev) => Math.max(prev, next));
    }
  };

  const goPrev = () => {
    if (step > 0) {
      setStep(step - 1);
    }
  };

  const goToStep = (target: number) => {
    if (target <= highestVisited) {
      setStep(target);
    }
  };

  const handleLaunch = async () => {
    if (!nameData.name.trim()) return;
    setLaunching(true);
    try {
      // Map strategy types to the API's strategy values
      const strategyMap: Record<StrategyType, string> = {
        sequential: "pipeline",
        parallel: "swarm",
        hybrid: "plan-code-review",
      };

      await startTeam("", "", {
        strategy: strategyMap[strategy],
        totalBudgetUsd: budgetData.totalTeamBudget || undefined,
      });

      // Reset and close
      resetWizard();
      onOpenChange(false);
    } catch {
      // Error handled by store
    } finally {
      setLaunching(false);
    }
  };

  const resetWizard = () => {
    setStep(0);
    setHighestVisited(0);
    setNameData({ name: "", description: "", objective: "" });
    setSelectedRoleIds([]);
    setAgentAssignments({});
    setStrategy("sequential");
    setBudgetData({
      maxBudgetPerAgent: 0,
      totalTeamBudget: 0,
      autoStopOnExceed: true,
    });
  };

  const handleOpenChange = (value: boolean) => {
    if (!value) {
      resetWizard();
    }
    onOpenChange(value);
  };

  return (
    <Sheet open={open} onOpenChange={handleOpenChange}>
      <SheetContent
        side="right"
        className="flex w-full flex-col sm:max-w-lg"
        showCloseButton
      >
        <SheetHeader>
          <SheetTitle>{t("wizard.title")}</SheetTitle>
          <p className="text-sm text-muted-foreground">
            {t(`wizard.step.${STEP_KEYS[step]}`)}
          </p>
        </SheetHeader>

        {/* Step indicator */}
        <div className="flex items-center gap-1.5 px-4">
          {Array.from({ length: TOTAL_STEPS }).map((_, i) => (
            <button
              key={i}
              type="button"
              onClick={() => goToStep(i)}
              disabled={i > highestVisited}
              aria-label={t(`wizard.step.${STEP_KEYS[i]}`)}
              className={cn(
                "h-1.5 flex-1 rounded-full transition-colors",
                i === step
                  ? "bg-primary"
                  : i <= highestVisited
                    ? "bg-primary/40 hover:bg-primary/60"
                    : "bg-muted"
              )}
            />
          ))}
        </div>

        {/* Step content */}
        <div className="flex-1 overflow-y-auto px-4">
          {step === 0 && <StepName data={nameData} onChange={setNameData} />}
          {step === 1 && (
            <StepRoles
              selectedRoleIds={selectedRoleIds}
              onChange={setSelectedRoleIds}
            />
          )}
          {step === 2 && (
            <StepAgents
              selectedRoleIds={selectedRoleIds}
              agentAssignments={agentAssignments}
              onChange={setAgentAssignments}
            />
          )}
          {step === 3 && (
            <StepStrategy strategy={strategy} onChange={setStrategy} />
          )}
          {step === 4 && (
            <StepBudget data={budgetData} onChange={setBudgetData} />
          )}
          {step === 5 && (
            <StepReview
              nameData={nameData}
              selectedRoleIds={selectedRoleIds}
              agentAssignments={agentAssignments}
              strategy={strategy}
              budgetData={budgetData}
            />
          )}
        </div>

        {/* Navigation */}
        <div className="flex items-center justify-between border-t px-4 py-3">
          <Button
            variant="outline"
            onClick={goPrev}
            disabled={step === 0}
          >
            {t("wizard.previous")}
          </Button>

          <span className="text-xs text-muted-foreground">
            {t("wizard.stepOf", { current: step + 1, total: TOTAL_STEPS })}
          </span>

          {step === TOTAL_STEPS - 1 ? (
            <Button onClick={handleLaunch} disabled={launching}>
              {launching ? t("wizard.launching") : t("wizard.launch")}
            </Button>
          ) : (
            <Button onClick={goNext} disabled={!canProceed()}>
              {t("wizard.next")}
            </Button>
          )}
        </div>
      </SheetContent>
    </Sheet>
  );
}
