"use client";

import { Copy, History, RotateCcw } from "lucide-react";
import { Button } from "@/components/ui/button";
import type { DocsVersion } from "@/lib/stores/docs-store";

export function VersionHistoryPanel({
  versions,
  selectedVersionId,
  onSelect,
  onRestore,
  onShare,
  readonly = false,
}: {
  versions: DocsVersion[];
  selectedVersionId?: string | null;
  onSelect?: (versionId: string) => void;
  onRestore?: (versionId: string) => void;
  onShare?: (versionId: string) => void;
  readonly?: boolean;
}) {
  return (
    <div className="flex flex-col gap-3 rounded-xl border border-border/60 bg-card/70 p-4">
      <div className="flex items-center gap-2">
        <History className="size-4 text-muted-foreground" />
        <h2 className="text-base font-semibold">Version History</h2>
      </div>
      {versions.map((version) => (
        <div
          key={version.id}
          className={`rounded-lg border p-3 ${
            selectedVersionId === version.id ? "border-primary bg-primary/5" : "border-border/60"
          }`}
        >
          <button
            type="button"
            className="w-full text-left"
            onClick={() => onSelect?.(version.id)}
          >
            <div className="font-medium">
              v{version.versionNumber} · {version.name}
            </div>
            <div className="text-xs text-muted-foreground">
              {new Date(version.createdAt).toLocaleString()}
            </div>
          </button>
          <div className="mt-3 flex gap-2">
            <Button
              size="sm"
              variant="outline"
              disabled={readonly}
              onClick={() => {
                if (typeof window !== "undefined") {
                  const confirmed = window.confirm(
                    `Restore ${version.name} (v${version.versionNumber})?`
                  );
                  if (!confirmed) {
                    return;
                  }
                }
                onRestore?.(version.id);
              }}
            >
              <RotateCcw className="mr-1 size-3.5" />
              Restore
            </Button>
            <Button size="sm" variant="ghost" onClick={() => onShare?.(version.id)}>
              <Copy className="mr-1 size-3.5" />
              Share
            </Button>
          </div>
        </div>
      ))}
      {versions.length === 0 ? (
        <p className="text-sm text-muted-foreground">No saved versions yet.</p>
      ) : null}
    </div>
  );
}
