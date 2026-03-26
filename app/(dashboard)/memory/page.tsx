"use client";

import { MemoryPanel } from "@/components/memory/memory-panel";
import { useDashboardStore } from "@/lib/stores/dashboard-store";

export default function MemoryPage() {
  const selectedProjectId = useDashboardStore((s) => s.selectedProjectId);

  if (!selectedProjectId) {
    return (
      <div className="flex flex-col gap-6">
        <h1 className="text-2xl font-bold">Agent Memory</h1>
        <p className="text-sm text-muted-foreground">
          Select a project from the Dashboard to browse agent memory entries.
        </p>
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-6">
      <h1 className="text-2xl font-bold">Agent Memory</h1>
      <MemoryPanel projectId={selectedProjectId} />
    </div>
  );
}
