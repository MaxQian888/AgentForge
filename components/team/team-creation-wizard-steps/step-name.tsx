"use client";

import { useTranslations } from "next-intl";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";

export interface StepNameData {
  name: string;
  description: string;
  objective: string;
}

interface StepNameProps {
  data: StepNameData;
  onChange: (data: StepNameData) => void;
}

export function StepName({ data, onChange }: StepNameProps) {
  const t = useTranslations("teams");

  return (
    <div className="flex flex-col gap-5">
      <div className="flex flex-col gap-2">
        <Label htmlFor="team-name">
          {t("wizard.nameLabel")} <span className="text-destructive">*</span>
        </Label>
        <Input
          id="team-name"
          value={data.name}
          onChange={(e) => onChange({ ...data, name: e.target.value })}
          placeholder={t("wizard.namePlaceholder")}
        />
      </div>

      <div className="flex flex-col gap-2">
        <Label htmlFor="team-description">{t("wizard.descriptionLabel")}</Label>
        <textarea
          id="team-description"
          value={data.description}
          onChange={(e) => onChange({ ...data, description: e.target.value })}
          placeholder={t("wizard.descriptionPlaceholder")}
          rows={3}
          className="w-full min-w-0 rounded-md border border-input bg-transparent px-3 py-2 text-base shadow-xs transition-[color,box-shadow] outline-none placeholder:text-muted-foreground focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50 disabled:pointer-events-none disabled:cursor-not-allowed disabled:opacity-50 md:text-sm dark:bg-input/30"
        />
      </div>

      <div className="flex flex-col gap-2">
        <Label htmlFor="team-objective">{t("wizard.objectiveLabel")}</Label>
        <Input
          id="team-objective"
          value={data.objective}
          onChange={(e) => onChange({ ...data, objective: e.target.value })}
          placeholder={t("wizard.objectivePlaceholder")}
        />
      </div>
    </div>
  );
}
