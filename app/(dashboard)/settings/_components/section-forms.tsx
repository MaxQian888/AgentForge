"use client";

import { useTranslations } from "next-intl";
import { Card, CardContent } from "@/components/ui/card";
import { FormBuilder } from "@/components/forms/form-builder";
import type { ProjectSectionProps } from "./types";

export function SectionForms({ project }: ProjectSectionProps) {
  const t = useTranslations("settings");

  return (
    <div className="space-y-6">
      <div>
        <h2 className="text-lg font-semibold">{t("forms")}</h2>
        <p className="text-sm text-muted-foreground">{t("formsDesc")}</p>
      </div>

      <Card>
        <CardContent className="pt-6">
          <FormBuilder projectId={project.id} />
        </CardContent>
      </Card>
    </div>
  );
}
