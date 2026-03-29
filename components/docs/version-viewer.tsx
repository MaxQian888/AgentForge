"use client";

import { useTranslations } from "next-intl";
import type { DocsVersion } from "@/lib/stores/docs-store";

export function VersionViewer({ version }: { version: DocsVersion | null }) {
  const t = useTranslations("docs");

  return (
    <div className="min-h-[220px] rounded-xl border border-border/60 bg-card/70 p-4">
      {version ? (
        <div className="space-y-3">
          <div>
            <h3 className="font-semibold">{version.name}</h3>
            <p className="text-xs text-muted-foreground">
              v{version.versionNumber} · {new Date(version.createdAt).toLocaleString()}
            </p>
          </div>
          <pre className="overflow-x-auto rounded-lg bg-muted/40 p-3 text-xs">
            {version.content}
          </pre>
        </div>
      ) : (
        <p className="text-sm text-muted-foreground">{t("versionViewer.selectVersion")}</p>
      )}
    </div>
  );
}
