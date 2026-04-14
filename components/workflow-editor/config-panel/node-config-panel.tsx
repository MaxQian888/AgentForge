"use client";

import React, { useRef } from "react";
import { X, Trash2 } from "lucide-react";
import {
  Accordion,
  AccordionContent,
  AccordionItem,
  AccordionTrigger,
} from "@/components/ui/accordion";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Switch } from "@/components/ui/switch";
import { Label } from "@/components/ui/label";
import { ScrollArea } from "@/components/ui/scroll-area";
import { Separator } from "@/components/ui/separator";
import { useEditor } from "../context";
import { getNodeMeta } from "../nodes/node-registry";
import type { ConfigField } from "../types";
import { ConditionBuilder } from "./condition-builder";
import { DataFlowPreview } from "./data-flow-preview";
import { LlmAgentConfig } from "./node-configs/llm-agent-config";
import { ConditionConfig } from "./node-configs/condition-config";
import { SubWorkflowConfig } from "./node-configs/sub-workflow-config";

// ── Helpers ───────────────────────────────────────────────────────────────────

/** Groups ConfigField[] by their `group` property */
function groupFields(fields: ConfigField[]): Map<string, ConfigField[]> {
  const map = new Map<string, ConfigField[]>();
  for (const f of fields) {
    const existing = map.get(f.group);
    if (existing) {
      existing.push(f);
    } else {
      map.set(f.group, [f]);
    }
  }
  return map;
}

// ── Schema-driven field renderer ──────────────────────────────────────────────

interface FieldRendererProps {
  field: ConfigField;
  config: Record<string, unknown>;
  onChange: (config: Record<string, unknown>) => void;
}

function FieldRenderer({ field, config, onChange }: FieldRendererProps) {
  const rawValue = config[field.key];

  function update(value: unknown) {
    onChange({ ...config, [field.key]: value });
  }

  switch (field.type) {
    case "text":
      return (
        <div className="flex flex-col gap-1.5">
          <Label className="text-xs">{field.label}</Label>
          <Input
            placeholder={field.placeholder}
            value={(rawValue as string | undefined) ?? ""}
            onChange={(e) => update(e.target.value)}
          />
        </div>
      );

    case "textarea":
      return (
        <div className="flex flex-col gap-1.5">
          <Label className="text-xs">{field.label}</Label>
          <Textarea
            placeholder={field.placeholder}
            rows={3}
            value={(rawValue as string | undefined) ?? ""}
            onChange={(e) => update(e.target.value)}
          />
        </div>
      );

    case "json":
      return (
        <div className="flex flex-col gap-1.5">
          <Label className="text-xs">{field.label}</Label>
          <Textarea
            placeholder={field.placeholder}
            rows={4}
            className="font-mono text-xs"
            value={(rawValue as string | undefined) ?? ""}
            onChange={(e) => update(e.target.value)}
          />
        </div>
      );

    case "select":
      return (
        <div className="flex flex-col gap-1.5">
          <Label className="text-xs">{field.label}</Label>
          <Select
            value={(rawValue as string | undefined) ?? ""}
            onValueChange={(v) => update(v)}
          >
            <SelectTrigger>
              <SelectValue placeholder={`Select ${field.label}…`} />
            </SelectTrigger>
            <SelectContent>
              {(field.options ?? []).map((opt) => (
                <SelectItem key={opt} value={opt}>
                  {opt}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
      );

    case "number":
      return (
        <div className="flex flex-col gap-1.5">
          <Label className="text-xs">{field.label}</Label>
          <Input
            type="number"
            placeholder={field.placeholder}
            value={(rawValue as string | number | undefined) ?? ""}
            onChange={(e) => update(e.target.value)}
          />
        </div>
      );

    case "boolean":
      return (
        <div className="flex items-center justify-between gap-2">
          <Label className="text-xs">{field.label}</Label>
          <Switch
            checked={Boolean(rawValue)}
            onCheckedChange={(checked) => update(checked)}
          />
        </div>
      );

    case "expression": {
      // ConditionBuilder needs upstream context — but in schema-driven mode
      // we don't have it readily available. Render a simple textarea fallback
      // (full ConditionBuilder is only used via the ConditionConfig override).
      return (
        <div className="flex flex-col gap-1.5">
          <Label className="text-xs">{field.label}</Label>
          <ConditionBuilder
            value={(rawValue as string | undefined) ?? ""}
            onChange={(expr) => update(expr)}
            upstreamNodes={[]}
          />
        </div>
      );
    }

    default:
      return null;
  }
}

// ── Main component ────────────────────────────────────────────────────────────

export function NodeConfigPanel() {
  const { state, dispatch } = useEditor();
  const { selectedNodeId, nodes } = state;

  // useRef must be called unconditionally before any early return
  const advancedRef = useRef<HTMLTextAreaElement>(null);

  if (!selectedNodeId) return null;

  const node = nodes.find((n) => n.id === selectedNodeId);
  if (!node) return null;

  const meta = getNodeMeta(node.type ?? "");
  const config = (node.data?.config as Record<string, unknown> | undefined) ?? {};
  const label = (node.data?.label as string | undefined) ?? "";

  // selectedNodeId is guaranteed non-null here (we returned early above)
  const nodeId = selectedNodeId as string;

  // Determine which (if any) custom override to render
  const hasCustomOverride =
    node.type === "llm_agent" ||
    node.type === "condition" ||
    node.type === "sub_workflow";

  // Grouped schema fields (used when there is no custom override)
  const fieldGroups = groupFields(meta?.configSchema ?? []);
  const groupNames = Array.from(fieldGroups.keys());

  function handleLabelChange(value: string) {
    dispatch({ type: "UPDATE_NODE_LABEL", nodeId, label: value });
  }

  function handleConfigChange(partial: Record<string, unknown>) {
    dispatch({
      type: "UPDATE_NODE_CONFIG",
      nodeId,
      config: partial,
    });
  }

  function handleClose() {
    dispatch({ type: "DESELECT" });
  }

  function handleDelete() {
    dispatch({ type: "DELETE_NODES", nodeIds: [nodeId] });
  }

  function handleAdvancedBlur() {
    if (!advancedRef.current) return;
    try {
      const parsed = JSON.parse(advancedRef.current.value) as Record<
        string,
        unknown
      >;
      dispatch({
        type: "UPDATE_NODE_CONFIG",
        nodeId,
        config: parsed,
      });
    } catch {
      // Ignore invalid JSON — keep the raw text as a visual indicator
    }
  }

  // Collect all accordion default-open values
  const defaultOpen = [
    ...(hasCustomOverride ? ["type-config"] : groupNames.map((g) => `group-${g}`)),
    "data-flow",
    "advanced",
  ];

  const NodeIcon = meta?.icon;

  return (
    <div className="flex h-full flex-col">
      {/* Header */}
      <div className="flex items-center gap-2 border-b px-4 py-3">
        {NodeIcon && (
          <NodeIcon
            className="h-4 w-4 shrink-0"
            style={{ color: meta?.color }}
          />
        )}
        <Input
          className="h-7 flex-1 border-none bg-transparent p-0 text-sm font-medium shadow-none focus-visible:ring-0"
          value={label}
          onChange={(e) => handleLabelChange(e.target.value)}
          aria-label="Node label"
        />
        <Button
          variant="ghost"
          size="icon"
          className="h-7 w-7 shrink-0"
          onClick={handleClose}
        >
          <X className="h-4 w-4" />
        </Button>
      </div>

      {/* Scrollable body */}
      <ScrollArea className="flex-1">
        <Accordion
          type="multiple"
          defaultValue={defaultOpen}
          className="px-4 pb-2"
        >
          {/* ── Type-specific config ──────────────────────────────────────────── */}
          {hasCustomOverride ? (
            <AccordionItem value="type-config">
              <AccordionTrigger className="text-xs font-semibold uppercase tracking-wide">
                {meta?.label ?? node.type} Config
              </AccordionTrigger>
              <AccordionContent>
                <div className="pb-2 pt-1">
                  {node.type === "llm_agent" && (
                    <LlmAgentConfig
                      config={config}
                      onChange={handleConfigChange}
                    />
                  )}
                  {node.type === "condition" && (
                    <ConditionConfig
                      config={config}
                      onChange={handleConfigChange}
                      nodeId={nodeId}
                    />
                  )}
                  {node.type === "sub_workflow" && (
                    <SubWorkflowConfig
                      config={config}
                      onChange={handleConfigChange}
                    />
                  )}
                </div>
              </AccordionContent>
            </AccordionItem>
          ) : (
            /* Schema-driven groups */
            groupNames.map((groupName) => (
              <AccordionItem key={groupName} value={`group-${groupName}`}>
                <AccordionTrigger className="text-xs font-semibold uppercase tracking-wide">
                  {groupName}
                </AccordionTrigger>
                <AccordionContent>
                  <div className="flex flex-col gap-3 pb-2 pt-1">
                    {(fieldGroups.get(groupName) ?? []).map((field) => (
                      <FieldRenderer
                        key={field.key}
                        field={field}
                        config={config}
                        onChange={handleConfigChange}
                      />
                    ))}
                  </div>
                </AccordionContent>
              </AccordionItem>
            ))
          )}

          {/* ── Data Flow ────────────────────────────────────────────────────── */}
          <AccordionItem value="data-flow">
            <AccordionTrigger className="text-xs font-semibold uppercase tracking-wide">
              Data Flow
            </AccordionTrigger>
            <AccordionContent>
              <div className="pb-2 pt-1">
                <DataFlowPreview nodeId={nodeId} />
              </div>
            </AccordionContent>
          </AccordionItem>

          {/* ── Advanced ─────────────────────────────────────────────────────── */}
          <AccordionItem value="advanced">
            <AccordionTrigger className="text-xs font-semibold uppercase tracking-wide">
              Advanced (Raw JSON)
            </AccordionTrigger>
            <AccordionContent>
              <div className="pb-2 pt-1">
                <Textarea
                  ref={advancedRef}
                  key={JSON.stringify(config)}
                  defaultValue={JSON.stringify(config, null, 2)}
                  rows={8}
                  className="font-mono text-xs"
                  onBlur={handleAdvancedBlur}
                />
              </div>
            </AccordionContent>
          </AccordionItem>
        </Accordion>
      </ScrollArea>

      {/* Footer */}
      <Separator />
      <div className="px-4 py-3">
        <Button
          variant="destructive"
          size="sm"
          className="w-full"
          onClick={handleDelete}
        >
          <Trash2 className="mr-2 h-4 w-4" />
          Delete Node
        </Button>
      </div>
    </div>
  );
}
