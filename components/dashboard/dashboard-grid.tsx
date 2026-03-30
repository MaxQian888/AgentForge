"use client";

import { useEffect, useMemo, useState } from "react";
import { useTranslations } from "next-intl";
import {
  Responsive,
  WidthProvider,
  type Layout,
  type LayoutItem,
  type ResponsiveLayouts,
} from "react-grid-layout/legacy";
import {
  useDashboardStore,
  type DashboardConfig,
  type DashboardWidget,
} from "@/lib/stores/dashboard-store";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Label } from "@/components/ui/label";
import { WidgetCard } from "./widget-card";
import { AddWidgetDialog } from "./add-widget-dialog";
import { getWidgetMetadata } from "./widget-catalog";
import { BurndownChartWidget, MetricCard, ThroughputChart } from "./widgets";

type WidgetData = Record<string, unknown>;
const ResponsiveGridLayout = WidthProvider(Responsive);

interface WidgetPosition {
  x: number;
  y: number;
  w: number;
  h: number;
}

function toLayoutItem(id: string, position: WidgetPosition): LayoutItem {
  return {
    i: id,
    x: position.x,
    y: position.y,
    w: position.w,
    h: position.h,
  };
}

function samePosition(left: WidgetPosition, right: WidgetPosition): boolean {
  return (
    left.x === right.x &&
    left.y === right.y &&
    left.w === right.w &&
    left.h === right.h
  );
}

function normalizePosition(position: unknown): WidgetPosition {
  if (!position || typeof position !== "object") {
    return { x: 0, y: 0, w: 1, h: 1 };
  }

  const candidate = position as Partial<WidgetPosition>;

  return {
    x: typeof candidate.x === "number" ? candidate.x : 0,
    y: typeof candidate.y === "number" ? candidate.y : 0,
    w: typeof candidate.w === "number" && candidate.w > 0 ? candidate.w : 1,
    h: typeof candidate.h === "number" && candidate.h > 0 ? candidate.h : 1,
  };
}

function isWidgetEmpty(widgetType: string, data: WidgetData | undefined) {
  if (!data) {
    return true;
  }

  if (widgetType === "throughput_chart" || widgetType === "burndown") {
    return !Array.isArray(data.points) || data.points.length === 0;
  }

  return false;
}

function renderWidgetBody(
  widgetType: string,
  data: WidgetData | undefined,
  t: ReturnType<typeof useTranslations>
) {
  if (widgetType === "throughput_chart") {
    return (
      <ThroughputChart
        data={(data?.points as Array<{ date: string; count: number }>) ?? []}
      />
    );
  }
  if (widgetType === "burndown") {
    return (
      <BurndownChartWidget
        data={
          (data?.points as Array<{
            date: string;
            remainingTasks: number;
            completedTasks: number;
          }>) ?? []
        }
      />
    );
  }
  if (widgetType === "blocker_count") {
    return (
      <MetricCard
        label={t("widget.blockedTasks")}
        value={String(data?.count ?? 0)}
      />
    );
  }
  if (widgetType === "budget_consumption") {
    return (
      <MetricCard
        label={t("widget.budget")}
        value={`$${Number(data?.spent ?? 0).toFixed(2)}`}
        secondary={t("widget.budgetAllocated", {
          amount: Number(data?.allocated ?? 0).toFixed(2),
        })}
      />
    );
  }
  if (widgetType === "agent_cost") {
    return (
      <MetricCard
        label={t("widget.agentCostEntries")}
        value={String((data?.entries as unknown[] | undefined)?.length ?? 0)}
      />
    );
  }
  if (widgetType === "review_backlog") {
    return (
      <MetricCard
        label={t("widget.reviewBacklog")}
        value={String(data?.count ?? 0)}
      />
    );
  }
  if (widgetType === "task_aging") {
    return (
      <MetricCard
        label={t("widget.agingBuckets")}
        value={String((data?.buckets as unknown[] | undefined)?.length ?? 0)}
      />
    );
  }

  return (
    <MetricCard
      label={t("widget.slaCompliance")}
      value={`${Math.round(Number(data?.rate ?? 0))}%`}
      secondary={t("widget.slaTasks", {
        compliant: Number(data?.compliant ?? 0),
        total: Number(data?.total ?? 0),
      })}
    />
  );
}

export function DashboardGrid({
  projectId,
  dashboard,
}: {
  projectId: string;
  dashboard: DashboardConfig;
}) {
  const t = useTranslations("dashboard");
  const widgets = useDashboardStore(
    (state) => state.widgetsByDashboard[dashboard.id] ?? []
  );
  const fetchWidgetData = useDashboardStore((state) => state.fetchWidgetData);
  const saveWidget = useDashboardStore((state) => state.saveWidget);
  const updateDashboard = useDashboardStore((state) => state.updateDashboard);
  const deleteWidget = useDashboardStore((state) => state.deleteWidget);
  const widgetData = useDashboardStore((state) => state.widgetData);
  const widgetRequestStateByKey = useDashboardStore(
    (state) => state.widgetRequestStateByKey
  );
  const [addOpen, setAddOpen] = useState(false);
  const [layoutDraft, setLayoutDraft] = useState<Record<string, WidgetPosition>>(
    {}
  );
  const [layoutStatus, setLayoutStatus] = useState<
    "idle" | "saving" | "saved" | "error"
  >("idle");
  const [configuringWidgetId, setConfiguringWidgetId] = useState<string | null>(
    null
  );
  const [configDraft, setConfigDraft] = useState<Record<string, unknown>>({});

  useEffect(() => {
    for (const widget of widgets) {
      void fetchWidgetData(projectId, widget.widgetType, widget.config);
    }
  }, [fetchWidgetData, projectId, widgets]);

  const items = useMemo(
    () =>
      [...widgets]
        .map((widget) => {
          const key = `${projectId}:${widget.widgetType}:${JSON.stringify(
            widget.config ?? {}
          )}`;
          const position =
            layoutDraft[widget.id] ?? normalizePosition(widget.position);

          return {
            widget,
            position,
            requestState: widgetRequestStateByKey[key] ?? {
              status: "idle" as const,
              error: null,
            },
            data: widgetData[key] as WidgetData | undefined,
          };
        })
        .sort(
          (left, right) =>
            left.position.y - right.position.y || left.position.x - right.position.x
        ),
    [layoutDraft, projectId, widgetData, widgetRequestStateByKey, widgets]
  );

  const configuringWidget = useMemo(
    () => widgets.find((widget) => widget.id === configuringWidgetId) ?? null,
    [configuringWidgetId, widgets]
  );

  const currentPositions = useMemo(
    () =>
      Object.fromEntries(
        items.map(({ widget, position }) => [widget.id, position])
      ) as Record<string, WidgetPosition>,
    [items]
  );
  const responsiveLayouts = useMemo<ResponsiveLayouts>(() => {
    const layout = items.map(({ widget, position }) =>
      toLayoutItem(widget.id, position)
    );

    return {
      lg: layout,
      md: layout,
      sm: layout,
    };
  }, [items]);

  const persistLayoutDraft = async (draft: Record<string, WidgetPosition>) => {
    setLayoutDraft(draft);
    setLayoutStatus("saving");

    try {
      await Promise.all(
        widgets.map((widget) =>
          saveWidget(projectId, dashboard.id, {
            id: widget.id,
            widgetType: widget.widgetType,
            config: widget.config,
            position: draft[widget.id] ?? normalizePosition(widget.position),
          })
        )
      );
      await updateDashboard(projectId, dashboard.id, {
        layout: widgets.map((item) => ({
          id: item.id,
          ...(draft[item.id] ?? normalizePosition(item.position)),
        })),
      });
      setLayoutStatus("saved");
    } catch {
      setLayoutStatus("error");
    }
  };

  const persistLayout = async (
    widget: DashboardWidget,
    nextPosition: WidgetPosition,
    nextDraft?: Record<string, WidgetPosition>
  ) => {
    const draft = nextDraft ?? {
      ...currentPositions,
      [widget.id]: nextPosition,
    };

    await persistLayoutDraft(draft);
  };

  const openWidgetConfig = (widget: DashboardWidget) => {
    setConfigDraft(
      typeof widget.config === "object" && widget.config
        ? (widget.config as Record<string, unknown>)
        : {}
    );
    setConfiguringWidgetId(widget.id);
  };

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <div className="text-lg font-semibold">{dashboard.name}</div>
        <Button type="button" variant="outline" onClick={() => setAddOpen(true)}>
          {t("widget.addWidget")}
        </Button>
      </div>

      {layoutStatus === "saving" ? (
        <div className="text-sm text-muted-foreground">
          {t("widget.layoutSaving")}
        </div>
      ) : layoutStatus === "saved" ? (
        <div className="text-sm text-emerald-600">{t("widget.layoutSaved")}</div>
      ) : layoutStatus === "error" ? (
        <div className="flex items-center gap-2 text-sm text-destructive">
          <span>{t("widget.errorFallback")}</span>
          <Button
            type="button"
            size="sm"
            variant="outline"
            onClick={() => setLayoutStatus("idle")}
          >
            {t("widget.layoutRetry")}
          </Button>
        </div>
      ) : null}

      <ResponsiveGridLayout
        layouts={responsiveLayouts}
        breakpoints={{ lg: 1280, md: 768, sm: 0 }}
        cols={{ lg: 3, md: 2, sm: 1 }}
        margin={[16, 16]}
        containerPadding={[0, 0]}
        rowHeight={220}
        isDraggable
        isResizable
        draggableHandle=".dashboard-widget-handle"
        onLayoutChange={(layout: Layout) => {
          if (layout.length === 0) {
            return;
          }

          const nextDraft: Record<string, WidgetPosition> = {};
          for (const item of layout) {
            nextDraft[item.i] = normalizePosition({
              x: item.x,
              y: item.y,
              w: item.w,
              h: item.h,
            });
          }
          const changed = layout.some((item) => {
            const current = currentPositions[item.i];
            const next = nextDraft[item.i];
            return !current || !samePosition(current, next);
          });

          if (!changed) {
            return;
          }

          void persistLayoutDraft({
            ...currentPositions,
            ...nextDraft,
          });
        }}
      >
        {items.map(({ widget, position, requestState, data }) => {
          const metadata = getWidgetMetadata(widget.widgetType);
          const wrapperState =
            requestState.status === "error"
              ? "error"
              : isWidgetEmpty(widget.widgetType, data)
                ? "empty"
                : "ready";

          return (
            <div key={widget.id}>
              <div className="dashboard-widget-handle mb-2 cursor-move text-xs font-medium text-muted-foreground">
                {t(metadata.titleKey)}
              </div>
              <WidgetCard
                title={t(metadata.titleKey)}
                state={wrapperState}
                message={
                  wrapperState === "error"
                    ? requestState.error ?? t("widget.errorFallback")
                    : undefined
                }
                onRefresh={() =>
                  void fetchWidgetData(projectId, widget.widgetType, widget.config)
                }
                onRetry={() =>
                  void fetchWidgetData(projectId, widget.widgetType, widget.config)
                }
                onConfigure={() => openWidgetConfig(widget)}
                onRemove={() =>
                  void deleteWidget(projectId, dashboard.id, widget.id)
                }
              >
                {renderWidgetBody(widget.widgetType, data, t)}
              </WidgetCard>
              <div className="mt-2 flex gap-2">
                <Button
                  type="button"
                  size="sm"
                  variant="outline"
                  onClick={() =>
                    void persistLayout(widget, {
                      ...position,
                      w: position.w > 1 ? 1 : 2,
                    })
                  }
                >
                  {position.w > 1
                    ? t("widget.resizeCompact")
                    : t("widget.resizeWide")}
                </Button>
              </div>
            </div>
          );
        })}
      </ResponsiveGridLayout>

      <AddWidgetDialog
        open={addOpen}
        onOpenChange={setAddOpen}
        projectId={projectId}
        dashboardId={dashboard.id}
      />

      <Dialog
        open={Boolean(configuringWidget)}
        onOpenChange={(open) => {
          if (!open) {
            setConfiguringWidgetId(null);
          }
        }}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t("widget.configure")}</DialogTitle>
            <DialogDescription>
              {configuringWidget
                ? t(getWidgetMetadata(configuringWidget.widgetType).descriptionKey)
                : ""}
            </DialogDescription>
          </DialogHeader>

          {configuringWidget?.widgetType === "throughput_chart" ||
          configuringWidget?.widgetType === "burndown" ? (
            <div className="grid gap-4 md:grid-cols-2">
              <div className="space-y-2">
                <Label htmlFor="widget-config-range">
                  {t("widget.config.range")}
                </Label>
                <select
                  id="widget-config-range"
                  className="h-9 rounded-md border border-input bg-background px-3 text-sm"
                  value={String(configDraft.range ?? "")}
                  onChange={(event) =>
                    setConfigDraft((current) => ({
                      ...current,
                      range: event.target.value,
                    }))
                  }
                >
                  <option value="7d">7d</option>
                  <option value="14d">14d</option>
                  <option value="30d">30d</option>
                  <option value="current_sprint">current_sprint</option>
                </select>
              </div>
              <div className="space-y-2">
                <Label htmlFor="widget-config-group-by">
                  {t("widget.config.groupBy")}
                </Label>
                <select
                  id="widget-config-group-by"
                  className="h-9 rounded-md border border-input bg-background px-3 text-sm"
                  value={String(configDraft.groupBy ?? "")}
                  onChange={(event) =>
                    setConfigDraft((current) => ({
                      ...current,
                      groupBy: event.target.value,
                    }))
                  }
                >
                  <option value="day">day</option>
                  <option value="week">week</option>
                </select>
              </div>
            </div>
          ) : (
            <div className="text-sm text-muted-foreground">
              {t("widget.config.noExtraSettings")}
            </div>
          )}

          <DialogFooter>
            <Button
              type="button"
              variant="outline"
              onClick={() => setConfiguringWidgetId(null)}
            >
              {t("widget.cancel")}
            </Button>
            <Button
              type="button"
              onClick={async () => {
                if (!configuringWidget) {
                  return;
                }
                const position =
                  layoutDraft[configuringWidget.id] ??
                  normalizePosition(configuringWidget.position);

                await saveWidget(projectId, dashboard.id, {
                  id: configuringWidget.id,
                  widgetType: configuringWidget.widgetType,
                  config: configDraft,
                  position,
                });
                await fetchWidgetData(
                  projectId,
                  configuringWidget.widgetType,
                  configDraft
                );
                setConfiguringWidgetId(null);
              }}
            >
              {t("widget.saveConfig")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
