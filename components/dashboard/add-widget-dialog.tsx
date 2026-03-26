"use client";

import { useState } from "react";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { useDashboardStore } from "@/lib/stores/dashboard-store";

const WIDGET_TYPES = [
  "throughput_chart",
  "burndown",
  "blocker_count",
  "budget_consumption",
  "agent_cost",
  "review_backlog",
  "task_aging",
  "sla_compliance",
];

export function AddWidgetDialog({
  open,
  onOpenChange,
  projectId,
  dashboardId,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  projectId: string;
  dashboardId: string;
}) {
  const saveWidget = useDashboardStore((state) => state.saveWidget);
  const [widgetType, setWidgetType] = useState(WIDGET_TYPES[0]);

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Add Widget</DialogTitle>
          <DialogDescription>Select a widget type to add to this dashboard.</DialogDescription>
        </DialogHeader>
        <div className="space-y-2">
          {WIDGET_TYPES.map((type) => (
            <label key={type} className="flex items-center gap-2 rounded-md border px-3 py-2 text-sm">
              <input type="radio" checked={widgetType === type} onChange={() => setWidgetType(type)} />
              {type}
            </label>
          ))}
        </div>
        <DialogFooter>
          <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
            Cancel
          </Button>
          <Button
            type="button"
            onClick={async () => {
              await saveWidget(projectId, dashboardId, { widgetType, config: {}, position: {} });
              onOpenChange(false);
            }}
          >
            Add
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
