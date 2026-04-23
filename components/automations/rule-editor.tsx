"use client";

import { useState } from "react";
import { useTranslations } from "next-intl";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  AUTOMATION_ACTION_TYPES,
  AUTOMATION_EVENT_TYPES,
  useAutomationStore,
} from "@/lib/stores/automation-store";

export function RuleEditor({ projectId }: { projectId: string }) {
  const t = useTranslations("settings");
  const createRule = useAutomationStore((state) => state.createRule);
  const [name, setName] = useState("");
  const [eventType, setEventType] = useState<(typeof AUTOMATION_EVENT_TYPES)[number]>(
    AUTOMATION_EVENT_TYPES[0]
  );
  const [conditionField, setConditionField] = useState("status");
  const [conditionValue, setConditionValue] = useState("done");
  const [actionType, setActionType] = useState("send_notification");
  const [workflowPluginId, setWorkflowPluginId] = useState("");

  const actionConfig =
    actionType === "start_workflow"
      ? { pluginId: workflowPluginId.trim() }
      : {};

  const handleEventTypeChange = (value: string) => {
    if ((AUTOMATION_EVENT_TYPES as readonly string[]).includes(value)) {
      setEventType(value as (typeof AUTOMATION_EVENT_TYPES)[number]);
    }
  };

  return (
    <div className="space-y-4">
      <div className="space-y-2">
        <Label>{t("automations.ruleName")}</Label>
        <Input value={name} onChange={(event) => setName(event.target.value)} placeholder={t("automations.ruleNamePlaceholder")} />
      </div>
      <div className="grid gap-3 md:grid-cols-3">
        <div className="space-y-2">
          <Label>{t("automations.event")}</Label>
          <Select value={eventType} onValueChange={handleEventTypeChange}>
            <SelectTrigger>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {AUTOMATION_EVENT_TYPES.map((eventTypeOption) => (
                <SelectItem key={eventTypeOption} value={eventTypeOption}>
                  {eventTypeOption}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
        <div className="space-y-2">
          <Label>{t("automations.conditionField")}</Label>
          <Input value={conditionField} onChange={(event) => setConditionField(event.target.value)} />
        </div>
        <div className="space-y-2">
          <Label>{t("automations.conditionValue")}</Label>
          <Input value={conditionValue} onChange={(event) => setConditionValue(event.target.value)} />
        </div>
      </div>
      <div className="space-y-2">
        <Label>{t("automations.action")}</Label>
        <Select value={actionType} onValueChange={setActionType}>
          <SelectTrigger>
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {AUTOMATION_ACTION_TYPES.map((type) => (
              <SelectItem key={type} value={type}>
                {type}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>
      {actionType === "start_workflow" ? (
        <div className="space-y-2">
          <Label>{t("automations.workflowPluginId")}</Label>
          <Input
            value={workflowPluginId}
            onChange={(event) => setWorkflowPluginId(event.target.value)}
            placeholder="task-delivery-flow"
          />
          <p className="text-xs text-muted-foreground">
            {t("automations.workflowPluginHelp")}
          </p>
        </div>
      ) : null}
      <Button
        type="button"
        disabled={!name.trim() || (actionType === "start_workflow" && workflowPluginId.trim() === "")}
        onClick={() =>
          void createRule(projectId, {
            name,
            enabled: true,
            eventType,
            conditions: [{ field: conditionField, op: "eq", value: conditionValue }],
            actions: [{ type: actionType, config: actionConfig }],
          })
        }
      >
        {t("automations.createRule")}
      </Button>
    </div>
  );
}
