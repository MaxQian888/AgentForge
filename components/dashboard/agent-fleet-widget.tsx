"use client";

import { useTranslations } from "next-intl";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { StatusDot } from "@/components/shared/status-dot";
import type { Agent } from "@/lib/stores/agent-store";

interface AgentFleetWidgetProps {
  agents: Agent[];
}

function formatCost(value: number): string {
  return `$${value.toFixed(2)}`;
}

export function AgentFleetWidget({ agents }: AgentFleetWidgetProps) {
  const t = useTranslations("dashboard");

  return (
    <Card>
      <CardHeader className="pb-3">
        <CardTitle className="text-sm font-medium">
          {t("agentFleet.title")}
        </CardTitle>
      </CardHeader>
      <CardContent>
        {agents.length === 0 ? (
          <p className="text-sm text-muted-foreground">
            {t("agentFleet.empty")}
          </p>
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead className="text-xs">{t("agentFleet.name")}</TableHead>
                <TableHead className="text-xs">{t("agentFleet.role")}</TableHead>
                <TableHead className="text-xs">{t("agentFleet.task")}</TableHead>
                <TableHead className="text-xs">{t("agentFleet.status")}</TableHead>
                <TableHead className="text-right text-xs">{t("agentFleet.cost")}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {agents.map((agent) => (
                <TableRow key={agent.id}>
                  <TableCell className="text-sm">{agent.memberId}</TableCell>
                  <TableCell className="text-sm text-muted-foreground">
                    {agent.roleName}
                  </TableCell>
                  <TableCell className="max-w-[150px] truncate text-sm text-muted-foreground">
                    {agent.taskTitle}
                  </TableCell>
                  <TableCell>
                    <div className="flex items-center gap-1.5">
                      <StatusDot status={agent.status} size="sm" />
                      <span className="text-sm">{agent.status}</span>
                    </div>
                  </TableCell>
                  <TableCell className="text-right text-sm">
                    {formatCost(agent.cost)}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        )}
      </CardContent>
    </Card>
  );
}
