"use client";

import { useTranslations } from "next-intl";
import { Save, Copy, LayoutTemplate } from "lucide-react";
import { Button } from "@/components/ui/button";

export function EditorToolbar({
  onSaveVersion,
  onSaveTemplate,
  onShareVersion,
  readonly = false,
  saving,
  templateActionLabel,
  templateActionDisabled,
}: {
  onSaveVersion?: () => void;
  onSaveTemplate?: () => void;
  onShareVersion?: () => void;
  readonly?: boolean;
  saving?: boolean;
  templateActionLabel?: string;
  templateActionDisabled?: boolean;
}) {
  const t = useTranslations("docs");

  return (
    <div className="flex flex-wrap items-center gap-2 rounded-xl border border-border/60 bg-card/80 px-3 py-2">
      <Button size="sm" variant="outline" onClick={onSaveVersion} disabled={saving || readonly}>
        <Save className="mr-1 size-4" />
        {t("editor.saveVersion")}
      </Button>
      <Button
        size="sm"
        variant="outline"
        onClick={onSaveTemplate}
        disabled={templateActionDisabled ?? readonly}
      >
        <LayoutTemplate className="mr-1 size-4" />
        {templateActionLabel ?? t("editor.saveAsTemplate")}
      </Button>
      <Button size="sm" variant="outline" onClick={onShareVersion}>
        <Copy className="mr-1 size-4" />
        {t("editor.shareVersion")}
      </Button>
    </div>
  );
}
