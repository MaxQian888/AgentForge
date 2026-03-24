"use client";

import { Badge } from "@/components/ui/badge";

export interface VelocityPoint {
  period: string;
  tasksCompleted: number;
  costUsd: number;
}

interface VelocityChartProps {
  data: VelocityPoint[];
}

export function VelocityChart({ data }: VelocityChartProps) {
  if (data.length === 0) {
    return (
      <div className="flex h-[200px] items-center justify-center text-sm text-muted-foreground">
        No velocity data available yet.
      </div>
    );
  }

  const maxTasks = Math.max(...data.map((d) => d.tasksCompleted), 1);
  const barWidth = Math.max(Math.floor(100 / data.length) - 2, 4);

  return (
    <div className="space-y-3">
      <div className="flex h-[200px] items-end gap-1">
        {data.map((point) => {
          const height = Math.round((point.tasksCompleted / maxTasks) * 100);
          return (
            <div
              key={point.period}
              className="flex flex-1 flex-col items-center gap-1"
              title={`${point.period}: ${point.tasksCompleted} tasks, $${point.costUsd.toFixed(2)}`}
            >
              <span className="text-xs text-muted-foreground">
                {point.tasksCompleted}
              </span>
              <div
                className="w-full rounded-t bg-primary transition-all"
                style={{
                  height: `${Math.max(height, 2)}%`,
                  maxWidth: `${barWidth}%`,
                }}
              />
              <span className="text-[10px] text-muted-foreground">
                {point.period.slice(5)}
              </span>
            </div>
          );
        })}
      </div>
      <div className="flex flex-wrap gap-2">
        <Badge variant="outline">
          Avg: {(data.reduce((s, d) => s + d.tasksCompleted, 0) / data.length).toFixed(1)} tasks/period
        </Badge>
        <Badge variant="secondary">
          Total: ${data.reduce((s, d) => s + d.costUsd, 0).toFixed(2)}
        </Badge>
      </div>
    </div>
  );
}
