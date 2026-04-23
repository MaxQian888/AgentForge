"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
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

type DashboardTimeRange = "all" | "7d" | "30d" | "current_sprint";
type DashboardCategory = "all" | "tasks" | "reviews" | "agents" | "budget";

interface DashboardFilterState {
  timeRange: DashboardTimeRange;
  category: DashboardCategory;
}

interface PersistedLayoutItem {
  id: string;
  x: number;
  y: number;
  w: number;
  h: number;
}

interface PersistedDashboardLayout {
  items?: PersistedLayoutItem[];
  filters?: Partial<DashboardFilterState>;
}

interface DashboardAlertItem {
  id: string;
  priority: number;
  title: string;
  message: string;
  href: string;
  actionLabel: string;
}

const DEFAULT_FILTERS: DashboardFilterState = {
  timeRange: "all",
  category: "all",
};

type AutoRefreshInterval = "30s" | "60s" | "300s" | "off";

const DEFAULT_AUTO_REFRESH_INTERVAL: AutoRefreshInterval = "30s";

function toAutoRefreshMs(interval: AutoRefreshInterval) {
  switch (interval) {
    case "60s":
      return 60_000;
    case "300s":
      return 300_000;
    case "off":
      return null;
    case "30s":
    default:
      return 30_000;
  }
}

interface WidgetPosition {
  x: number;
  y: number;
  w: number;
  h: number;
}

function readPersistedDashboardLayout(
  layout: unknown,
): { items: PersistedLayoutItem[]; filters: DashboardFilterState } {
  if (Array.isArray(layout)) {
    return {
      items: layout as PersistedLayoutItem[],
      filters: DEFAULT_FILTERS,
    };
  }

  if (!layout || typeof layout !== "object") {
    return {
      items: [],
      filters: DEFAULT_FILTERS,
    };
  }

  const candidate = layout as PersistedDashboardLayout;
  return {
    items: Array.isArray(candidate.items)
      ? (candidate.items as PersistedLayoutItem[])
      : [],
    filters: {
      timeRange:
        candidate.filters?.timeRange ?? DEFAULT_FILTERS.timeRange,
      category: candidate.filters?.category ?? DEFAULT_FILTERS.category,
    },
  };
}

function serializeDashboardLayout(
  items: PersistedLayoutItem[],
  filters: DashboardFilterState,
) {
  if (
    filters.timeRange === DEFAULT_FILTERS.timeRange &&
    filters.category === DEFAULT_FILTERS.category
  ) {
    return items;
  }

  return {
    items,
    filters,
  };
}

function widgetSupportsTimeRange(widgetType: string) {
  return widgetType === "throughput_chart" || widgetType === "burndown";
}

function widgetSupportsCategory(widgetType: string) {
  return widgetType !== "budget_consumption";
}

function mergeWidgetConfig(
  widgetType: string,
  config: unknown,
  filters: DashboardFilterState,
) {
  const baseConfig =
    typeof config === "object" && config ? { ...(config as Record<string, unknown>) } : {};

  if (widgetSupportsTimeRange(widgetType) && filters.timeRange !== "all") {
    baseConfig.range = filters.timeRange;
  }

  if (widgetSupportsCategory(widgetType) && filters.category !== "all") {
    baseConfig.category = filters.category;
  }

  return baseConfig;
}

function buildWidgetRequestConfig(
  widgetType: string,
  config: unknown,
  filters: DashboardFilterState,
) {
  const merged = mergeWidgetConfig(widgetType, config, filters);
  delete merged.autoRefreshInterval;
  delete merged.autoRefreshPaused;
  return merged;
}

function getAutoRefreshInterval(config: unknown): AutoRefreshInterval {
  if (
    config &&
    typeof config === "object" &&
    typeof (config as Record<string, unknown>).autoRefreshInterval === "string"
  ) {
    return (config as Record<string, unknown>)
      .autoRefreshInterval as AutoRefreshInterval;
  }

  return DEFAULT_AUTO_REFRESH_INTERVAL;
}

function getAutoRefreshPaused(config: unknown) {
  if (
    config &&
    typeof config === "object" &&
    typeof (config as Record<string, unknown>).autoRefreshPaused === "boolean"
  ) {
    return Boolean((config as Record<string, unknown>).autoRefreshPaused);
  }

  return false;
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
  config: unknown,
  t: ReturnType<typeof useTranslations>
) {
  if (widgetType === "throughput_chart") {
    const chartType =
      config &&
      typeof config === "object" &&
      (config as Record<string, unknown>).chartType === "line"
        ? "line"
        : "bar";
    return (
      <ThroughputChart
        data={(data?.points as Array<{ date: string; count: number }>) ?? []}
        chartType={chartType}
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

function validateWidgetConfig(
  widgetType: string,
  config: Record<string, unknown>,
) {
  const range = typeof config.range === "string" ? config.range : undefined;
  const groupBy =
    typeof config.groupBy === "string" ? config.groupBy : undefined;
  const chartType =
    typeof config.chartType === "string" ? config.chartType : undefined;

  if (
    (widgetType === "throughput_chart" || widgetType === "burndown") &&
    range &&
    !["7d", "14d", "30d", "current_sprint"].includes(range)
  ) {
    return "widget.config.invalid";
  }

  if (
    (widgetType === "throughput_chart" || widgetType === "burndown") &&
    groupBy &&
    !["day", "week"].includes(groupBy)
  ) {
    return "widget.config.invalid";
  }

  if (
    widgetType === "throughput_chart" &&
    chartType &&
    !["bar", "line"].includes(chartType)
  ) {
    return "widget.config.invalid";
  }

  return null;
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
  const persistedLayout = useMemo(
    () => readPersistedDashboardLayout(dashboard.layout),
    [dashboard.layout],
  );
  const [filterDraft, setFilterDraft] = useState<{
    dashboardId: string;
    filters: DashboardFilterState;
    dirty: boolean;
  }>(() => ({
    dashboardId: dashboard.id,
    filters: persistedLayout.filters,
    dirty: false,
  }));
  const globalFilters =
    filterDraft.dashboardId === dashboard.id && filterDraft.dirty
      ? filterDraft.filters
      : persistedLayout.filters;
  const [lastUpdatedByWidgetId, setLastUpdatedByWidgetId] = useState<
    Record<string, number>
  >({});
  const [dismissedAlertState, setDismissedAlertState] = useState<{
    dashboardId: string;
    ids: string[];
  }>({
    dashboardId: dashboard.id,
    ids: [],
  });

  const performWidgetFetch = useCallback(
    async (
      widget: DashboardWidget,
      options?: {
        trackTimestamp?: boolean;
      },
    ) => {
      if (options?.trackTimestamp) {
        setLastUpdatedByWidgetId((current) => ({
          ...current,
          [widget.id]: Date.now(),
        }));
      }

      await fetchWidgetData(
        projectId,
        widget.widgetType,
        buildWidgetRequestConfig(widget.widgetType, widget.config, globalFilters),
      );
    },
    [fetchWidgetData, globalFilters, projectId],
  );

  const refreshWidget = useCallback(async (widget: DashboardWidget) => {
    await performWidgetFetch(widget, { trackTimestamp: true });
  }, [performWidgetFetch]);
  useEffect(() => {
    for (const widget of widgets) {
      void fetchWidgetData(
        projectId,
        widget.widgetType,
        buildWidgetRequestConfig(widget.widgetType, widget.config, globalFilters),
      );
    }
  }, [fetchWidgetData, globalFilters, projectId, widgets]);

  useEffect(() => {
    const cleanups: Array<() => void> = [];

    widgets.forEach((widget, index) => {
      const interval = getAutoRefreshInterval(widget.config);
      const paused = getAutoRefreshPaused(widget.config);
      const intervalMs = toAutoRefreshMs(interval);

      if (paused || intervalMs == null) {
        return;
      }

      let repeatingTimer: ReturnType<typeof setInterval> | null = null;
      const staggerMs = index * 500;
      const initialDelay = intervalMs + staggerMs;
      const initialTimer = setTimeout(() => {
        void refreshWidget(widget);
        repeatingTimer = setInterval(() => {
          void refreshWidget(widget);
        }, intervalMs);
      }, initialDelay);

      cleanups.push(() => {
        clearTimeout(initialTimer);
        if (repeatingTimer) {
          clearInterval(repeatingTimer);
        }
      });
    });

    return () => {
      cleanups.forEach((cleanup) => cleanup());
    };
  }, [globalFilters, projectId, refreshWidget, widgets]);

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
  const configValidationMessage = configuringWidget
    ? validateWidgetConfig(configuringWidget.widgetType, configDraft)
    : null;

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
      const nextItems = widgets.map((item) => ({
        id: item.id,
        ...(draft[item.id] ?? normalizePosition(item.position)),
      }));
      await updateDashboard(projectId, dashboard.id, {
        layout: serializeDashboardLayout(nextItems, globalFilters),
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

  const persistFilters = async (nextFilters: DashboardFilterState) => {
    setFilterDraft({
      dashboardId: dashboard.id,
      filters: nextFilters,
      dirty: true,
    });
    setLayoutStatus("saving");

    try {
      const nextItems = widgets.map((item) => ({
        id: item.id,
        ...(currentPositions[item.id] ?? normalizePosition(item.position)),
      }));
      await updateDashboard(projectId, dashboard.id, {
        layout: serializeDashboardLayout(nextItems, nextFilters),
      });
      setLayoutStatus("saved");
    } catch {
      setLayoutStatus("error");
    }
  };

  const activeFilterSummary =
    globalFilters.category !== "all" || globalFilters.timeRange !== "all"
      ? t("widget.filter.applied", {
          category: t(`widget.filter.category.${globalFilters.category}`),
          timeRange: t(`widget.filter.timeRange.${globalFilters.timeRange}`),
        })
      : null;
  const alerts = useMemo<DashboardAlertItem[]>(() => {
    const dismissedAlertIds =
      dismissedAlertState.dashboardId === dashboard.id
        ? dismissedAlertState.ids
        : [];
    const nextAlerts: DashboardAlertItem[] = [];

    items.forEach(({ widget, data }) => {
      if (widget.widgetType === "budget_consumption") {
        const spent = Number(data?.spent ?? 0);
        const allocated = Number(data?.allocated ?? 0);
        if (allocated > 0 && spent / allocated >= 0.9) {
          nextAlerts.push({
            id: `${widget.id}:budget-threshold`,
            priority: 100,
            title: t("widget.alert.budget.title"),
            message: t("widget.alert.budget.message"),
            href: "/cost",
            actionLabel: t("widget.alert.budget.action"),
          });
        }
      }

      if (widget.widgetType === "blocker_count") {
        const count = Number(data?.count ?? 0);
        if (count > 0) {
          nextAlerts.push({
            id: `${widget.id}:blockers`,
            priority: 50,
            title: t("widget.alert.blockers.title"),
            message: t("widget.alert.blockers.message"),
            href: `/project?id=${projectId}`,
            actionLabel: t("widget.alert.blockers.action"),
          });
        }
      }
    });

    return nextAlerts
      .filter((alert) => !dismissedAlertIds.includes(alert.id))
      .sort((left, right) => right.priority - left.priority);
  }, [dashboard.id, dismissedAlertState.dashboardId, dismissedAlertState.ids, items, projectId, t]);

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

      {alerts.length > 0 ? (
        <div className="space-y-3">
          {alerts.map((alert) => (
            <div
              key={alert.id}
              className="rounded-lg border border-amber-300 bg-amber-50 p-4 shadow-sm"
            >
              <div className="flex items-start justify-between gap-4">
                <div className="space-y-1">
                  <h3 className="text-sm font-semibold text-amber-950">
                    {alert.title}
                  </h3>
                  <p className="text-sm text-amber-900">{alert.message}</p>
                  <a
                    href={alert.href}
                    className="text-sm font-medium text-amber-900 underline underline-offset-4"
                  >
                    {alert.actionLabel}
                  </a>
                </div>
                <Button
                  type="button"
                  size="sm"
                  variant="outline"
                  onClick={() =>
                    setDismissedAlertState((current) => ({
                      dashboardId: dashboard.id,
                      ids:
                        current.dashboardId === dashboard.id
                          ? [...current.ids, alert.id]
                          : [alert.id],
                    }))
                  }
                >
                  {t("widget.alert.dismiss")}
                </Button>
              </div>
            </div>
          ))}
        </div>
      ) : null}

      <div className="flex flex-wrap items-end gap-3 rounded-lg border bg-card p-4">
        <label className="flex min-w-[180px] flex-col gap-1.5 text-sm font-medium">
          <span>{t("widget.filter.timeRange")}</span>
          <select
            aria-label={t("widget.filter.timeRange")}
            className="h-9 rounded-md border border-input bg-background px-3 text-sm font-normal"
            value={globalFilters.timeRange}
            onChange={(event) =>
              void persistFilters({
                ...globalFilters,
                timeRange: event.target.value as DashboardTimeRange,
              })
            }
          >
            <option value="all">{t("widget.filter.timeRange.all")}</option>
            <option value="7d">{t("widget.filter.timeRange.7d")}</option>
            <option value="30d">{t("widget.filter.timeRange.30d")}</option>
            <option value="current_sprint">
              {t("widget.filter.timeRange.current_sprint")}
            </option>
          </select>
        </label>

        <label className="flex min-w-[180px] flex-col gap-1.5 text-sm font-medium">
          <span>{t("widget.filter.category")}</span>
          <select
            aria-label={t("widget.filter.category")}
            className="h-9 rounded-md border border-input bg-background px-3 text-sm font-normal"
            value={globalFilters.category}
            onChange={(event) =>
              void persistFilters({
                ...globalFilters,
                category: event.target.value as DashboardCategory,
              })
            }
          >
            <option value="all">{t("widget.filter.category.all")}</option>
            <option value="tasks">{t("widget.filter.category.tasks")}</option>
            <option value="reviews">{t("widget.filter.category.reviews")}</option>
            <option value="agents">{t("widget.filter.category.agents")}</option>
            <option value="budget">{t("widget.filter.category.budget")}</option>
          </select>
        </label>

        {(globalFilters.timeRange !== "all" || globalFilters.category !== "all") && (
          <Button
            type="button"
            variant="outline"
            onClick={() => void persistFilters(DEFAULT_FILTERS)}
          >
            {t("widget.filter.clear")}
          </Button>
        )}

        {activeFilterSummary ? (
          <div className="text-sm text-muted-foreground">{activeFilterSummary}</div>
        ) : null}
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

          const autoRefreshInterval = getAutoRefreshInterval(widget.config);
          const autoRefreshPaused = getAutoRefreshPaused(widget.config);
          const lastUpdated = lastUpdatedByWidgetId[widget.id];

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
                autoRefresh={{
                  interval: autoRefreshInterval,
                  paused: autoRefreshPaused,
                  lastUpdatedLabel: lastUpdated
                    ? t("widget.lastUpdated", {
                        time: t(`widget.autoRefresh.interval.${autoRefreshInterval}`),
                      })
                    : undefined,
                  onPauseToggle: async () => {
                    await saveWidget(projectId, dashboard.id, {
                      id: widget.id,
                      widgetType: widget.widgetType,
                      config: {
                        ...(typeof widget.config === "object" && widget.config
                          ? (widget.config as Record<string, unknown>)
                          : {}),
                        autoRefreshInterval,
                        autoRefreshPaused: !autoRefreshPaused,
                      },
                      position,
                    });
                  },
                  onIntervalChange: async (interval) => {
                    await saveWidget(projectId, dashboard.id, {
                      id: widget.id,
                      widgetType: widget.widgetType,
                      config: {
                        ...(typeof widget.config === "object" && widget.config
                          ? (widget.config as Record<string, unknown>)
                          : {}),
                        autoRefreshInterval: interval,
                        autoRefreshPaused: autoRefreshPaused,
                      },
                      position,
                    });
                  },
                }}
                onRefresh={() =>
                  void refreshWidget(widget)
                }
                onRetry={() =>
                  void refreshWidget(widget)
                }
                onConfigure={() => openWidgetConfig(widget)}
                onRemove={() =>
                  void deleteWidget(projectId, dashboard.id, widget.id)
                }
              >
                {globalFilters.category !== "all" &&
                !widgetSupportsCategory(widget.widgetType) ? (
                  <div className="mb-3 text-xs text-muted-foreground">
                    {t("widget.filter.notApplicable")}
                  </div>
                ) : null}
                {renderWidgetBody(widget.widgetType, data, widget.config, t)}
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
                  <option value="7d">{t("widget.filter.timeRange.7d")}</option>
                  <option value="14d">{t("widget.filter.timeRange.14d")}</option>
                  <option value="30d">{t("widget.filter.timeRange.30d")}</option>
                  <option value="current_sprint">{t("widget.filter.timeRange.current_sprint")}</option>
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
                  <option value="day">{t("widget.config.groupBy.day")}</option>
                  <option value="week">{t("widget.config.groupBy.week")}</option>
                </select>
              </div>
              {configuringWidget?.widgetType === "throughput_chart" ? (
                <div className="space-y-2">
                  <Label htmlFor="widget-config-chart-type">
                    {t("widget.config.chartType")}
                  </Label>
                  <select
                    id="widget-config-chart-type"
                    aria-label={t("widget.config.chartType")}
                    className="h-9 rounded-md border border-input bg-background px-3 text-sm"
                    value={String(configDraft.chartType ?? "")}
                    onChange={(event) =>
                      setConfigDraft((current) => ({
                        ...current,
                        chartType: event.target.value,
                      }))
                    }
                  >
                    <option value="bar">{t("widget.config.chartType.bar")}</option>
                    <option value="line">{t("widget.config.chartType.line")}</option>
                  </select>
                </div>
              ) : null}
            </div>
          ) : (
            <div className="text-sm text-muted-foreground">
              {t("widget.config.noExtraSettings")}
            </div>
          )}
          {configValidationMessage ? (
            <div className="text-sm text-destructive">
              {t(configValidationMessage)}
            </div>
          ) : null}

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
                  buildWidgetRequestConfig(
                    configuringWidget.widgetType,
                    configDraft,
                    globalFilters
                  )
                );
                setConfiguringWidgetId(null);
              }}
              disabled={Boolean(configValidationMessage)}
            >
              {t("widget.saveConfig")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
