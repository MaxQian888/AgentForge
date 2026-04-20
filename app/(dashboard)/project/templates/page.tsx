"use client";

import { useEffect, useMemo, useState } from "react";
import { useTranslations } from "next-intl";
import { useProjectTemplateStore } from "@/lib/stores/project-template-store";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogDescription,
} from "@/components/ui/dialog";

/**
 * Project templates management page.
 *
 * Lists every template visible to the caller (system + their own user
 * templates + marketplace installs) and lets them rename or delete the ones
 * they own. System templates are shown as read-only reference entries.
 *
 * This page is the primary admin surface for the "save this project as a
 * template" flow — the save-as-template dialog is launched elsewhere, but
 * managing the resulting library lives here.
 */
export default function ProjectTemplatesPage() {
  const t = useTranslations("projectTemplates");
  const templates = useProjectTemplateStore((s) => s.templates);
  const loading = useProjectTemplateStore((s) => s.loading);
  const fetchTemplates = useProjectTemplateStore((s) => s.fetchTemplates);
  const updateTemplate = useProjectTemplateStore((s) => s.updateTemplate);
  const deleteTemplate = useProjectTemplateStore((s) => s.deleteTemplate);

  const [deleteTarget, setDeleteTarget] = useState<string | null>(null);
  const [editing, setEditing] = useState<string | null>(null);
  const [editName, setEditName] = useState("");
  const [editDescription, setEditDescription] = useState("");

  useEffect(() => {
    fetchTemplates();
  }, [fetchTemplates]);

  const grouped = useMemo(() => {
    return {
      system: templates.filter((t) => t.source === "system"),
      user: templates.filter((t) => t.source === "user"),
      marketplace: templates.filter((t) => t.source === "marketplace"),
    };
  }, [templates]);

  const startEdit = (id: string) => {
    const tpl = templates.find((t) => t.id === id);
    if (!tpl) return;
    setEditing(id);
    setEditName(tpl.name);
    setEditDescription(tpl.description ?? "");
  };

  const submitEdit = async () => {
    if (!editing) return;
    await updateTemplate(editing, { name: editName, description: editDescription });
    setEditing(null);
  };

  const confirmDelete = async () => {
    if (!deleteTarget) return;
    await deleteTemplate(deleteTarget);
    setDeleteTarget(null);
  };

  return (
    <div className="flex flex-col gap-[var(--space-section-gap)] p-[var(--space-page-inline)]">
      <header className="flex flex-col gap-1">
        <h1 className="text-fluid-title font-semibold tracking-tight">{t("page.title")}</h1>
        <p className="text-fluid-body text-muted-foreground">{t("page.description")}</p>
      </header>

      {loading ? (
        <p className="text-sm text-muted-foreground">{t("page.loading")}</p>
      ) : (
        <div className="flex flex-col gap-8">
          {(["user", "system", "marketplace"] as const).map((source) => (
            <section key={source} className="flex flex-col gap-3">
              <h2 className="text-sm font-semibold uppercase text-muted-foreground">
                {t(`source.${source}`)}
              </h2>
              {grouped[source].length === 0 ? (
                <p className="text-sm text-muted-foreground">
                  {t(`page.empty.${source}`)}
                </p>
              ) : (
                <ul className="flex flex-col divide-y rounded-md border">
                  {grouped[source].map((tpl) => (
                    <li
                      key={tpl.id}
                      className="flex items-start justify-between gap-4 p-4"
                    >
                      <div className="flex flex-col gap-1">
                        <p className="font-medium">{tpl.name}</p>
                        {tpl.description && (
                          <p className="text-sm text-muted-foreground">
                            {tpl.description}
                          </p>
                        )}
                        <p className="text-xs text-muted-foreground">
                          {t("page.versionLabel", { version: tpl.snapshotVersion })}
                        </p>
                      </div>
                      {source !== "system" && (
                        <div className="flex shrink-0 gap-2">
                          <Button
                            type="button"
                            size="sm"
                            variant="outline"
                            onClick={() => startEdit(tpl.id)}
                          >
                            {t("page.edit")}
                          </Button>
                          <Button
                            type="button"
                            size="sm"
                            variant="destructive"
                            onClick={() => setDeleteTarget(tpl.id)}
                          >
                            {t("page.delete")}
                          </Button>
                        </div>
                      )}
                    </li>
                  ))}
                </ul>
              )}
            </section>
          ))}
        </div>
      )}

      <Dialog open={!!editing} onOpenChange={(v) => !v && setEditing(null)}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>{t("page.editTitle")}</DialogTitle>
            <DialogDescription>{t("page.editDescription")}</DialogDescription>
          </DialogHeader>
          <div className="flex flex-col gap-4 py-4">
            <label className="flex flex-col gap-2 text-sm">
              {t("saveAs.nameLabel")}
              <input
                className="rounded-md border px-3 py-2"
                value={editName}
                onChange={(e) => setEditName(e.target.value)}
                maxLength={128}
              />
            </label>
            <label className="flex flex-col gap-2 text-sm">
              {t("saveAs.descriptionLabel")}
              <textarea
                className="rounded-md border px-3 py-2"
                rows={4}
                value={editDescription}
                onChange={(e) => setEditDescription(e.target.value)}
                maxLength={4096}
              />
            </label>
          </div>
          <DialogFooter>
            <Button variant="ghost" onClick={() => setEditing(null)}>
              {t("saveAs.cancel")}
            </Button>
            <Button onClick={submitEdit} disabled={!editName.trim()}>
              {t("saveAs.save")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={!!deleteTarget} onOpenChange={(v) => !v && setDeleteTarget(null)}>
        <DialogContent className="sm:max-w-sm">
          <DialogHeader>
            <DialogTitle>{t("page.deleteTitle")}</DialogTitle>
            <DialogDescription>{t("page.deleteDescription")}</DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="ghost" onClick={() => setDeleteTarget(null)}>
              {t("saveAs.cancel")}
            </Button>
            <Button variant="destructive" onClick={confirmDelete}>
              {t("page.delete")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
