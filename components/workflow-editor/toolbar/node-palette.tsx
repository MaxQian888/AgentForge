"use client";

import { useState } from "react";
import { ChevronDown, ChevronRight, Search } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from "@/components/ui/collapsible";
import { cn } from "@/lib/utils";
import { NODE_REGISTRY, getNodesByCategory } from "../nodes/node-registry";
import type { NodeCategory } from "../types";

// ── Types ─────────────────────────────────────────────────────────────────────

export interface NodePaletteProps {
  onAddNode: (type: string) => void;
}

// ── Category config ───────────────────────────────────────────────────────────

const CATEGORIES: { id: NodeCategory; label: string }[] = [
  { id: "entry", label: "Entry" },
  { id: "logic", label: "Logic" },
  { id: "agent", label: "Agent" },
  { id: "flow", label: "Flow Control" },
  { id: "human", label: "Human" },
  { id: "action", label: "Action" },
];

// ── Component ─────────────────────────────────────────────────────────────────

export function NodePalette({ onAddNode }: NodePaletteProps) {
  const [query, setQuery] = useState("");
  const [openCategories, setOpenCategories] = useState<
    Record<NodeCategory, boolean>
  >({
    entry: true,
    logic: true,
    agent: true,
    flow: true,
    human: true,
    action: true,
  });

  const lowerQuery = query.toLowerCase().trim();

  // When searching, show all nodes filtered by label — ignoring categories
  const isSearching = lowerQuery.length > 0;
  const filteredAll = isSearching
    ? NODE_REGISTRY.filter((n) =>
        n.label.toLowerCase().includes(lowerQuery)
      )
    : null;

  function toggleCategory(id: NodeCategory) {
    setOpenCategories((prev) => ({ ...prev, [id]: !prev[id] }));
  }

  function handleDragStart(
    e: React.DragEvent<HTMLButtonElement>,
    type: string
  ) {
    e.dataTransfer.setData("application/workflow-node-type", type);
    e.dataTransfer.effectAllowed = "move";
  }

  return (
    <div className="flex flex-col gap-1 min-w-[180px] max-w-[220px]">
      {/* Search */}
      <div className="relative">
        <Search className="absolute left-2 top-1/2 -translate-y-1/2 h-3.5 w-3.5 text-muted-foreground pointer-events-none" />
        <Input
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          placeholder="Search nodes…"
          className="pl-7 h-7 text-xs"
        />
      </div>

      {/* Search results (flat) */}
      {isSearching && filteredAll && (
        <div className="flex flex-col gap-0.5 mt-1">
          {filteredAll.length === 0 ? (
            <p className="text-xs text-muted-foreground px-1 py-2">
              No nodes found.
            </p>
          ) : (
            filteredAll.map((node) => {
              const Icon = node.icon;
              return (
                <Tooltip key={node.type}>
                  <TooltipTrigger asChild>
                    <Button
                      variant="ghost"
                      size="sm"
                      className="h-7 justify-start gap-2 text-xs px-2"
                      onClick={() => onAddNode(node.type)}
                      draggable
                      onDragStart={(e) => handleDragStart(e, node.type)}
                    >
                      <Icon
                        className="h-3.5 w-3.5 shrink-0"
                        style={{ color: node.color }}
                      />
                      {node.label}
                    </Button>
                  </TooltipTrigger>
                  <TooltipContent side="right">
                    {node.description}
                  </TooltipContent>
                </Tooltip>
              );
            })
          )}
        </div>
      )}

      {/* Categorized list */}
      {!isSearching &&
        CATEGORIES.map(({ id, label }) => {
          const nodes = getNodesByCategory(id);
          if (nodes.length === 0) return null;
          const isOpen = openCategories[id];

          return (
            <Collapsible
              key={id}
              open={isOpen}
              onOpenChange={() => toggleCategory(id)}
            >
              <CollapsibleTrigger asChild>
                <button className="flex w-full items-center gap-1 px-1 py-0.5 text-xs font-semibold text-muted-foreground hover:text-foreground transition-colors">
                  {isOpen ? (
                    <ChevronDown className="h-3 w-3 shrink-0" />
                  ) : (
                    <ChevronRight className="h-3 w-3 shrink-0" />
                  )}
                  {label}
                </button>
              </CollapsibleTrigger>
              <CollapsibleContent>
                <div className="flex flex-col gap-0.5 pb-1">
                  {nodes.map((node) => {
                    const Icon = node.icon;
                    return (
                      <Tooltip key={node.type}>
                        <TooltipTrigger asChild>
                          <Button
                            variant="ghost"
                            size="sm"
                            className={cn(
                              "h-7 justify-start gap-2 text-xs px-2 ml-4"
                            )}
                            onClick={() => onAddNode(node.type)}
                            draggable
                            onDragStart={(e) => handleDragStart(e, node.type)}
                          >
                            <Icon
                              className="h-3.5 w-3.5 shrink-0"
                              style={{ color: node.color }}
                            />
                            {node.label}
                          </Button>
                        </TooltipTrigger>
                        <TooltipContent side="right">
                          {node.description}
                        </TooltipContent>
                      </Tooltip>
                    );
                  })}
                </div>
              </CollapsibleContent>
            </Collapsible>
          );
        })}
    </div>
  );
}
