import type { TaskDependencySummary } from "@/lib/tasks/task-dependencies";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import type {
  TaskCostSummary,
  TaskHealthCounts,
} from "@/lib/tasks/task-context-rail";

export interface TaskProgressSummaryProps {
  counts: TaskHealthCounts;
  dependencySummary: TaskDependencySummary;
  costSummary: TaskCostSummary;
  realtimeState: "live" | "degraded";
}

export function TaskProgressSummary({
  counts,
  dependencySummary,
  costSummary,
  realtimeState,
}: TaskProgressSummaryProps) {
  return (
    <Card>
      <CardHeader className="pb-3">
        <div className="flex items-center justify-between gap-3">
          <CardTitle className="text-base">Progress health</CardTitle>
          <Badge variant={realtimeState === "live" ? "secondary" : "outline"}>
            {realtimeState === "live" ? "Realtime live" : "Realtime degraded"}
          </Badge>
        </div>
      </CardHeader>
      <CardContent className="flex flex-col gap-2 text-sm">
        <div>Healthy {counts.healthy}</div>
        <div>Warning {counts.warning}</div>
        <div>Stalled {counts.stalled}</div>
        <div>Unscheduled {counts.unscheduled}</div>
        <div>Blocked {dependencySummary.blocked}</div>
        <div>Ready to unblock {dependencySummary.readyToUnblock}</div>
        <div>
          Task spend ${costSummary.totalSpentUsd.toFixed(2)} / $
          {costSummary.totalBudgetUsd.toFixed(2)}
        </div>
        <div>
          Active runs {costSummary.activeRunCount} using $
          {costSummary.activeRunCostUsd.toFixed(2)} / $
          {costSummary.activeRunBudgetUsd.toFixed(2)}
        </div>
        <div>
          Budgeted tasks {costSummary.budgetedTaskCount}, over budget{" "}
          {costSummary.overBudgetTaskCount}
        </div>
        {realtimeState === "degraded" ? (
          <div className="text-muted-foreground">Realtime updates unavailable.</div>
        ) : null}
      </CardContent>
    </Card>
  );
}
