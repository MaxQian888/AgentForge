"use client";

import { useTranslations } from "next-intl";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { FieldError } from "@/components/shared/field-error";
import type { ProjectSectionProps } from "./types";

export function SectionGeneral({ draft, patchDraft, validationErrors, clearValidationError }: ProjectSectionProps) {
  const t = useTranslations("settings");

  return (
    <div className="space-y-6">
      <div>
        <h2 className="text-lg font-semibold">{t("general")}</h2>
      </div>

      <Card>
        <CardHeader>
          <CardTitle className="text-base">{t("projectName")}</CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="flex flex-col gap-2">
            <Label htmlFor="settings-project-name">{t("projectName")}</Label>
            <Input
              id="settings-project-name"
              value={draft.name}
              aria-invalid={Boolean(validationErrors.name)}
              onChange={(e) => {
                patchDraft((d) => ({ ...d, name: e.target.value }));
                clearValidationError("name");
              }}
            />
            <FieldError message={validationErrors.name} />
          </div>
          <div className="flex flex-col gap-2">
            <Label htmlFor="settings-description">{t("description")}</Label>
            <Input
              id="settings-description"
              value={draft.description}
              onChange={(e) => patchDraft((d) => ({ ...d, description: e.target.value }))}
            />
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
