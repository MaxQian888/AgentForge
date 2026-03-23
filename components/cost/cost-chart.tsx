"use client";

import {
  LineChart,
  Line,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
} from "recharts";

interface CostDataPoint {
  date: string;
  cost: number;
}

interface CostChartProps {
  data: CostDataPoint[];
}

export function CostChart({ data }: CostChartProps) {
  return (
    <ResponsiveContainer width="100%" height={300}>
      <LineChart data={data}>
        <CartesianGrid strokeDasharray="3 3" className="stroke-border" />
        <XAxis
          dataKey="date"
          className="text-xs"
          tick={{ fill: "var(--muted-foreground)" }}
        />
        <YAxis
          className="text-xs"
          tick={{ fill: "var(--muted-foreground)" }}
          tickFormatter={(v: number) => `$${v}`}
        />
        <Tooltip
          formatter={(value) => [`$${Number(value).toFixed(2)}`, "Cost"]}
          contentStyle={{
            backgroundColor: "var(--popover)",
            border: "1px solid var(--border)",
            borderRadius: "6px",
          }}
        />
        <Line
          type="monotone"
          dataKey="cost"
          stroke="var(--primary)"
          strokeWidth={2}
          dot={false}
        />
      </LineChart>
    </ResponsiveContainer>
  );
}
