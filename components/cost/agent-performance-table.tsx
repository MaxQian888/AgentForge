"use client";

import { Badge } from "@/components/ui/badge";

export interface AgentPerformanceRecord {
  agentId: string;
  agentName: string;
  taskCount: number;
  successRate: number;
  avgCostUsd: number;
  avgDurationMinutes: number;
  totalCostUsd: number;
}

interface AgentPerformanceTableProps {
  data: AgentPerformanceRecord[];
}

export function AgentPerformanceTable({ data }: AgentPerformanceTableProps) {
  if (data.length === 0) {
    return (
      <p className="text-sm text-muted-foreground">
        No agent performance data available yet.
      </p>
    );
  }

  return (
    <div className="overflow-x-auto">
      <table className="w-full text-sm">
        <thead>
          <tr className="border-b text-left text-muted-foreground">
            <th className="pb-2 pr-4 font-medium">Agent</th>
            <th className="pb-2 pr-4 font-medium">Tasks</th>
            <th className="pb-2 pr-4 font-medium">Success</th>
            <th className="pb-2 pr-4 font-medium">Avg Cost</th>
            <th className="pb-2 pr-4 font-medium">Avg Duration</th>
            <th className="pb-2 font-medium">Total Cost</th>
          </tr>
        </thead>
        <tbody>
          {data.map((agent) => (
            <tr key={agent.agentId} className="border-b border-border/40">
              <td className="py-2 pr-4 font-medium">{agent.agentName}</td>
              <td className="py-2 pr-4">{agent.taskCount}</td>
              <td className="py-2 pr-4">
                <Badge
                  variant={agent.successRate >= 0.8 ? "secondary" : "destructive"}
                >
                  {Math.round(agent.successRate * 100)}%
                </Badge>
              </td>
              <td className="py-2 pr-4">${agent.avgCostUsd.toFixed(2)}</td>
              <td className="py-2 pr-4">{agent.avgDurationMinutes.toFixed(0)}m</td>
              <td className="py-2">${agent.totalCostUsd.toFixed(2)}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
