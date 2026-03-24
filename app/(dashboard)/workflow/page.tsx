"use client";

import { useDashboardStore } from "@/lib/stores/dashboard-store";
import { WorkflowConfigPanel } from "@/components/workflow/workflow-config-panel";

export default function WorkflowPage() {
  const selectedProjectId = useDashboardStore((s) => s.selectedProjectId);

  return (
    <div className="flex flex-col gap-6">
      <h1 className="text-2xl font-bold">Workflow Configuration</h1>
      {selectedProjectId ? (
        <WorkflowConfigPanel projectId={selectedProjectId} />
      ) : (
        <div className="text-sm text-muted-foreground">
          Select a project to configure its task workflow.
        </div>
      )}
    </div>
  );
}
