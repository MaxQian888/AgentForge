"use client";

import { useState } from "react";
import { AlertTriangle, X } from "lucide-react";
import { Button } from "@/components/ui/button";
import { useTranslations } from "next-intl";

/**
 * Dismissible banner shown on a wiki page that was materialized from a file
 * when the source file has been updated since materialization.
 */
export function SourceUpdatedBanner() {
  const t = useTranslations("knowledge");
  const [dismissed, setDismissed] = useState(false);

  if (dismissed) return null;

  return (
    <div
      role="alert"
      className="flex items-center justify-between gap-3 rounded-xl border border-amber-400/60 bg-amber-50/60 px-4 py-3 dark:border-amber-400/30 dark:bg-amber-900/20"
    >
      <div className="flex items-center gap-2 text-sm text-amber-800 dark:text-amber-300">
        <AlertTriangle className="size-4 shrink-0" />
        <span>{t("sourceUpdatedBanner.text")}</span>
      </div>
      <Button
        size="icon-sm"
        variant="ghost"
        className="text-amber-700 hover:text-amber-900 dark:text-amber-300 dark:hover:text-amber-100"
        onClick={() => setDismissed(true)}
        aria-label={t("sourceUpdatedBanner.dismiss")}
      >
        <X className="size-4" />
      </Button>
    </div>
  );
}
