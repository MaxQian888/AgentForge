"use client";

import { useTranslations } from "next-intl";
import { Card, CardContent } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { FieldError } from "@/components/shared/field-error";
import type { ProjectSectionProps } from "./types";

export function SectionRepository({ draft, patchDraft, validationErrors, clearValidationError }: ProjectSectionProps) {
  const t = useTranslations("settings");

  return (
    <div className="space-y-6">
      <div>
        <h2 className="text-lg font-semibold">{t("repository")}</h2>
      </div>

      <Card>
        <CardContent className="space-y-4 pt-6">
          <div className="flex flex-col gap-2">
            <Label htmlFor="settings-repo-url">{t("repoUrl")}</Label>
            <Input
              id="settings-repo-url"
              value={draft.repoUrl}
              placeholder={t("repoUrlPlaceholder")}
              onChange={(e) => patchDraft((d) => ({ ...d, repoUrl: e.target.value }))}
            />
          </div>
          <div className="flex flex-col gap-2">
            <Label htmlFor="settings-default-branch">{t("defaultBranch")}</Label>
            <Input
              id="settings-default-branch"
              value={draft.defaultBranch}
              aria-invalid={Boolean(validationErrors.defaultBranch)}
              onChange={(e) => {
                patchDraft((d) => ({ ...d, defaultBranch: e.target.value }));
                clearValidationError("defaultBranch");
              }}
            />
            <FieldError message={validationErrors.defaultBranch} />
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
