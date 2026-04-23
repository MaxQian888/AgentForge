"use client";

import React from "react";
import { X, Trash2 } from "lucide-react";
import { useTranslations } from "next-intl";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Separator } from "@/components/ui/separator";
import { useEditor } from "../context";
import { findPredecessors } from "../hooks/use-data-flow";
import { ConditionBuilder } from "./condition-builder";

// ── Component ─────────────────────────────────────────────────────────────────

export function EdgeConfigPanel() {
  const { state, dispatch } = useEditor();
  const { selectedEdgeId, edges, nodes } = state;
  const t = useTranslations("workflow");

  if (!selectedEdgeId) return null;

  const edge = edges.find((e) => e.id === selectedEdgeId);
  if (!edge) return null;

  const sourceNode = nodes.find((n) => n.id === edge.source);
  const targetNode = nodes.find((n) => n.id === edge.target);

  const sourceLabel =
    (sourceNode?.data?.label as string | undefined) ?? edge.source;
  const targetLabel =
    (targetNode?.data?.label as string | undefined) ?? edge.target;

  const edgeLabel =
    typeof edge.label === "string"
      ? edge.label
      : (edge.data?.label as string | undefined) ?? "";
  const condition = (edge.data?.condition as string | undefined) ?? "";

  // Upstream nodes for the ConditionBuilder = predecessors of source + source itself
  const sourcePredecessors = sourceNode
    ? findPredecessors(sourceNode.id, nodes, edges)
    : [];

  const upstreamNodes = [
    ...(sourceNode
      ? [
          {
            id: sourceNode.id,
            label:
              (sourceNode.data?.label as string | undefined) ?? sourceNode.id,
            type: sourceNode.type ?? "unknown",
          },
        ]
      : []),
    ...sourcePredecessors.map((n) => ({
      id: n.id,
      label: (n.data?.label as string | undefined) ?? n.id,
      type: n.type ?? "unknown",
    })),
  ];

  function handleLabelChange(value: string) {
    dispatch({
      type: "UPDATE_EDGE_CONDITION",
      edgeId: selectedEdgeId!,
      condition,
      label: value,
    });
  }

  function handleConditionChange(expr: string) {
    dispatch({
      type: "UPDATE_EDGE_CONDITION",
      edgeId: selectedEdgeId!,
      condition: expr,
      label: edgeLabel || undefined,
    });
  }

  function handleDelete() {
    dispatch({ type: "DELETE_EDGE", edgeId: selectedEdgeId! });
  }

  function handleClose() {
    dispatch({ type: "DESELECT" });
  }

  return (
    <div className="flex h-full flex-col">
      {/* Header */}
      <div className="flex items-center justify-between border-b px-4 py-3">
        <div className="flex flex-col gap-0.5 min-w-0">
          <p className="text-xs text-muted-foreground">{t("nodeConfig.edge.title")}</p>
          <p className="text-sm font-medium truncate">
            {sourceLabel} → {targetLabel}
          </p>
        </div>
        <Button
          variant="ghost"
          size="icon"
          className="h-7 w-7 shrink-0"
          onClick={handleClose}
        >
          <X className="h-4 w-4" />
        </Button>
      </div>

      {/* Body */}
      <div className="flex-1 overflow-y-auto">
        <div className="flex flex-col gap-4 p-4">
          {/* Label */}
          <div className="flex flex-col gap-1.5">
            <Label className="text-xs">{t("nodeConfig.edge.displayLabel")}</Label>
            <Input
              placeholder={t("nodeConfig.edge.displayLabelPlaceholder")}
              value={edgeLabel}
              onChange={(e) => handleLabelChange(e.target.value)}
            />
          </div>

          <Separator />

          {/* Condition */}
          <div className="flex flex-col gap-2">
            <Label className="text-xs">{t("nodeConfig.edge.condition")}</Label>
            <ConditionBuilder
              value={condition}
              onChange={handleConditionChange}
              upstreamNodes={upstreamNodes}
            />
          </div>
        </div>
      </div>

      {/* Footer */}
      <div className="border-t px-4 py-3">
        <Button
          variant="destructive"
          size="sm"
          className="w-full"
          onClick={handleDelete}
        >
          <Trash2 className="mr-2 h-4 w-4" />
          {t("nodeConfig.edge.deleteEdge")}
        </Button>
      </div>
    </div>
  );
}
