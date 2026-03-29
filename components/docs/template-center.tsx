"use client";

import { useTranslations } from "next-intl";
import { LayoutTemplate } from "lucide-react";
import { Button } from "@/components/ui/button";
import type { DocsPage } from "@/lib/stores/docs-store";

export function TemplateCenter({
  templates,
  onCreateFromTemplate,
}: {
  templates: DocsPage[];
  onCreateFromTemplate?: (templateId: string) => void;
}) {
  const t = useTranslations("docs");

  return (
    <div className="flex flex-col gap-4 rounded-xl border border-border/60 bg-card/70 p-4">
      <div>
        <h2 className="flex items-center gap-2 text-base font-semibold">
          <LayoutTemplate className="size-4 text-muted-foreground" />
          {t("templateCenter.title")}
        </h2>
        <p className="text-sm text-muted-foreground">
          {t("templateCenter.desc")}
        </p>
      </div>
      <div className="grid gap-3 md:grid-cols-2">
        {templates.map((template) => (
          <div key={template.id} className="rounded-lg border border-border/60 p-4">
            <div className="font-medium">{template.title}</div>
            <div className="text-xs text-muted-foreground">
              {template.templateCategory || t("templatePicker.uncategorized")} ·{" "}
              {template.isSystem ? t("templatePicker.system") : t("templatePicker.custom")}
            </div>
            <Button
              size="sm"
              variant="outline"
              className="mt-3"
              onClick={() => onCreateFromTemplate?.(template.id)}
            >
              {t("templateCenter.useTemplate")}
            </Button>
          </div>
        ))}
      </div>
      {templates.length === 0 ? (
        <p className="text-sm text-muted-foreground">{t("templateCenter.noTemplates")}</p>
      ) : null}
    </div>
  );
}
