"use client";

import React from "react";
import { useEditor } from "../../context";
import { findPredecessors } from "../../hooks/use-data-flow";
import { ConditionBuilder } from "../condition-builder";

// ── Types ─────────────────────────────────────────────────────────────────────

interface ConditionConfigProps {
  config: Record<string, unknown>;
  onChange: (config: Record<string, unknown>) => void;
  nodeId: string;
}

// ── Component ─────────────────────────────────────────────────────────────────

export function ConditionConfig({
  config,
  onChange,
  nodeId,
}: ConditionConfigProps) {
  const { state } = useEditor();
  const { nodes, edges } = state;

  const predecessors = findPredecessors(nodeId, nodes, edges);

  const upstreamNodes = predecessors.map((n) => ({
    id: n.id,
    label: (n.data.label as string | undefined) ?? n.id,
    type: n.type ?? "unknown",
  }));

  const expression = (config.expression as string | undefined) ?? "";

  function handleChange(expr: string) {
    onChange({ ...config, expression: expr });
  }

  return (
    <ConditionBuilder
      value={expression}
      onChange={handleChange}
      upstreamNodes={upstreamNodes}
    />
  );
}
