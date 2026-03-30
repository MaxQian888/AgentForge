"use client";

import { useEffect } from "react";
import { useTranslations } from "next-intl";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { useRoleStore } from "@/lib/stores/role-store";
import { useAgentStore } from "@/lib/stores/agent-store";

const AUTO_ASSIGN = "__auto__";

interface StepAgentsProps {
  selectedRoleIds: string[];
  agentAssignments: Record<string, string>;
  onChange: (assignments: Record<string, string>) => void;
}

export function StepAgents({
  selectedRoleIds,
  agentAssignments,
  onChange,
}: StepAgentsProps) {
  const t = useTranslations("teams");
  const roles = useRoleStore((s) => s.roles);
  const { agents, fetchAgents } = useAgentStore();

  useEffect(() => {
    if (agents.length === 0) {
      void fetchAgents();
    }
  }, [agents.length, fetchAgents]);

  const availableAgents = agents.filter(
    (a) => a.status === "completed" || a.status === "paused" || a.status === "running"
  );

  const selectedRoles = roles.filter((r) => selectedRoleIds.includes(r.metadata.id));

  if (selectedRoles.length === 0) {
    return (
      <p className="text-sm text-muted-foreground">{t("wizard.noRolesSelected")}</p>
    );
  }

  return (
    <div className="flex flex-col gap-4">
      <p className="text-sm text-muted-foreground">{t("wizard.agentsHint")}</p>
      {selectedRoles.map((role) => {
        const roleId = role.metadata.id;
        const currentValue = agentAssignments[roleId] ?? AUTO_ASSIGN;

        return (
          <div key={roleId} className="flex flex-col gap-2">
            <Label>{role.metadata.name}</Label>
            <Select
              value={currentValue}
              onValueChange={(value) =>
                onChange({ ...agentAssignments, [roleId]: value })
              }
            >
              <SelectTrigger className="w-full">
                <SelectValue placeholder={t("wizard.selectAgent")} />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value={AUTO_ASSIGN}>
                  {t("wizard.autoAssign")}
                </SelectItem>
                {availableAgents.map((agent) => (
                  <SelectItem key={agent.id} value={agent.id}>
                    {agent.roleName} ({agent.id.slice(0, 8)})
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
        );
      })}
    </div>
  );
}

export { AUTO_ASSIGN };
