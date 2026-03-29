"use client";

import { useEffect } from "react";
import { useTranslations } from "next-intl";
import { Button } from "@/components/ui/button";
import { useAutomationStore } from "@/lib/stores/automation-store";

export function RuleList({ projectId }: { projectId: string }) {
  const t = useTranslations("settings");
  const rules = useAutomationStore((state) => state.rulesByProject[projectId] ?? []);
  const fetchRules = useAutomationStore((state) => state.fetchRules);
  const updateRule = useAutomationStore((state) => state.updateRule);
  const deleteRule = useAutomationStore((state) => state.deleteRule);

  useEffect(() => {
    void fetchRules(projectId);
  }, [fetchRules, projectId]);

  return (
    <div className="space-y-2">
      {rules.map((rule) => (
        <div key={rule.id} className="flex items-center justify-between rounded-md border px-3 py-2 text-sm">
          <div>
            <div className="font-medium">{rule.name}</div>
            <div className="text-muted-foreground">{rule.eventType}</div>
          </div>
          <div className="flex items-center gap-2">
            <Button
              type="button"
              size="sm"
              variant="outline"
              onClick={() => void updateRule(projectId, rule.id, { enabled: !rule.enabled })}
            >
              {rule.enabled ? t("automations.disable") : t("automations.enable")}
            </Button>
            <Button type="button" size="sm" variant="destructive" onClick={() => void deleteRule(projectId, rule.id)}>
              {t("automations.delete")}
            </Button>
          </div>
        </div>
      ))}
    </div>
  );
}
