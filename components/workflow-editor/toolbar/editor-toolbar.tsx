"use client";

import { useState } from "react";
import { Save, PlayCircle, Undo2, Redo2, GripVertical, Loader2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Badge } from "@/components/ui/badge";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import { useEditor } from "../context";
import { useUndoRedo } from "../hooks/use-undo-redo";
import { NodePalette } from "./node-palette";

// ── Types ─────────────────────────────────────────────────────────────────────

export interface EditorToolbarProps {
  status: string;
  saving: boolean;
  onExecute: () => void;
  onSave: () => void;
  onAddNode: (type: string) => void;
}

// ── Component ─────────────────────────────────────────────────────────────────

export function EditorToolbar({
  status,
  saving,
  onExecute,
  onSave,
  onAddNode,
}: EditorToolbarProps) {
  const { state, dispatch } = useEditor();
  const { canUndo, canRedo, undo, redo } = useUndoRedo();
  const [showPalette, setShowPalette] = useState(true);

  return (
    <div className="flex flex-col gap-3 p-3 border-b bg-background">
      {/* Row 1: name / description / status / actions */}
      <div className="flex items-center gap-3">
        <Input
          value={state.name}
          onChange={(e) =>
            dispatch({ type: "UPDATE_NAME", name: e.target.value })
          }
          placeholder="Workflow name"
          className="max-w-xs font-medium"
        />
        <Input
          value={state.description}
          onChange={(e) =>
            dispatch({
              type: "UPDATE_DESCRIPTION",
              description: e.target.value,
            })
          }
          placeholder="Description"
          className="max-w-sm text-sm"
        />
        <Badge variant={status === "active" ? "default" : "secondary"}>
          {status}
        </Badge>

        <div className="ml-auto flex items-center gap-1">
          {/* Undo */}
          <Tooltip>
            <TooltipTrigger asChild>
              <Button
                variant="ghost"
                size="icon"
                className="h-8 w-8"
                onClick={undo}
                disabled={!canUndo}
                aria-label="Undo"
              >
                <Undo2 className="h-4 w-4" />
              </Button>
            </TooltipTrigger>
            <TooltipContent>Undo (Ctrl+Z)</TooltipContent>
          </Tooltip>

          {/* Redo */}
          <Tooltip>
            <TooltipTrigger asChild>
              <Button
                variant="ghost"
                size="icon"
                className="h-8 w-8"
                onClick={redo}
                disabled={!canRedo}
                aria-label="Redo"
              >
                <Redo2 className="h-4 w-4" />
              </Button>
            </TooltipTrigger>
            <TooltipContent>Redo (Ctrl+Shift+Z)</TooltipContent>
          </Tooltip>

          {/* Save */}
          <Button
            variant="outline"
            size="sm"
            onClick={onSave}
            disabled={saving}
          >
            {saving ? (
              <Loader2 className="h-4 w-4 mr-1 animate-spin" />
            ) : (
              <Save className="h-4 w-4 mr-1" />
            )}
            {saving ? "Saving…" : "Save"}
          </Button>

          {/* Execute */}
          <Button
            size="sm"
            onClick={onExecute}
            disabled={status !== "active"}
          >
            <PlayCircle className="h-4 w-4 mr-1" />
            Execute
          </Button>
        </div>
      </div>

      {/* Row 2: palette toggle + palette */}
      <div className="flex items-start gap-3">
        <Button
          variant="ghost"
          size="sm"
          onClick={() => setShowPalette((v) => !v)}
          className="text-xs text-muted-foreground shrink-0"
        >
          <GripVertical className="h-3 w-3 mr-1" />
          {showPalette ? "Hide palette" : "Show palette"}
        </Button>

        {showPalette && (
          <NodePalette onAddNode={onAddNode} />
        )}
      </div>
    </div>
  );
}
