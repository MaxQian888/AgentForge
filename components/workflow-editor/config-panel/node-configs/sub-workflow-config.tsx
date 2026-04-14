"use client";

import React from "react";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { Label } from "@/components/ui/label";

// ── Types ─────────────────────────────────────────────────────────────────────

interface SubWorkflowConfigProps {
  config: Record<string, unknown>;
  onChange: (config: Record<string, unknown>) => void;
}

const INPUT_MAPPING_PLACEHOLDER = `{
  "inputKey": "{{nodeId.output.value}}"
}`;

// ── Component ─────────────────────────────────────────────────────────────────

export function SubWorkflowConfig({ config, onChange }: SubWorkflowConfigProps) {
  const workflowId = (config.workflowId as string | undefined) ?? "";
  const inputMapping = (config.inputMapping as string | undefined) ?? "";

  function update(partial: Record<string, unknown>) {
    onChange({ ...config, ...partial });
  }

  return (
    <div className="flex flex-col gap-4">
      {/* Workflow ID */}
      <div className="flex flex-col gap-1.5">
        <Label className="text-xs">Workflow ID</Label>
        <Input
          placeholder="Enter workflow definition ID"
          value={workflowId}
          onChange={(e) => update({ workflowId: e.target.value })}
        />
      </div>

      {/* Input Mapping */}
      <div className="flex flex-col gap-1.5">
        <Label className="text-xs">Input Mapping (JSON)</Label>
        <Textarea
          rows={6}
          placeholder={INPUT_MAPPING_PLACEHOLDER}
          value={inputMapping}
          onChange={(e) => update({ inputMapping: e.target.value })}
          className="font-mono text-xs"
        />
      </div>
    </div>
  );
}
