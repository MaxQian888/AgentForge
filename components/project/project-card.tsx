"use client";

import { useState } from "react";
import Link from "next/link";
import { useTranslations } from "next-intl";
import { FolderKanban, GitBranch, Pencil, Trash2 } from "lucide-react";
import { Card, CardContent } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { ConfirmDialog } from "@/components/shared/confirm-dialog";
import { buildProjectScopedHref } from "@/lib/route-hrefs";
import type { Project } from "@/lib/stores/project-store";

const statusColors: Record<string, string> = {
  active: "bg-emerald-500/15 text-emerald-700 dark:text-emerald-400",
  archived: "bg-zinc-500/15 text-zinc-600 dark:text-zinc-400",
  paused: "bg-amber-500/15 text-amber-700 dark:text-amber-400",
};

interface ProjectCardProps {
  project: Project;
  onEdit: (project: Project) => void;
  onDelete: (id: string) => void;
}

export function ProjectCard({ project, onEdit, onDelete }: ProjectCardProps) {
  const t = useTranslations("projects");
  const [confirmDelete, setConfirmDelete] = useState(false);

  return (
    <>
      <Link href={buildProjectScopedHref("/", { projectId: project.id })}>
        <Card className="transition-shadow hover:shadow-md">
          <CardContent className="flex flex-col gap-3 py-4">
            {/* Header: icon + name + status + actions */}
            <div className="flex items-center justify-between gap-2">
              <div className="flex min-w-0 items-center gap-2">
                <FolderKanban className="size-4 shrink-0 text-primary" />
                <span className="truncate font-medium">{project.name}</span>
              </div>
              <div className="flex shrink-0 items-center gap-1">
                <Badge
                  variant="secondary"
                  className={
                    statusColors[project.status] ?? statusColors.active
                  }
                >
                  {t(`status.${project.status}` as Parameters<typeof t>[0])}
                </Badge>
                <Button
                  variant="ghost"
                  size="icon"
                  className="size-7"
                  onClick={(e) => {
                    e.preventDefault();
                    e.stopPropagation();
                    onEdit(project);
                  }}
                >
                  <Pencil className="size-3.5" />
                </Button>
                <Button
                  variant="ghost"
                  size="icon"
                  className="size-7 text-destructive hover:text-destructive"
                  onClick={(e) => {
                    e.preventDefault();
                    e.stopPropagation();
                    setConfirmDelete(true);
                  }}
                >
                  <Trash2 className="size-3.5" />
                </Button>
              </div>
            </div>

            {/* Description */}
            <p className="text-sm text-muted-foreground line-clamp-2">
              {project.description || t("noDescription")}
            </p>

            {/* Badges */}
            <div className="flex items-center gap-2">
              <Badge variant="secondary">
                {t("taskCount", { count: project.taskCount })}
              </Badge>
              <Badge variant="outline">
                {t("agentCount", { count: project.agentCount })}
              </Badge>
              {project.repoUrl && (
                <GitBranch className="size-3.5 text-muted-foreground" />
              )}
            </div>

            {/* Footer */}
            <p className="text-xs text-muted-foreground">
              {t("card.created", {
                date: new Date(project.createdAt).toLocaleDateString(),
              })}
            </p>
          </CardContent>
        </Card>
      </Link>

      <ConfirmDialog
        open={confirmDelete}
        title={t("deleteProject.title")}
        description={t("deleteProject.description", { name: project.name })}
        confirmLabel={t("deleteProject.confirm")}
        variant="destructive"
        onConfirm={() => {
          setConfirmDelete(false);
          onDelete(project.id);
        }}
        onCancel={() => setConfirmDelete(false)}
      />
    </>
  );
}
