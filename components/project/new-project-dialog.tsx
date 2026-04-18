"use client";

import { useEffect, useMemo, useState } from "react";
import { useTranslations } from "next-intl";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { useProjectStore } from "@/lib/stores/project-store";
import {
  useProjectTemplateStore,
  type ProjectTemplate,
  type ProjectTemplateSource,
} from "@/lib/stores/project-template-store";

type NewProjectStartMode = "blank" | "template";

interface NewProjectDialogProps {
  open: boolean;
  onClose: () => void;
  onCreated?: (projectId: string) => void;
}

/**
 * Two-step project creation dialog.
 *
 * Step 1: pick a starting mode — "blank project" or "from template". When
 * the user picks a template, its source + id get threaded onto the create
 * request; the backend materializes the snapshot into the new project in
 * the same creation call. Blank creation path is identical to the legacy
 * behavior (no template params sent).
 */
export function NewProjectDialog({ open, onClose, onCreated }: NewProjectDialogProps) {
  const t = useTranslations("projectTemplates");
  const tProjects = useTranslations("projects");
  const createProject = useProjectStore((s) => s.createProject);
  const templates = useProjectTemplateStore((s) => s.templates);
  const loadingTemplates = useProjectTemplateStore((s) => s.loading);
  const fetchTemplates = useProjectTemplateStore((s) => s.fetchTemplates);

  const [mode, setMode] = useState<NewProjectStartMode>("blank");
  const [selectedTemplate, setSelectedTemplate] = useState<ProjectTemplate | null>(null);
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [creating, setCreating] = useState(false);

  useEffect(() => {
    if (open) {
      fetchTemplates();
    }
  }, [open, fetchTemplates]);

  useEffect(() => {
    if (!open) {
      setMode("blank");
      setSelectedTemplate(null);
      setName("");
      setDescription("");
    }
  }, [open]);

  const groupedTemplates = useMemo(() => {
    const groups: Record<ProjectTemplateSource, ProjectTemplate[]> = {
      system: [],
      user: [],
      marketplace: [],
    };
    for (const tpl of templates) {
      groups[tpl.source]?.push(tpl);
    }
    return groups;
  }, [templates]);

  const handleCreate = async () => {
    const trimmed = name.trim();
    if (!trimmed) return;
    setCreating(true);
    try {
      const result = await createProject({
        name: trimmed,
        description,
        ...(mode === "template" && selectedTemplate
          ? {
              templateSource: selectedTemplate.source,
              templateId: selectedTemplate.id,
            }
          : {}),
      });
      if (result) {
        onCreated?.(result.id);
        onClose();
      }
    } finally {
      setCreating(false);
    }
  };

  const canSubmit = Boolean(name.trim()) && (mode === "blank" || !!selectedTemplate);

  return (
    <Dialog open={open} onOpenChange={(v) => !v && onClose()}>
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>{t("newProject.title")}</DialogTitle>
          <DialogDescription>{t("newProject.description")}</DialogDescription>
        </DialogHeader>
        <div className="flex flex-col gap-6 py-4">
          <div className="flex flex-col gap-2">
            <Label>{t("newProject.startFrom")}</Label>
            <div className="flex gap-2">
              <Button
                type="button"
                variant={mode === "blank" ? "default" : "outline"}
                onClick={() => setMode("blank")}
              >
                {t("newProject.blank")}
              </Button>
              <Button
                type="button"
                variant={mode === "template" ? "default" : "outline"}
                onClick={() => setMode("template")}
              >
                {t("newProject.fromTemplate")}
              </Button>
            </div>
          </div>

          {mode === "template" && (
            <div className="flex flex-col gap-2">
              <Label>{t("newProject.selectTemplate")}</Label>
              {loadingTemplates ? (
                <p className="text-sm text-muted-foreground">{t("newProject.loading")}</p>
              ) : (
                <div className="flex max-h-64 flex-col gap-4 overflow-y-auto rounded-md border p-2">
                  {(["system", "user", "marketplace"] as ProjectTemplateSource[]).map(
                    (source) =>
                      groupedTemplates[source].length > 0 && (
                        <div key={source} className="flex flex-col gap-2">
                          <p className="text-xs font-semibold uppercase text-muted-foreground">
                            {t(`source.${source}`)}
                          </p>
                          {groupedTemplates[source].map((tpl) => {
                            const active = selectedTemplate?.id === tpl.id;
                            return (
                              <button
                                key={tpl.id}
                                type="button"
                                onClick={() => setSelectedTemplate(tpl)}
                                className={`rounded-md border p-2 text-left text-sm transition ${
                                  active ? "border-primary bg-primary/5" : "hover:bg-accent"
                                }`}
                              >
                                <p className="font-medium">{tpl.name}</p>
                                {tpl.description && (
                                  <p className="mt-1 text-xs text-muted-foreground">
                                    {tpl.description}
                                  </p>
                                )}
                              </button>
                            );
                          })}
                        </div>
                      ),
                  )}
                  {templates.length === 0 && (
                    <p className="text-sm text-muted-foreground">
                      {t("newProject.noTemplates")}
                    </p>
                  )}
                </div>
              )}
            </div>
          )}

          <div className="flex flex-col gap-2">
            <Label htmlFor="new-project-name">{tProjects("createProject.nameLabel")}</Label>
            <Input
              id="new-project-name"
              value={name}
              onChange={(e) => setName(e.target.value)}
              required
              maxLength={100}
            />
          </div>
          <div className="flex flex-col gap-2">
            <Label htmlFor="new-project-description">
              {tProjects("createProject.descriptionLabel")}
            </Label>
            <Textarea
              id="new-project-description"
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              rows={3}
            />
          </div>
        </div>
        <DialogFooter>
          <Button variant="ghost" onClick={onClose} disabled={creating}>
            {t("newProject.cancel")}
          </Button>
          <Button onClick={handleCreate} disabled={!canSubmit || creating}>
            {creating ? t("newProject.creating") : t("newProject.create")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
