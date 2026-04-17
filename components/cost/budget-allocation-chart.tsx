"use client";

import { useTranslations } from "next-intl";
import {
  Cell,
  Legend,
  Pie,
  PieChart,
  ResponsiveContainer,
  Tooltip,
} from "recharts";

export interface BudgetAllocationSlice {
  category: string;
  amountUsd: number;
}

interface BudgetAllocationChartProps {
  data: BudgetAllocationSlice[];
  onConfigureBudget?: () => void;
}

// Semantic category palette — tokens read from CSS variables where possible but
// recharts needs direct hex values for SVG fills.
const SLICE_COLORS = [
  "#6366f1",
  "#22c55e",
  "#f59e0b",
  "#ef4444",
  "#06b6d4",
  "#a855f7",
];

export function BudgetAllocationChart({
  data,
  onConfigureBudget,
}: BudgetAllocationChartProps) {
  const t = useTranslations("cost");
  const hasData = data.length > 0 && data.some((d) => d.amountUsd > 0);

  if (!hasData) {
    return (
      <div
        className="flex h-[240px] flex-col items-center justify-center gap-2 text-center"
        data-testid="budget-allocation-empty"
      >
        <p className="text-sm text-muted-foreground">{t("noBudgetConfigured")}</p>
        {onConfigureBudget ? (
          <button
            type="button"
            onClick={onConfigureBudget}
            className="text-sm font-medium text-primary hover:underline"
          >
            {t("configureBudgetLink")}
          </button>
        ) : null}
      </div>
    );
  }

  return (
    <ResponsiveContainer width="100%" height={260}>
      <PieChart>
        <Pie
          data={data}
          dataKey="amountUsd"
          nameKey="category"
          innerRadius={60}
          outerRadius={90}
          paddingAngle={2}
        >
          {data.map((_entry, idx) => (
            <Cell
              key={`slice-${idx}`}
              fill={SLICE_COLORS[idx % SLICE_COLORS.length]}
            />
          ))}
        </Pie>
        <Tooltip
          formatter={(value, name) => [
            `$${Number(value).toFixed(2)}`,
            String(name),
          ]}
          contentStyle={{
            backgroundColor: "var(--popover)",
            border: "1px solid var(--border)",
            borderRadius: "6px",
          }}
        />
        <Legend
          verticalAlign="bottom"
          iconType="circle"
          formatter={(value) => (
            <span className="text-xs text-muted-foreground">{value}</span>
          )}
        />
      </PieChart>
    </ResponsiveContainer>
  );
}
