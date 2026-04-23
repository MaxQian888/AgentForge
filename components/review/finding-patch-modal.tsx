"use client";

import { useTranslations } from "next-intl";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import ReactDiffViewer, { DiffMethod } from "react-diff-viewer-continued";

interface FindingPatchModalProps {
  patch: string | null | undefined;
  open: boolean;
  onClose: () => void;
}

/**
 * Parses a unified diff into old/new text for display.
 * For a simple heuristic, extracts removed/added lines.
 */
function parseUnifiedDiff(patch: string): { oldValue: string; newValue: string } {
  const lines = patch.split("\n");
  const oldLines: string[] = [];
  const newLines: string[] = [];

  for (const line of lines) {
    if (line.startsWith("---") || line.startsWith("+++") || line.startsWith("@@")) {
      continue;
    }
    if (line.startsWith("-")) {
      oldLines.push(line.slice(1));
    } else if (line.startsWith("+")) {
      newLines.push(line.slice(1));
    } else {
      // Context line (starts with space or no prefix)
      const text = line.startsWith(" ") ? line.slice(1) : line;
      oldLines.push(text);
      newLines.push(text);
    }
  }

  return {
    oldValue: oldLines.join("\n"),
    newValue: newLines.join("\n"),
  };
}

export function FindingPatchModal({ patch, open, onClose }: FindingPatchModalProps) {
  const t = useTranslations("reviews");

  if (!patch) {
    return (
      <Dialog open={open} onOpenChange={(v) => !v && onClose()}>
        <DialogContent className="max-w-2xl">
          <DialogHeader>
            <DialogTitle>{t("patchTitle")}</DialogTitle>
          </DialogHeader>
          <p className="py-4 text-center text-sm text-muted-foreground">
            {t("patchUnavailable")}
          </p>
        </DialogContent>
      </Dialog>
    );
  }

  const { oldValue, newValue } = parseUnifiedDiff(patch);

  return (
    <Dialog open={open} onOpenChange={(v) => !v && onClose()}>
      <DialogContent className="max-w-4xl max-h-[80vh] overflow-auto">
        <DialogHeader>
          <DialogTitle>{t("patchTitle")}</DialogTitle>
        </DialogHeader>
        <div className="text-xs">
          <ReactDiffViewer
            oldValue={oldValue}
            newValue={newValue}
            splitView
            compareMethod={DiffMethod.LINES}
          />
        </div>
      </DialogContent>
    </Dialog>
  );
}
