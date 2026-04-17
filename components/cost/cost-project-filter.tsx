"use client";

import { useTranslations } from "next-intl";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";

export interface CostProjectOption {
  id: string;
  name: string;
}

interface CostProjectFilterProps {
  projects: CostProjectOption[];
  selectedProjectId: string | null;
  onChange: (projectId: string | null) => void;
}

const ALL_PROJECTS_VALUE = "__all__";

export function CostProjectFilter({
  projects,
  selectedProjectId,
  onChange,
}: CostProjectFilterProps) {
  const t = useTranslations("cost");

  return (
    <div className="flex items-center gap-2">
      <span className="text-xs text-muted-foreground">
        {t("projectFilterLabel")}
      </span>
      <Select
        value={selectedProjectId ?? ALL_PROJECTS_VALUE}
        onValueChange={(next) =>
          onChange(next === ALL_PROJECTS_VALUE ? null : next)
        }
      >
        <SelectTrigger
          className="h-8 w-[220px] text-sm"
          aria-label={t("projectFilterLabel")}
        >
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value={ALL_PROJECTS_VALUE}>
            {t("projectFilterAll")}
          </SelectItem>
          {projects.map((project) => (
            <SelectItem key={project.id} value={project.id}>
              {project.name}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    </div>
  );
}
