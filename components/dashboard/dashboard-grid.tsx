"use client";

import { useEffect, useMemo, useState } from "react";
import { useDashboardStore, type DashboardConfig } from "@/lib/stores/dashboard-store";
import { WidgetWrapper } from "./widget-wrapper";
import { AddWidgetDialog } from "./add-widget-dialog";
import { BurndownChartWidget, MetricCard, ThroughputChart } from "./widgets";

type WidgetData = Record<string, unknown>;

export function DashboardGrid({
  projectId,
  dashboard,
}: {
  projectId: string;
  dashboard: DashboardConfig;
}) {
  const widgets = useDashboardStore((state) => state.widgetsByDashboard[dashboard.id] ?? []);
  const fetchWidgetData = useDashboardStore((state) => state.fetchWidgetData);
  const widgetData = useDashboardStore((state) => state.widgetData);
  const [addOpen, setAddOpen] = useState(false);

  useEffect(() => {
    for (const widget of widgets) {
      void fetchWidgetData(projectId, widget.widgetType, widget.config);
    }
  }, [fetchWidgetData, projectId, widgets]);

  const items = useMemo(
    () =>
      widgets.map((widget) => ({
        widget,
        data: widgetData[
          `${projectId}:${widget.widgetType}:${JSON.stringify(widget.config ?? {})}`
        ] as WidgetData | undefined,
      })),
    [projectId, widgetData, widgets]
  );

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <div className="text-lg font-semibold">{dashboard.name}</div>
        <button type="button" className="rounded-md border px-3 py-2 text-sm" onClick={() => setAddOpen(true)}>
          Add Widget
        </button>
      </div>
      <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
        {items.map(({ widget, data }) => (
          <WidgetWrapper
            key={widget.id}
            title={widget.widgetType}
            onRefresh={() => void fetchWidgetData(projectId, widget.widgetType, widget.config)}
          >
            {widget.widgetType === "throughput_chart" ? (
              <ThroughputChart data={(data?.points as Array<{ date: string; count: number }>) ?? []} />
            ) : widget.widgetType === "burndown" ? (
              <BurndownChartWidget data={(data?.points as Array<{ date: string; remainingTasks: number; completedTasks: number }>) ?? []} />
            ) : widget.widgetType === "blocker_count" ? (
              <MetricCard label="Blocked Tasks" value={String(data?.count ?? 0)} />
            ) : widget.widgetType === "budget_consumption" ? (
              <MetricCard label="Budget" value={`$${Number(data?.spent ?? 0).toFixed(2)}`} secondary={`Allocated $${Number(data?.allocated ?? 0).toFixed(2)}`} />
            ) : widget.widgetType === "agent_cost" ? (
              <MetricCard label="Agent Cost Entries" value={String((data?.entries as unknown[] | undefined)?.length ?? 0)} />
            ) : widget.widgetType === "review_backlog" ? (
              <MetricCard label="Review Backlog" value={String(data?.count ?? 0)} />
            ) : widget.widgetType === "task_aging" ? (
              <MetricCard label="Aging Buckets" value={String((data?.buckets as unknown[] | undefined)?.length ?? 0)} />
            ) : (
              <MetricCard label="SLA Compliance" value={`${Math.round(Number(data?.rate ?? 0))}%`} secondary={`${data?.compliant ?? 0}/${data?.total ?? 0} tasks`} />
            )}
          </WidgetWrapper>
        ))}
      </div>
      <AddWidgetDialog open={addOpen} onOpenChange={setAddOpen} projectId={projectId} dashboardId={dashboard.id} />
    </div>
  );
}
