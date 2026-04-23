"use client";

import { useTranslations } from "next-intl";
import { Badge } from "@/components/ui/badge";
import { EmptyState } from "@/components/shared/empty-state";
import { ResponsiveTable } from "@/components/shared/responsive-table";
import { BarChart3 } from "lucide-react";

export interface AgentPerformanceRecord {
  bucketId: string;
  label: string;
  runCount: number;
  successRate: number;
  avgCostUsd: number;
  avgDurationMinutes: number;
  totalCostUsd: number;
}

interface AgentPerformanceTableProps {
  data: AgentPerformanceRecord[];
}

export function AgentPerformanceTable({ data }: AgentPerformanceTableProps) {
  const t = useTranslations("cost");
  if (data.length === 0) {
    return (
      <EmptyState
        icon={BarChart3}
        title={t("noAgentPerformance")}
        className="py-8"
      />
    );
  }

  return (
    <ResponsiveTable
      columns={[
        {
          key: "bucket",
          header: t("colBucket"),
          renderCell: (agent) => <span className="font-medium">{agent.label}</span>,
          hideOnCard: true,
        },
        {
          key: "runs",
          header: t("colRuns"),
          renderCell: (agent) => agent.runCount,
        },
        {
          key: "success",
          header: t("colSuccess"),
          renderCell: (agent) => (
            <Badge
              variant={agent.successRate >= 0.8 ? "secondary" : "destructive"}
            >
              {Math.round(agent.successRate * 100)}%
            </Badge>
          ),
        },
        {
          key: "avg-cost",
          header: t("colAvgCost"),
          renderCell: (agent) => `$${agent.avgCostUsd.toFixed(2)}`,
          hideOnTablet: true,
        },
        {
          key: "avg-duration",
          header: t("colAvgDuration"),
          renderCell: (agent) =>
            `${agent.avgDurationMinutes.toFixed(0)}${t("durationMinutesSuffix")}`,
          hideOnTablet: true,
        },
        {
          key: "total-cost",
          header: t("colTotalCost"),
          renderCell: (agent) => `$${agent.totalCostUsd.toFixed(2)}`,
        },
      ]}
      data={data}
      getRowId={(agent) => agent.bucketId}
      mobileCardTitle={(agent) => agent.label}
    />
  );
}
