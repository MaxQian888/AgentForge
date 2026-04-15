"use client";

import { useEffect } from "react";
import {
  type AutomationLogActionOutcome,
  useAutomationStore,
} from "@/lib/stores/automation-store";

function renderActionOutcome(outcome: AutomationLogActionOutcome) {
  const target = outcome.pluginId ? ` ${outcome.pluginId}` : "";
  const run = outcome.runId ? ` (#${outcome.runId})` : "";
  const reason = outcome.reason ? ` - ${outcome.reason}` : "";
  return `${outcome.type} ${outcome.outcome}${target}${run}${reason}`;
}

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
          {log.detail.actionOutcomes?.length ? (
            <div className="mt-1 text-xs text-muted-foreground">
              {log.detail.actionOutcomes.map((outcome) => (
                <div key={`${log.id}-${outcome.type}-${outcome.runId ?? outcome.reasonCode ?? outcome.outcome}`}>
                  {renderActionOutcome(outcome)}
                </div>
              ))}
            </div>
          ) : null}
        </div>
      ))}
    </div>
  );
}
