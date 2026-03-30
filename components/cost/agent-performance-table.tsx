"use client";

import { useTranslations } from "next-intl";
import { Badge } from "@/components/ui/badge";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { EmptyState } from "@/components/shared/empty-state";
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
    <div className="overflow-x-auto">
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>{t("colBucket")}</TableHead>
            <TableHead>{t("colRuns")}</TableHead>
            <TableHead>{t("colSuccess")}</TableHead>
            <TableHead>{t("colAvgCost")}</TableHead>
            <TableHead>{t("colAvgDuration")}</TableHead>
            <TableHead>{t("colTotalCost")}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {data.map((agent) => (
            <TableRow key={agent.bucketId}>
              <TableCell className="font-medium">{agent.label}</TableCell>
              <TableCell>{agent.runCount}</TableCell>
              <TableCell>
                <Badge
                  variant={agent.successRate >= 0.8 ? "secondary" : "destructive"}
                >
                  {Math.round(agent.successRate * 100)}%
                </Badge>
              </TableCell>
              <TableCell>${agent.avgCostUsd.toFixed(2)}</TableCell>
              <TableCell>{agent.avgDurationMinutes.toFixed(0)}m</TableCell>
              <TableCell>${agent.totalCostUsd.toFixed(2)}</TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  );
}
