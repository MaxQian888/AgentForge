"use client";

import { useMemo, useState } from "react";
import { useTranslations } from "next-intl";
import { LayoutTemplate, Plus, Search } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import type { DocsPage } from "@/lib/stores/docs-store";

type TemplateDraftMode = "create" | "duplicate";

export function TemplateCenter({
  templates,
  onCreateFromTemplate,
  onCreateTemplate,
  onEditTemplate,
  onDuplicateTemplate,
  onDeleteTemplate,
}: {
  templates: DocsPage[];
  onCreateFromTemplate?: (templateId: string) => void;
  onCreateTemplate?: (input: { title: string; category: string }) => void | Promise<void>;
  onEditTemplate?: (templateId: string) => void;
  onDuplicateTemplate?: (input: {
    templateId: string;
    name: string;
    category: string;
  }) => void | Promise<void>;
  onDeleteTemplate?: (templateId: string) => void | Promise<void>;
}) {
  const t = useTranslations("docs");
  const [query, setQuery] = useState("");
  const [source, setSource] = useState<"all" | "system" | "custom">("all");
  const [selectedTemplateId, setSelectedTemplateId] = useState<string>("");
  const [draftMode, setDraftMode] = useState<TemplateDraftMode | null>(null);
  const [draftTitle, setDraftTitle] = useState("");
  const [draftCategory, setDraftCategory] = useState("custom");

  const categories = useMemo(() => {
    return Array.from(
      new Set(
        templates
          .map((template) => template.templateCategory)
          .filter((value): value is string => Boolean(value)),
      ),
    ).sort();
  }, [templates]);

  const filteredTemplates = useMemo(() => {
    const normalizedQuery = query.trim().toLowerCase();
    return templates.filter((template) => {
      const templateSource = template.templateSource ?? (template.isSystem ? "system" : "custom");
      if (source !== "all" && templateSource !== source) {
        return false;
      }
      if (!normalizedQuery) {
        return true;
      }
      const haystack = [
        template.title,
        template.templateCategory,
        template.previewSnippet,
        template.contentText,
      ]
        .filter(Boolean)
        .join(" ")
        .toLowerCase();
      return haystack.includes(normalizedQuery);
    });
  }, [query, source, templates]);

  const selectedTemplate =
    filteredTemplates.find((template) => template.id === selectedTemplateId) ??
    filteredTemplates[0] ??
    null;

  return (
    <div className="flex flex-col gap-4 rounded-xl border border-border/60 bg-card/70 p-4">
      <div className="flex flex-col gap-3 lg:flex-row lg:items-start lg:justify-between">
        <div>
          <h2 className="flex items-center gap-2 text-base font-semibold">
            <LayoutTemplate className="size-4 text-muted-foreground" />
            {t("templateCenter.title")}
          </h2>
          <p className="text-sm text-muted-foreground">{t("templateCenter.desc")}</p>
        </div>
        {onCreateTemplate ? (
          <Button
            size="sm"
            onClick={() => {
              setDraftMode("create");
              setDraftTitle("");
              setDraftCategory("custom");
            }}
          >
            <Plus className="mr-1 size-4" />
            {t("templateCenter.newTemplate")}
          </Button>
        ) : null}
      </div>

      <div className="flex flex-col gap-3 lg:flex-row lg:items-center">
        <div className="relative flex-1">
          <Search className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
          <Input
            className="pl-9"
            value={query}
            onChange={(event) => setQuery(event.target.value)}
            placeholder={t("templateCenter.searchPlaceholder")}
          />
        </div>
        <div className="flex gap-2">
          {(["all", "system", "custom"] as const).map((value) => (
            <Button
              key={value}
              size="sm"
              variant={source === value ? "default" : "outline"}
              onClick={() => setSource(value)}
            >
              {t(`templateCenter.filters.${value}`)}
            </Button>
          ))}
        </div>
      </div>

      {filteredTemplates.length === 0 ? (
        <p className="text-sm text-muted-foreground">{t("templateCenter.noTemplates")}</p>
      ) : (
        <div className="grid gap-4 lg:grid-cols-[minmax(0,1.2fr)_minmax(0,0.8fr)]">
          <div className="grid gap-3 md:grid-cols-2">
            {filteredTemplates.map((template) => {
              const templateSource =
                template.templateSource ?? (template.isSystem ? "system" : "custom");
              const selected = template.id === selectedTemplateId;
              return (
                <button
                  key={template.id}
                  type="button"
                  className={`rounded-lg border p-4 text-left transition-colors ${
                    selected
                      ? "border-primary bg-primary/5"
                      : "border-border/60 hover:bg-accent/40"
                  }`}
                  onClick={() => setSelectedTemplateId(template.id)}
                >
                  <div className="font-medium">{template.title}</div>
                  <div className="text-xs text-muted-foreground">
                    {template.templateCategory || t("templateCenter.uncategorized")} ·{" "}
                    {t(`templateCenter.filters.${templateSource}`)}
                  </div>
                  {template.previewSnippet ? (
                    <p className="mt-2 line-clamp-3 text-sm text-muted-foreground">
                      {template.previewSnippet}
                    </p>
                  ) : null}
                </button>
              );
            })}
          </div>

          <div className="rounded-lg border border-border/60 bg-background/70 p-4">
            {selectedTemplate ? (
              <>
                <div className="space-y-1">
                  <h3 className="text-sm font-semibold">{t("templateCenter.previewTitle")}</h3>
                  <div className="font-medium">{selectedTemplate.title}</div>
                  <div className="text-xs text-muted-foreground">
                    {selectedTemplate.templateCategory || t("templateCenter.uncategorized")} ·{" "}
                    {t(
                      `templateCenter.filters.${
                        selectedTemplate.templateSource ?? (selectedTemplate.isSystem ? "system" : "custom")
                      }`,
                    )}
                  </div>
                  <p className="mt-2 min-h-20 text-sm text-muted-foreground">
                    {selectedTemplate.previewSnippet || t("templateCenter.noPreview")}
                  </p>
                </div>

                <div className="mt-4 flex flex-wrap gap-2">
                  {onCreateFromTemplate ? (
                    <Button size="sm" onClick={() => onCreateFromTemplate(selectedTemplate.id)}>
                      {t("templateCenter.useTemplate")}
                    </Button>
                  ) : null}
                  {selectedTemplate.canEdit && onEditTemplate ? (
                    <Button
                      size="sm"
                      variant="outline"
                      onClick={() => onEditTemplate(selectedTemplate.id)}
                    >
                      {t("templateCenter.edit")}
                    </Button>
                  ) : null}
                  {selectedTemplate.canDuplicate && onDuplicateTemplate ? (
                    <Button
                      size="sm"
                      variant="outline"
                      onClick={() => {
                        setDraftMode("duplicate");
                        setDraftTitle(`${selectedTemplate.title} Copy`);
                        setDraftCategory(selectedTemplate.templateCategory || "custom");
                      }}
                    >
                      {t("templateCenter.duplicate")}
                    </Button>
                  ) : null}
                  {selectedTemplate.canDelete && onDeleteTemplate ? (
                    <Button
                      size="sm"
                      variant="outline"
                      onClick={() => {
                        if (window.confirm(t("templateCenter.confirmDelete", { title: selectedTemplate.title }))) {
                          void onDeleteTemplate(selectedTemplate.id);
                        }
                      }}
                    >
                      {t("templateCenter.delete")}
                    </Button>
                  ) : null}
                </div>
              </>
            ) : (
              <p className="text-sm text-muted-foreground">{t("templateCenter.noSelection")}</p>
            )}
          </div>
        </div>
      )}

      <Dialog open={draftMode !== null} onOpenChange={(open) => (!open ? setDraftMode(null) : undefined)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>
              {draftMode === "create"
                ? t("templateCenter.createDialog.title")
                : t("templateCenter.duplicateDialog.title")}
            </DialogTitle>
            <DialogDescription>
              {draftMode === "create"
                ? t("templateCenter.createDialog.desc")
                : t("templateCenter.duplicateDialog.desc")}
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-4 py-2">
            <div className="space-y-2">
              <Label htmlFor="template-center-title">{t("templateCenter.dialog.titleLabel")}</Label>
              <Input
                id="template-center-title"
                value={draftTitle}
                onChange={(event) => setDraftTitle(event.target.value)}
                placeholder={t("templateCenter.dialog.titlePlaceholder")}
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="template-center-category">{t("templateCenter.dialog.categoryLabel")}</Label>
              <select
                id="template-center-category"
                className="flex h-9 w-full rounded-md border border-input bg-background px-3 py-1 text-sm"
                value={draftCategory}
                onChange={(event) => setDraftCategory(event.target.value)}
              >
                <option value="custom">{t("templateCenter.dialog.defaultCategory")}</option>
                {categories.map((category) => (
                  <option key={category} value={category}>
                    {category}
                  </option>
                ))}
              </select>
            </div>
          </div>
          <DialogFooter>
            <Button variant="ghost" onClick={() => setDraftMode(null)}>
              {t("templateCenter.dialog.cancel")}
            </Button>
            <Button
              onClick={() => {
                if (!draftTitle.trim()) {
                  return;
                }
                if (draftMode === "create") {
                  void onCreateTemplate?.({
                    title: draftTitle.trim(),
                    category: draftCategory,
                  });
                }
                if (draftMode === "duplicate" && selectedTemplate) {
                  void onDuplicateTemplate?.({
                    templateId: selectedTemplate.id,
                    name: draftTitle.trim(),
                    category: draftCategory,
                  });
                }
                setDraftMode(null);
              }}
              disabled={!draftTitle.trim()}
            >
              {draftMode === "create"
                ? t("templateCenter.createDialog.confirm")
                : t("templateCenter.duplicateDialog.confirm")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
