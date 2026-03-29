"use client";

import { useTranslations } from "next-intl";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import type { DocsPage } from "@/lib/stores/docs-store";

export function TemplatePicker({
  open,
  onOpenChange,
  templates,
  onPick,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  templates: DocsPage[];
  onPick: (templateId: string) => void;
}) {
  const t = useTranslations("docs");

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t("templatePicker.title")}</DialogTitle>
          <DialogDescription>
            {t("templatePicker.desc")}
          </DialogDescription>
        </DialogHeader>
        <div className="grid gap-3">
          {templates.map((template) => (
            <button
              key={template.id}
              type="button"
              className="rounded-lg border border-border/60 px-4 py-3 text-left hover:bg-accent/40"
              onClick={() => onPick(template.id)}
            >
              <div className="font-medium">{template.title}</div>
              <div className="text-xs text-muted-foreground">
                {template.templateCategory || t("templatePicker.uncategorized")} ·{" "}
                {template.isSystem ? t("templatePicker.system") : t("templatePicker.custom")}
              </div>
            </button>
          ))}
          {templates.length === 0 ? (
            <div className="rounded-lg border border-dashed border-border/70 p-4 text-sm text-muted-foreground">
              {t("templatePicker.noTemplates")}
            </div>
          ) : null}
        </div>
        <div className="flex justify-end">
          <Button variant="ghost" onClick={() => onOpenChange(false)}>
            {t("templatePicker.close")}
          </Button>
        </div>
      </DialogContent>
    </Dialog>
  );
}
