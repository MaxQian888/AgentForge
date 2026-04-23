"use client";

import { useRef, useState } from "react";
import {
  Save,
  PlayCircle,
  Undo2,
  Redo2,
  GripVertical,
  Loader2,
  Download,
  Upload,
} from "lucide-react";
import { useTranslations } from "next-intl";
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
  onExport?: () => void;
  onImport?: (file: File) => void;
}

// ── Component ─────────────────────────────────────────────────────────────────

export function EditorToolbar({
  status,
  saving,
  onExecute,
  onSave,
  onAddNode,
  onExport,
  onImport,
}: EditorToolbarProps) {
  const { state, dispatch } = useEditor();
  const { canUndo, canRedo, undo, redo } = useUndoRedo();
  const [showPalette, setShowPalette] = useState(true);
  const fileInputRef = useRef<HTMLInputElement>(null);
  const t = useTranslations("workflow");

  function triggerImport() {
    fileInputRef.current?.click();
  }

  function handleFileChange(e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0];
    if (file && onImport) {
      onImport(file);
    }
    // Reset so picking the same file again re-triggers onChange
    if (fileInputRef.current) fileInputRef.current.value = "";
  }

  return (
    <div className="flex flex-col gap-3 p-3 border-b bg-background">
      {/* Row 1: name / description / status / actions */}
      <div className="flex items-center gap-3">
        <Input
          value={state.name}
          onChange={(e) =>
            dispatch({ type: "UPDATE_NAME", name: e.target.value })
          }
          placeholder={t("toolbar.workflowNamePlaceholder")}
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
          placeholder={t("toolbar.descriptionPlaceholder")}
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
                aria-label={t("editor.undo")}
              >
                <Undo2 className="h-4 w-4" />
              </Button>
            </TooltipTrigger>
            <TooltipContent>{t("toolbar.undoTooltip")}</TooltipContent>
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
                aria-label={t("editor.redo")}
              >
                <Redo2 className="h-4 w-4" />
              </Button>
            </TooltipTrigger>
            <TooltipContent>{t("toolbar.redoTooltip")}</TooltipContent>
          </Tooltip>

          {/* Import (hidden file input + trigger button) */}
          {onImport && (
            <>
              <input
                ref={fileInputRef}
                type="file"
                accept="application/json,.json"
                className="hidden"
                onChange={handleFileChange}
                aria-label={t("toolbar.importAria")}
                data-testid="import-workflow-file-input"
              />
              <Tooltip>
                <TooltipTrigger asChild>
                  <Button
                    variant="ghost"
                    size="icon"
                    className="h-8 w-8"
                    onClick={triggerImport}
                    aria-label={t("toolbar.importWorkflowAria")}
                  >
                    <Upload className="h-4 w-4" />
                  </Button>
                </TooltipTrigger>
                <TooltipContent>{t("toolbar.importTooltip")}</TooltipContent>
              </Tooltip>
            </>
          )}

          {/* Export */}
          {onExport && (
            <Tooltip>
              <TooltipTrigger asChild>
                <Button
                  variant="ghost"
                  size="icon"
                  className="h-8 w-8"
                  onClick={onExport}
                  aria-label={t("toolbar.exportAria")}
                >
                  <Download className="h-4 w-4" />
                </Button>
              </TooltipTrigger>
              <TooltipContent>{t("toolbar.exportTooltip")}</TooltipContent>
            </Tooltip>
          )}

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
            {saving ? t("toolbar.saving") : t("editor.save")}
          </Button>

          {/* Execute */}
          <Button
            size="sm"
            onClick={onExecute}
            disabled={status !== "active"}
          >
            <PlayCircle className="h-4 w-4 mr-1" />
            {t("editor.execute")}
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
          {showPalette ? t("editor.hidePalette") : t("editor.showPalette")}
        </Button>

        {showPalette && (
          <NodePalette onAddNode={onAddNode} />
        )}
      </div>
    </div>
  );
}
