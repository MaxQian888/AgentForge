"use client";

import { useEffect } from "react";
import { useMilestoneStore } from "@/lib/stores/milestone-store";
import type { Sprint } from "@/lib/stores/sprint-store";
import type { Task } from "@/lib/stores/task-store";

export function RoadmapView({
  projectId,
  tasks,
  sprints,
}: {
  projectId: string;
  tasks: Task[];
  sprints: Sprint[];
}) {
  const milestones = useMilestoneStore((state) => state.milestonesByProject[projectId] ?? []);
  const fetchMilestones = useMilestoneStore((state) => state.fetchMilestones);

  useEffect(() => {
    void fetchMilestones(projectId);
  }, [fetchMilestones, projectId]);

  return (
    <div className="space-y-4">
      {milestones.map((milestone) => {
        const milestoneTasks = tasks.filter((task) => task.milestoneId === milestone.id);
        const milestoneSprints = sprints.filter((sprint) => sprint.milestoneId === milestone.id);
        return (
          <div key={milestone.id} className="rounded-lg border bg-card p-4">
            <div className="flex items-center justify-between gap-3">
              <div>
                <div className="text-lg font-semibold">{milestone.name}</div>
                <div className="text-sm text-muted-foreground">
                  {milestone.targetDate ?? "No target date"} · {milestone.status}
                </div>
              </div>
              <div className="text-sm text-muted-foreground">
                {milestone.metrics?.completionRate ?? 0}% complete
              </div>
            </div>
            <div className="mt-4 grid gap-4 md:grid-cols-2">
              <div>
                <div className="mb-2 text-sm font-medium">Sprints</div>
                <div className="space-y-2">
                  {milestoneSprints.map((sprint) => (
                    <div key={sprint.id} className="rounded-md border px-3 py-2 text-sm">
                      {sprint.name}
                    </div>
                  ))}
                </div>
              </div>
              <div>
                <div className="mb-2 text-sm font-medium">Tasks</div>
                <div className="space-y-2">
                  {milestoneTasks.map((task) => (
                    <div key={task.id} className="rounded-md border px-3 py-2 text-sm">
                      {task.title}
                    </div>
                  ))}
                </div>
              </div>
            </div>
          </div>
        );
      })}
    </div>
  );
}
