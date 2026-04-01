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
import { useAutomationStore } from "@/lib/stores/automation-store";

const EVENT_TYPES = [
  "task.status_changed",
  "task.assignee_changed",
  "task.field_changed",
  "task.due_date_approaching",
  "review.completed",
  "budget.threshold_reached",
];

export function RuleEditor({ projectId }: { projectId: string }) {
  const t = useTranslations("settings");
  const createRule = useAutomationStore((state) => state.createRule);
  const [name, setName] = useState("");
  const [eventType, setEventType] = useState(EVENT_TYPES[0]);
  const [conditionField, setConditionField] = useState("status");
  const [conditionValue, setConditionValue] = useState("done");
  const [actionType, setActionType] = useState("send_notification");

  return (
    <div className="space-y-4">
      <div className="space-y-2">
        <Label>{t("automations.ruleName")}</Label>
        <Input value={name} onChange={(event) => setName(event.target.value)} placeholder={t("automations.ruleNamePlaceholder")} />
      </div>
      <div className="grid gap-3 md:grid-cols-3">
        <div className="space-y-2">
          <Label>{t("automations.event")}</Label>
          <Select value={eventType} onValueChange={setEventType}>
            <SelectTrigger>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {EVENT_TYPES.map((eventTypeOption) => (
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
            {["send_notification", "send_im_message", "update_field", "assign_user", "move_to_column", "create_subtask", "invoke_plugin"].map((type) => (
              <SelectItem key={type} value={type}>
                {type}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>
      <Button
        type="button"
        disabled={!name.trim()}
        onClick={() =>
          void createRule(projectId, {
            name,
            enabled: true,
            eventType,
            conditions: [{ field: conditionField, op: "eq", value: conditionValue }],
            actions: [{ type: actionType, config: {} }],
          })
        }
      >
        {t("automations.createRule")}
      </Button>
    </div>
  );
}
