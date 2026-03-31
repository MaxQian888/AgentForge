export const WIDGET_TYPES = [
  "throughput_chart",
  "burndown",
  "blocker_count",
  "budget_consumption",
  "agent_cost",
  "review_backlog",
  "task_aging",
  "sla_compliance",
] as const;

export type DashboardWidgetType = (typeof WIDGET_TYPES)[number];

export interface DashboardWidgetMetadata {
  type: DashboardWidgetType;
  titleKey: string;
  descriptionKey: string;
  defaultConfig: Record<string, unknown>;
}

export const DASHBOARD_WIDGETS: DashboardWidgetMetadata[] = [
  {
    type: "throughput_chart",
    titleKey: "widget.throughput_chart.title",
    descriptionKey: "widget.throughput_chart.description",
    defaultConfig: { range: "14d", groupBy: "day", chartType: "bar" },
  },
  {
    type: "burndown",
    titleKey: "widget.burndown.title",
    descriptionKey: "widget.burndown.description",
    defaultConfig: { range: "current_sprint", groupBy: "day" },
  },
  {
    type: "blocker_count",
    titleKey: "widget.blocker_count.title",
    descriptionKey: "widget.blocker_count.description",
    defaultConfig: {},
  },
  {
    type: "budget_consumption",
    titleKey: "widget.budget_consumption.title",
    descriptionKey: "widget.budget_consumption.description",
    defaultConfig: {},
  },
  {
    type: "agent_cost",
    titleKey: "widget.agent_cost.title",
    descriptionKey: "widget.agent_cost.description",
    defaultConfig: {},
  },
  {
    type: "review_backlog",
    titleKey: "widget.review_backlog.title",
    descriptionKey: "widget.review_backlog.description",
    defaultConfig: {},
  },
  {
    type: "task_aging",
    titleKey: "widget.task_aging.title",
    descriptionKey: "widget.task_aging.description",
    defaultConfig: {},
  },
  {
    type: "sla_compliance",
    titleKey: "widget.sla_compliance.title",
    descriptionKey: "widget.sla_compliance.description",
    defaultConfig: {},
  },
];

export function getWidgetMetadata(type: string): DashboardWidgetMetadata {
  return (
    DASHBOARD_WIDGETS.find((widget) => widget.type === type) ?? {
      type: "throughput_chart",
      titleKey: "widget.unknown.title",
      descriptionKey: "widget.unknown.description",
      defaultConfig: {},
    }
  );
}
