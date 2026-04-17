"use client";

import { useMemo, useState } from "react";
import { useTranslations } from "next-intl";
import {
  CartesianGrid,
  Line,
  LineChart,
  ReferenceLine,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";

export interface SpendingTrendPoint {
  date: string;
  cost: number;
}

type TrendPeriod = "7d" | "30d" | "90d";

interface SpendingTrendChartProps {
  data: SpendingTrendPoint[];
  defaultPeriod?: TrendPeriod;
}

const PERIOD_DAYS: Record<TrendPeriod, number> = {
  "7d": 7,
  "30d": 30,
  "90d": 90,
};

export function SpendingTrendChart({
  data,
  defaultPeriod = "30d",
}: SpendingTrendChartProps) {
  const t = useTranslations("cost");
  const [period, setPeriod] = useState<TrendPeriod>(defaultPeriod);

  const filtered = useMemo(() => {
    const n = PERIOD_DAYS[period];
    // data is expected in ascending date order — keep the last N entries
    return data.slice(-n);
  }, [data, period]);

  const average = useMemo(() => {
    if (filtered.length === 0) return 0;
    const total = filtered.reduce((sum, p) => sum + p.cost, 0);
    return total / filtered.length;
  }, [filtered]);

  return (
    <div className="space-y-3">
      <div className="flex items-center justify-between gap-2">
        <span className="text-sm text-muted-foreground">
          {t("trendAverageLabel", { avg: `$${average.toFixed(2)}` })}
        </span>
        <Select
          value={period}
          onValueChange={(next) => setPeriod(next as TrendPeriod)}
        >
          <SelectTrigger
            className="h-8 w-[140px] text-xs"
            aria-label={t("trendPeriodLabel")}
          >
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="7d">{t("trendPeriod7d")}</SelectItem>
            <SelectItem value="30d">{t("trendPeriod30d")}</SelectItem>
            <SelectItem value="90d">{t("trendPeriod90d")}</SelectItem>
          </SelectContent>
        </Select>
      </div>
      {filtered.length === 0 ? (
        <div
          className="flex h-[240px] items-center justify-center text-sm text-muted-foreground"
          data-testid="spending-trend-empty"
        >
          {t("noChartData")}
        </div>
      ) : (
        <ResponsiveContainer width="100%" height={240}>
          <LineChart data={filtered}>
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
            <ReferenceLine
              y={average}
              stroke="var(--muted-foreground)"
              strokeDasharray="4 4"
              label={{
                value: t("trendAverageReference"),
                fontSize: 10,
                fill: "var(--muted-foreground)",
                position: "insideTopRight",
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
      )}
    </div>
  );
}
