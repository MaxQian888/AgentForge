"use client";

import { useState } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
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
  const createRule = useAutomationStore((state) => state.createRule);
  const [name, setName] = useState("");
  const [eventType, setEventType] = useState(EVENT_TYPES[0]);
  const [conditionField, setConditionField] = useState("status");
  const [conditionValue, setConditionValue] = useState("done");
  const [actionType, setActionType] = useState("send_notification");

  return (
    <div className="space-y-4">
      <div className="space-y-2">
        <Label>Rule name</Label>
        <Input value={name} onChange={(event) => setName(event.target.value)} placeholder="Notify when done" />
      </div>
      <div className="grid gap-3 md:grid-cols-3">
        <div className="space-y-2">
          <Label>Event</Label>
          <select className="h-10 rounded-md border bg-background px-3 text-sm" value={eventType} onChange={(event) => setEventType(event.target.value)}>
            {EVENT_TYPES.map((eventTypeOption) => (
              <option key={eventTypeOption} value={eventTypeOption}>
                {eventTypeOption}
              </option>
            ))}
          </select>
        </div>
        <div className="space-y-2">
          <Label>Condition field</Label>
          <Input value={conditionField} onChange={(event) => setConditionField(event.target.value)} />
        </div>
        <div className="space-y-2">
          <Label>Condition value</Label>
          <Input value={conditionValue} onChange={(event) => setConditionValue(event.target.value)} />
        </div>
      </div>
      <div className="space-y-2">
        <Label>Action</Label>
        <select className="h-10 rounded-md border bg-background px-3 text-sm" value={actionType} onChange={(event) => setActionType(event.target.value)}>
          {["send_notification", "send_im_message", "update_field", "assign_user", "move_to_column", "create_subtask", "invoke_plugin"].map((type) => (
            <option key={type} value={type}>
              {type}
            </option>
          ))}
        </select>
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
        Create rule
      </Button>
    </div>
  );
}
