"use client";

import { useTranslations } from "next-intl";
import { Card, CardContent } from "@/components/ui/card";
import { RuleEditor } from "@/components/automations/rule-editor";
import { RuleList } from "@/components/automations/rule-list";
import { AutomationLogViewer } from "@/components/automations/automation-log-viewer";
import type { ProjectSectionProps } from "./types";

export function SectionAutomations({ project }: ProjectSectionProps) {
  const t = useTranslations("settings");

  return (
    <div className="space-y-6">
      <div>
        <h2 className="text-lg font-semibold">{t("automationsLabel")}</h2>
        <p className="text-sm text-muted-foreground">{t("automationsDesc")}</p>
      </div>

      <Card>
        <CardContent className="space-y-6 pt-6">
          <RuleEditor projectId={project.id} />
          <RuleList projectId={project.id} />
          <AutomationLogViewer projectId={project.id} />
        </CardContent>
      </Card>
    </div>
  );
}
