"use client";

import { Archive } from "lucide-react";
import { useTranslations } from "next-intl";
import type { Project } from "@/lib/stores/project-store";

interface ArchivedProjectBannerProps {
  project: Project | null | undefined;
}

export function ArchivedProjectBanner({ project }: ArchivedProjectBannerProps) {
  const t = useTranslations("projects");
  if (!project || project.status !== "archived") return null;
  const archivedAt = project.archivedAt
    ? new Date(project.archivedAt).toLocaleDateString()
    : null;
  return (
    <div className="flex items-center gap-3 rounded-md border border-zinc-500/30 bg-zinc-500/10 px-4 py-3 text-sm">
      <Archive className="size-4 shrink-0 text-zinc-600 dark:text-zinc-400" />
      <div className="flex flex-col gap-0.5">
        <p className="font-medium">{t("workspaceReadOnly.title")}</p>
        <p className="text-xs text-muted-foreground">
          {t("workspaceReadOnly.body")}
          {archivedAt ? ` · ${t("card.archivedOn", { date: archivedAt })}` : ""}
        </p>
      </div>
    </div>
  );
}
