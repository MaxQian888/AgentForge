"use client";

import { useState } from "react";
import { useTranslations } from "next-intl";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import type { DocsPage } from "@/lib/stores/docs-store";

export interface TemplatePickerDestination {
  id: string | null;
  title: string;
}

function TemplatePickerContent({
  templates,
  onOpenChange,
  onPick,
  destinations,
  initialTemplateId,
  defaultTitle,
  defaultParentId,
}: {
  templates: DocsPage[];
  onOpenChange: (open: boolean) => void;
  onPick: (selection: {
    templateId: string;
    title: string;
    parentId?: string | null;
  }) => void | Promise<void>;
  destinations: TemplatePickerDestination[];
  initialTemplateId?: string | null;
  defaultTitle?: string;
  defaultParentId?: string | null;
}) {
  const t = useTranslations("docs");
  const selectableTemplates = templates.filter((template) => template.canUse ?? template.isTemplate);
  const initialSelectedTemplate =
    (initialTemplateId
      ? selectableTemplates.find((template) => template.id === initialTemplateId)
      : undefined) ?? selectableTemplates[0] ?? null;
  const [selectedTemplateId, setSelectedTemplateId] = useState(initialSelectedTemplate?.id ?? "");
  const [title, setTitle] = useState(defaultTitle ?? initialSelectedTemplate?.title ?? "");
  const [parentId, setParentId] = useState<string | null>(defaultParentId ?? null);

  const selectedTemplate = selectableTemplates.find((template) => template.id === selectedTemplateId) ?? null;

  return (
    <DialogContent className="max-w-4xl">
      <DialogHeader>
        <DialogTitle>{t("templatePicker.title")}</DialogTitle>
        <DialogDescription>{t("templatePicker.desc")}</DialogDescription>
      </DialogHeader>
      <div className="grid gap-4 lg:grid-cols-[minmax(0,1.2fr)_minmax(0,0.8fr)]">
        <div className="grid gap-3">
          {selectableTemplates.map((template) => {
            const selected = template.id === selectedTemplateId;
            return (
              <button
                key={template.id}
                type="button"
                className={`rounded-lg border px-4 py-3 text-left transition-colors ${
                  selected
                    ? "border-primary bg-primary/5"
                    : "border-border/60 hover:bg-accent/40"
                }`}
                onClick={() => {
                  setSelectedTemplateId(template.id);
                  if (!title.trim() || title === defaultTitle) {
                    setTitle(template.title);
                  }
                }}
              >
                <div className="font-medium">{template.title}</div>
                <div className="text-xs text-muted-foreground">
                  {template.templateCategory || t("templatePicker.uncategorized")} ·{" "}
                  {template.templateSource === "system"
                    ? t("templatePicker.system")
                    : t("templatePicker.custom")}
                </div>
                {template.previewSnippet ? (
                  <p className="mt-2 text-sm text-muted-foreground">
                    {template.previewSnippet}
                  </p>
                ) : null}
              </button>
            );
          })}
          {selectableTemplates.length === 0 ? (
            <div className="rounded-lg border border-dashed border-border/70 p-4 text-sm text-muted-foreground">
              {t("templatePicker.noTemplates")}
            </div>
          ) : null}
        </div>

        <div className="rounded-lg border border-border/60 bg-card/70 p-4">
          <div className="space-y-1">
            <div className="text-sm font-semibold">{t("templatePicker.previewTitle")}</div>
            {selectedTemplate ? (
              <>
                <div className="font-medium">{selectedTemplate.title}</div>
                <div className="text-xs text-muted-foreground">
                  {selectedTemplate.templateCategory || t("templatePicker.uncategorized")} ·{" "}
                  {selectedTemplate.templateSource === "system"
                    ? t("templatePicker.system")
                    : t("templatePicker.custom")}
                </div>
                <p className="mt-2 min-h-16 text-sm text-muted-foreground">
                  {selectedTemplate.previewSnippet || t("templatePicker.noPreview")}
                </p>
              </>
            ) : (
              <p className="text-sm text-muted-foreground">{t("templatePicker.noSelection")}</p>
            )}
          </div>

          <div className="mt-4 space-y-3">
            <div className="space-y-2">
              <Label htmlFor="template-picker-title">{t("templatePicker.titleField")}</Label>
              <Input
                id="template-picker-title"
                value={title}
                onChange={(event) => setTitle(event.target.value)}
                placeholder={t("templatePicker.titlePlaceholder")}
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="template-picker-parent">{t("templatePicker.locationField")}</Label>
              <select
                id="template-picker-parent"
                className="flex h-9 w-full rounded-md border border-input bg-background px-3 py-1 text-sm"
                value={parentId ?? ""}
                onChange={(event) => setParentId(event.target.value || null)}
              >
                <option value="">{t("templatePicker.rootLocation")}</option>
                {destinations.map((destination) => (
                  <option key={destination.id ?? "root"} value={destination.id ?? ""}>
                    {destination.title}
                  </option>
                ))}
              </select>
            </div>
          </div>

          <div className="mt-6 flex justify-end gap-2">
            <Button variant="ghost" onClick={() => onOpenChange(false)}>
              {t("templatePicker.close")}
            </Button>
            <Button
              onClick={() => {
                if (!selectedTemplate || !title.trim()) {
                  return;
                }
                void Promise.resolve(
                  onPick({
                    templateId: selectedTemplate.id,
                    title: title.trim(),
                    parentId,
                  }),
                );
              }}
              disabled={!selectedTemplate || !title.trim()}
            >
              {t("templatePicker.createDocument")}
            </Button>
          </div>
        </div>
      </div>
    </DialogContent>
  );
}

export function TemplatePicker({
  open,
  onOpenChange,
  templates,
  onPick,
  destinations = [],
  initialTemplateId,
  defaultTitle,
  defaultParentId = null,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  templates: DocsPage[];
  onPick: (selection: {
    templateId: string;
    title: string;
    parentId?: string | null;
  }) => void | Promise<void>;
  destinations?: TemplatePickerDestination[];
  initialTemplateId?: string | null;
  defaultTitle?: string;
  defaultParentId?: string | null;
}) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      {open ? (
        <TemplatePickerContent
          templates={templates}
          onOpenChange={onOpenChange}
          onPick={onPick}
          destinations={destinations}
          initialTemplateId={initialTemplateId}
          defaultTitle={defaultTitle}
          defaultParentId={defaultParentId}
        />
      ) : null}
    </Dialog>
  );
}
