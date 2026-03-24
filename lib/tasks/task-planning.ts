export type PlanningNormalizationResult =
  | {
      kind: "unscheduled";
    }
  | {
      kind: "scheduled";
      plannedStartAt: string;
      plannedEndAt: string;
    }
  | {
      kind: "invalid";
      reason: "end_before_start";
    };

interface NormalizePlanningInput {
  startDate: string;
  endDate: string;
}

export function normalizePlanningInput({
  startDate,
  endDate,
}: NormalizePlanningInput): PlanningNormalizationResult {
  if (!startDate && !endDate) {
    return { kind: "unscheduled" };
  }

  const normalizedStart = startDate || endDate;
  const normalizedEnd = endDate || startDate;

  if (normalizedEnd < normalizedStart) {
    return { kind: "invalid", reason: "end_before_start" };
  }

  return {
    kind: "scheduled",
    plannedStartAt: `${normalizedStart}T09:00:00.000Z`,
    plannedEndAt: `${normalizedEnd}T18:00:00.000Z`,
  };
}
