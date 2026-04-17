"use client";

import { useTranslations } from "next-intl";
import { Copy, History, RotateCcw } from "lucide-react";
import { Button } from "@/components/ui/button";
import type { AssetVersion } from "@/lib/stores/knowledge-store";

export function VersionHistoryPanel({
  versions,
  selectedVersionId,
  onSelect,
  onRestore,
  onShare,
  readonly = false,
}: {
  versions: AssetVersion[];
  selectedVersionId?: string | null;
  onSelect?: (versionId: string) => void;
  onRestore?: (versionId: string) => void;
  onShare?: (versionId: string) => void;
  readonly?: boolean;
}) {
  const t = useTranslations("docs");

  return (
    <div className="flex flex-col gap-3 rounded-xl border border-border/60 bg-card/70 p-4">
      <div className="flex items-center gap-2">
        <History className="size-4 text-muted-foreground" />
        <h2 className="text-base font-semibold">{t("versionHistory.title")}</h2>
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
                    t("versionHistory.confirmRestore", { name: version.name, version: version.versionNumber })
                  );
                  if (!confirmed) {
                    return;
                  }
                }
                onRestore?.(version.id);
              }}
            >
              <RotateCcw className="mr-1 size-3.5" />
              {t("versionHistory.restore")}
            </Button>
            <Button size="sm" variant="ghost" onClick={() => onShare?.(version.id)}>
              <Copy className="mr-1 size-3.5" />
              {t("versionHistory.share")}
            </Button>
          </div>
        </div>
      ))}
      {versions.length === 0 ? (
        <p className="text-sm text-muted-foreground">{t("versionHistory.noVersions")}</p>
      ) : null}
    </div>
  );
}
