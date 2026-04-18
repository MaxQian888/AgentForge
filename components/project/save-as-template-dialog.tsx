"use client";

import { useState } from "react";
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
import { useProjectTemplateStore } from "@/lib/stores/project-template-store";

interface SaveAsTemplateDialogProps {
  open: boolean;
  projectId: string;
  projectName: string;
  onClose: () => void;
}

/**
 * Admin-gated affordance that serializes the current project's configuration
 * into a new user-source project template. RBAC (admin+) is enforced on the
 * backend via `project.save_as_template`; the caller is responsible for only
 * rendering this dialog to admins/owners.
 */
export function SaveAsTemplateDialog({
  open,
  projectId,
  projectName,
  onClose,
}: SaveAsTemplateDialogProps) {
  const t = useTranslations("projectTemplates");
  const saveAsTemplate = useProjectTemplateStore((s) => s.saveAsTemplate);
  const saving = useProjectTemplateStore((s) => s.saving);
  const [name, setName] = useState(`${projectName} Template`);
  const [description, setDescription] = useState("");

  const handleSave = async () => {
    const trimmed = name.trim();
    if (!trimmed) return;
    const result = await saveAsTemplate(projectId, { name: trimmed, description });
    if (result) {
      onClose();
    }
  };

  return (
    <Dialog open={open} onOpenChange={(v) => !v && onClose()}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>{t("saveAs.title")}</DialogTitle>
          <DialogDescription>{t("saveAs.description")}</DialogDescription>
        </DialogHeader>
        <div className="flex flex-col gap-4 py-4">
          <div className="flex flex-col gap-2">
            <Label htmlFor="save-as-template-name">{t("saveAs.nameLabel")}</Label>
            <Input
              id="save-as-template-name"
              value={name}
              onChange={(e) => setName(e.target.value)}
              required
              maxLength={128}
            />
          </div>
          <div className="flex flex-col gap-2">
            <Label htmlFor="save-as-template-description">
              {t("saveAs.descriptionLabel")}
            </Label>
            <Textarea
              id="save-as-template-description"
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              rows={4}
              maxLength={4096}
              placeholder={t("saveAs.descriptionPlaceholder")}
            />
          </div>
        </div>
        <DialogFooter>
          <Button variant="ghost" onClick={onClose} disabled={saving}>
            {t("saveAs.cancel")}
          </Button>
          <Button onClick={handleSave} disabled={saving || !name.trim()}>
            {saving ? t("saveAs.saving") : t("saveAs.save")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
