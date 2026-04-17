"use client";

import { useMemo } from "react";
import { useTranslations } from "next-intl";
import {
  Bar,
  BarChart,
  CartesianGrid,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";

export interface AgentCostEntry {
  label: string;
  totalCostUsd: number;
}

interface AgentCostBarChartProps {
  data: AgentCostEntry[];
}

export function AgentCostBarChart({ data }: AgentCostBarChartProps) {
  const t = useTranslations("cost");

  // Sort descending by cost and filter zero-cost entries.
  const prepared = useMemo(
    () =>
      data
        .filter((entry) => entry.totalCostUsd > 0)
        .sort((a, b) => b.totalCostUsd - a.totalCostUsd),
    [data],
  );

  if (prepared.length === 0) {
    return (
      <div
        className="flex h-[220px] items-center justify-center text-sm text-muted-foreground"
        data-testid="agent-cost-empty"
      >
        {t("noAgentCostData")}
      </div>
    );
  }

  // Horizontal bar — vertical layout with category on Y and value on X.
  const chartHeight = Math.max(220, prepared.length * 36);

  return (
    <ResponsiveContainer width="100%" height={chartHeight}>
      <BarChart data={prepared} layout="vertical" margin={{ left: 12 }}>
        <CartesianGrid strokeDasharray="3 3" className="stroke-border" />
        <XAxis
          type="number"
          className="text-xs"
          tick={{ fill: "var(--muted-foreground)" }}
          tickFormatter={(v: number) => `$${v}`}
        />
        <YAxis
          type="category"
          dataKey="label"
          width={120}
          className="text-xs"
          tick={{ fill: "var(--muted-foreground)" }}
        />
        <Tooltip
          formatter={(value) => [
            `$${Number(value).toFixed(2)}`,
            t("tooltipCost"),
          ]}
          contentStyle={{
            backgroundColor: "var(--popover)",
            border: "1px solid var(--border)",
            borderRadius: "6px",
          }}
        />
        <Bar dataKey="totalCostUsd" fill="var(--primary)" radius={[0, 4, 4, 0]} />
      </BarChart>
    </ResponsiveContainer>
  );
}
