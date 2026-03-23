"use client";

import { DollarSign, TrendingUp, Calendar } from "lucide-react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { CostChart } from "@/components/cost/cost-chart";
import { useAgentStore } from "@/lib/stores/agent-store";

// Placeholder data for the chart until real data is fetched
const placeholderChartData = [
  { date: "Mon", cost: 0 },
  { date: "Tue", cost: 0 },
  { date: "Wed", cost: 0 },
  { date: "Thu", cost: 0 },
  { date: "Fri", cost: 0 },
  { date: "Sat", cost: 0 },
  { date: "Sun", cost: 0 },
];

export default function CostPage() {
  const agents = useAgentStore((s) => s.agents);
  const totalCost = agents.reduce((sum, a) => sum + a.cost, 0);

  return (
    <div className="flex flex-col gap-6">
      <h1 className="text-2xl font-bold">Cost Overview</h1>

      <div className="grid gap-4 sm:grid-cols-3">
        <Card>
          <CardHeader className="flex flex-row items-center justify-between pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">
              Total Spend
            </CardTitle>
            <DollarSign className="size-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">${totalCost.toFixed(2)}</div>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="flex flex-row items-center justify-between pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">
              This Week
            </CardTitle>
            <TrendingUp className="size-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">${totalCost.toFixed(2)}</div>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="flex flex-row items-center justify-between pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">
              This Month
            </CardTitle>
            <Calendar className="size-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">${totalCost.toFixed(2)}</div>
          </CardContent>
        </Card>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>Cost Over Time</CardTitle>
        </CardHeader>
        <CardContent>
          <CostChart data={placeholderChartData} />
        </CardContent>
      </Card>
    </div>
  );
}
