"use client";

import { useEffect } from "react";
import { useAutomationStore } from "@/lib/stores/automation-store";

export function AutomationLogViewer({ projectId }: { projectId: string }) {
  const logs = useAutomationStore((state) => state.logsByProject[projectId] ?? []);
  const fetchLogs = useAutomationStore((state) => state.fetchLogs);

  useEffect(() => {
    void fetchLogs(projectId);
  }, [fetchLogs, projectId]);

  return (
    <div className="space-y-2">
      {logs.map((log) => (
        <div key={log.id} className="rounded-md border px-3 py-2 text-sm">
          <div className="font-medium">
            {log.eventType} · {log.status}
          </div>
          <div className="text-muted-foreground">{log.triggeredAt}</div>
        </div>
      ))}
    </div>
  );
}
