"use client";

import { useTranslations } from "next-intl";
import { Card, CardContent } from "@/components/ui/card";
import { FieldDefinitionEditor } from "@/components/fields/field-definition-editor";
import type { ProjectSectionProps } from "./types";

export function SectionCustomFields({ project }: ProjectSectionProps) {
  const t = useTranslations("settings");

  return (
    <div className="space-y-6">
      <div>
        <h2 className="text-lg font-semibold">{t("customFields")}</h2>
        <p className="text-sm text-muted-foreground">{t("customFieldsDesc")}</p>
      </div>

      <Card>
        <CardContent className="pt-6">
          <FieldDefinitionEditor projectId={project.id} />
        </CardContent>
      </Card>
    </div>
  );
}
