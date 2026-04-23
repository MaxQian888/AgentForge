"use client";

import React, { useState } from "react";
import { Copy, ChevronRight } from "lucide-react";
import { useTranslations } from "next-intl";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from "@/components/ui/collapsible";
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import { useEditor } from "../context";
import {
  findPredecessors,
  getUpstreamOutputFields,
} from "../hooks/use-data-flow";

// ── Types ─────────────────────────────────────────────────────────────────────

interface DataFlowPreviewProps {
  nodeId: string;
}

// ── Component ─────────────────────────────────────────────────────────────────

export function DataFlowPreview({ nodeId }: DataFlowPreviewProps) {
  const { state } = useEditor();
  const { nodes, edges } = state;
  const t = useTranslations("workflow");

  // All predecessors ordered by BFS distance (direct first)
  const predecessors = findPredecessors(nodeId, nodes, edges);

  // Direct: nodes that have a direct edge to the current node
  const directIds = new Set(
    edges.filter((e) => e.target === nodeId).map((e) => e.source)
  );

  const directPredecessors = predecessors.filter((n) => directIds.has(n.id));
  const indirectPredecessors = predecessors.filter((n) => !directIds.has(n.id));

  const [indirectOpen, setIndirectOpen] = useState(false);

  if (predecessors.length === 0) {
    return (
      <p className="text-xs text-muted-foreground">
        {t("nodeConfig.dataFlow.noUpstream")}
      </p>
    );
  }

  return (
    <TooltipProvider>
      <div className="flex flex-col gap-3">
        {/* Direct inputs */}
        {directPredecessors.length > 0 && (
          <div className="flex flex-col gap-2">
            <p className="text-xs font-medium text-foreground">
              {t("nodeConfig.dataFlow.directInputs")}
            </p>
            {directPredecessors.map((n) => (
              <PredecessorBlock key={n.id} node={n} />
            ))}
          </div>
        )}

        {/* Indirect inputs (collapsed by default) */}
        {indirectPredecessors.length > 0 && (
          <Collapsible open={indirectOpen} onOpenChange={setIndirectOpen}>
            <CollapsibleTrigger asChild>
              <button className="flex items-center gap-1 text-xs font-medium text-muted-foreground hover:text-foreground transition-colors">
                <ChevronRight
                  className={`h-3 w-3 transition-transform ${indirectOpen ? "rotate-90" : ""}`}
                />
                {t("nodeConfig.dataFlow.indirect")} ({indirectPredecessors.length})
              </button>
            </CollapsibleTrigger>
            <CollapsibleContent>
              <div className="mt-2 flex flex-col gap-2">
                {indirectPredecessors.map((n) => (
                  <PredecessorBlock key={n.id} node={n} />
                ))}
              </div>
            </CollapsibleContent>
          </Collapsible>
        )}

        {/* Hint */}
        <p className="text-xs text-muted-foreground border-t pt-2">
          {t("nodeConfig.dataFlow.hintPrefix")}{" "}
          <code className="rounded bg-muted px-1 font-mono">
            {"{{node.output.field}}"}
          </code>{" "}
          {t("nodeConfig.dataFlow.hintSuffix")}
        </p>
      </div>
    </TooltipProvider>
  );
}

// ── PredecessorBlock ──────────────────────────────────────────────────────────

interface NodeLike {
  id: string;
  type?: string;
  data: Record<string, unknown>;
}

function PredecessorBlock({ node }: { node: NodeLike }) {
  const t = useTranslations("workflow");
  const nodeType = node.type ?? "unknown";
  const label = (node.data.label as string | undefined) ?? node.id;
  const fields = getUpstreamOutputFields(node.id, nodeType);

  return (
    <div className="rounded border bg-muted/30 p-2 flex flex-col gap-1.5">
      {/* Header */}
      <div className="flex items-center gap-2">
        <Badge variant="secondary" className="text-[10px] px-1.5 py-0">
          {nodeType}
        </Badge>
        <span className="text-xs font-medium truncate">{label}</span>
      </div>

      {/* Output fields */}
      {fields.map((f) => (
        <div key={f.path} className="flex items-center justify-between gap-2">
          <code className="font-mono text-[11px] text-muted-foreground truncate">
            {f.copyTemplate}
          </code>
          <Tooltip>
            <TooltipTrigger asChild>
              <Button
                variant="ghost"
                size="icon"
                className="h-5 w-5 shrink-0"
                onClick={() => {
                  navigator.clipboard.writeText(f.copyTemplate).catch(() => {});
                }}
              >
                <Copy className="h-3 w-3" />
              </Button>
            </TooltipTrigger>
            <TooltipContent side="left">
              <p>{t("nodeConfig.dataFlow.copyReference")}</p>
            </TooltipContent>
          </Tooltip>
        </div>
      ))}
    </div>
  );
}
