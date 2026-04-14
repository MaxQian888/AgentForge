"use client";

import React, { useState } from "react";
import { toast } from "sonner";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { RadioGroup, RadioGroupItem } from "@/components/ui/radio-group";
import { Label } from "@/components/ui/label";
import { cn } from "@/lib/utils";

// ── Pure utility functions ────────────────────────────────────────────────────

/**
 * Serializes individual parts into a `{{nodeId.field}} operator value` string.
 */
export function serializeVisualRule(
  nodeId: string,
  field: string,
  operator: string,
  value: string
): string {
  return `{{${nodeId}.${field}}} ${operator} ${value}`;
}

const EXPRESSION_RE = /^\{\{(\w+)\.([^}]+)\}\}\s*(==|!=|>=?|<=?)\s*(.+)$/;

/**
 * Parses an expression of the form `{{nodeId.field}} op value`.
 * Returns the decomposed parts, or null when the expression is too complex.
 */
export function parseExpression(
  expr: string
): { nodeId: string; field: string; operator: string; value: string } | null {
  const match = expr.trim().match(EXPRESSION_RE);
  if (!match) return null;
  const [, nodeId, field, operator, value] = match;
  return { nodeId, field, operator, value: value.trim() };
}

// ── Types ─────────────────────────────────────────────────────────────────────

export interface ConditionBuilderProps {
  value: string;
  onChange: (expr: string) => void;
  upstreamNodes: { id: string; label: string; type: string }[];
}

const OPERATORS = ["==", "!=", ">", "<", ">=", "<="] as const;

// ── Component ─────────────────────────────────────────────────────────────────

export function ConditionBuilder({
  value,
  onChange,
  upstreamNodes,
}: ConditionBuilderProps) {
  const parsed = parseExpression(value);

  const [mode, setMode] = useState<"visual" | "expression">(
    parsed ? "visual" : "expression"
  );

  // Visual mode state
  const [nodeId, setNodeId] = useState(parsed?.nodeId ?? "");
  const [field, setField] = useState(parsed?.field ?? "");
  const [operator, setOperator] = useState(parsed?.operator ?? "==");
  const [ruleValue, setRuleValue] = useState(parsed?.value ?? "");

  // ── Handlers ────────────────────────────────────────────────────────────────

  function handleModeChange(next: "visual" | "expression") {
    if (next === "expression") {
      // Serialize current visual state into the expression
      if (nodeId && field && ruleValue) {
        onChange(serializeVisualRule(nodeId, field, operator, ruleValue));
      }
      setMode("expression");
      return;
    }

    // Switching visual → visual: try to parse current expression value
    const p = parseExpression(value);
    if (!p) {
      toast.error("Expression too complex for visual mode");
      return;
    }
    setNodeId(p.nodeId);
    setField(p.field);
    setOperator(p.operator);
    setRuleValue(p.value);
    setMode("visual");
  }

  function handleNodeChange(id: string) {
    setNodeId(id);
    if (id && field && ruleValue) {
      onChange(serializeVisualRule(id, field, operator, ruleValue));
    }
  }

  function handleFieldChange(f: string) {
    setField(f);
    if (nodeId && f && ruleValue) {
      onChange(serializeVisualRule(nodeId, f, operator, ruleValue));
    }
  }

  function handleOperatorChange(op: string) {
    setOperator(op);
    if (nodeId && field && ruleValue) {
      onChange(serializeVisualRule(nodeId, field, op, ruleValue));
    }
  }

  function handleValueChange(v: string) {
    setRuleValue(v);
    if (nodeId && field && v) {
      onChange(serializeVisualRule(nodeId, field, operator, v));
    }
  }

  // ── Render ──────────────────────────────────────────────────────────────────

  const previewExpr =
    nodeId && field && ruleValue
      ? serializeVisualRule(nodeId, field, operator, ruleValue)
      : "";

  return (
    <div className={cn("flex flex-col gap-3")}>
      {/* Mode toggle */}
      <RadioGroup
        value={mode}
        onValueChange={(v) => handleModeChange(v as "visual" | "expression")}
        className="flex flex-row gap-4"
      >
        <div className="flex items-center gap-1.5">
          <RadioGroupItem value="visual" id="cb-mode-visual" />
          <Label htmlFor="cb-mode-visual" className="cursor-pointer">
            Visual
          </Label>
        </div>
        <div className="flex items-center gap-1.5">
          <RadioGroupItem value="expression" id="cb-mode-expression" />
          <Label htmlFor="cb-mode-expression" className="cursor-pointer">
            Expression
          </Label>
        </div>
      </RadioGroup>

      {mode === "visual" ? (
        <div className="flex flex-col gap-2">
          {/* Node selector */}
          <Select value={nodeId} onValueChange={handleNodeChange}>
            <SelectTrigger>
              <SelectValue placeholder="Select upstream node…" />
            </SelectTrigger>
            <SelectContent>
              {upstreamNodes.map((n) => (
                <SelectItem key={n.id} value={n.id}>
                  {n.label}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>

          {/* Field input */}
          <Input
            placeholder="Field path (e.g. output.ok)"
            value={field}
            onChange={(e) => handleFieldChange(e.target.value)}
          />

          {/* Operator selector */}
          <Select value={operator} onValueChange={handleOperatorChange}>
            <SelectTrigger>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {OPERATORS.map((op) => (
                <SelectItem key={op} value={op}>
                  {op}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>

          {/* Value input */}
          <Input
            placeholder="Value"
            value={ruleValue}
            onChange={(e) => handleValueChange(e.target.value)}
          />

          {/* Live preview */}
          {previewExpr && (
            <p className="rounded bg-muted px-2 py-1 font-mono text-xs text-muted-foreground">
              {previewExpr}
            </p>
          )}
        </div>
      ) : (
        <div className="flex flex-col gap-1.5">
          <Textarea
            value={value}
            onChange={(e) => onChange(e.target.value)}
            rows={3}
            className="font-mono text-sm"
            placeholder="{{node.output.field}} == value"
          />
          <p className="text-xs text-muted-foreground">
            Use{" "}
            <code className="rounded bg-muted px-1 font-mono">
              {"{{node.output.field}}"}
            </code>{" "}
            to reference upstream data
          </p>
        </div>
      )}
    </div>
  );
}
