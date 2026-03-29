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
import type { Project, ProjectUpdateInput } from "@/lib/stores/project-store";

interface EditProjectDialogProps {
  open: boolean;
  project: Project;
  onSave: (id: string, input: ProjectUpdateInput) => Promise<void>;
  onClose: () => void;
}

export function EditProjectDialog({
  open,
  project,
  onSave,
  onClose,
}: EditProjectDialogProps) {
  const t = useTranslations("projects");
  const [name, setName] = useState(project.name);
  const [description, setDescription] = useState(project.description);
  const [repoUrl, setRepoUrl] = useState(project.repoUrl ?? "");
  const [defaultBranch, setDefaultBranch] = useState(
    project.defaultBranch ?? "main"
  );
  const [saving, setSaving] = useState(false);

  const handleSave = async () => {
    setSaving(true);
    try {
      const input: ProjectUpdateInput = {};
      if (name.trim() !== project.name) input.name = name.trim();
      if (description !== project.description) input.description = description;
      if (repoUrl !== (project.repoUrl ?? "")) input.repoUrl = repoUrl;
      if (defaultBranch !== (project.defaultBranch ?? "main"))
        input.defaultBranch = defaultBranch;
      await onSave(project.id, input);
      onClose();
    } finally {
      setSaving(false);
    }
  };

  return (
    <Dialog open={open} onOpenChange={(v) => !v && onClose()}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>{t("editProject.title")}</DialogTitle>
          <DialogDescription>{t("editProject.description")}</DialogDescription>
        </DialogHeader>
        <div className="flex flex-col gap-4 py-4">
          <div className="flex flex-col gap-2">
            <Label htmlFor="edit-project-name">
              {t("editProject.nameLabel")}
            </Label>
            <Input
              id="edit-project-name"
              value={name}
              onChange={(e) => setName(e.target.value)}
              required
            />
          </div>
          <div className="flex flex-col gap-2">
            <Label htmlFor="edit-project-desc">
              {t("editProject.descriptionLabel")}
            </Label>
            <Input
              id="edit-project-desc"
              value={description}
              onChange={(e) => setDescription(e.target.value)}
            />
          </div>
          <div className="flex flex-col gap-2">
            <Label htmlFor="edit-project-repo">
              {t("editProject.repoUrlLabel")}
            </Label>
            <Input
              id="edit-project-repo"
              value={repoUrl}
              onChange={(e) => setRepoUrl(e.target.value)}
              placeholder="https://github.com/org/repo"
            />
          </div>
          <div className="flex flex-col gap-2">
            <Label htmlFor="edit-project-branch">
              {t("editProject.defaultBranchLabel")}
            </Label>
            <Input
              id="edit-project-branch"
              value={defaultBranch}
              onChange={(e) => setDefaultBranch(e.target.value)}
            />
          </div>
        </div>
        <DialogFooter>
          <Button variant="ghost" onClick={onClose}>
            {t("editProject.cancel")}
          </Button>
          <Button
            onClick={handleSave}
            disabled={saving || !name.trim()}
          >
            {saving ? t("editProject.saving") : t("editProject.save")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
