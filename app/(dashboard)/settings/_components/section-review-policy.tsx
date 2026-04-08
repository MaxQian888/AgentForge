"use client";

import { useTranslations } from "next-intl";
import { Card, CardContent } from "@/components/ui/card";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  getPrimaryReviewLayerLabel,
  getMinRiskLevelForBlockValue,
} from "@/lib/settings/project-settings-workspace";
import type { ProjectSectionProps } from "./types";

export function SectionReviewPolicy({ draft, patchDraft }: ProjectSectionProps) {
  const t = useTranslations("settings");

  const patchReview = (field: string, value: unknown) => {
    patchDraft((d) => ({
      ...d,
      settings: {
        ...d.settings,
        reviewPolicy: { ...d.settings.reviewPolicy, [field]: value },
      },
    }));
  };

  return (
    <div className="space-y-6">
      <div>
        <h2 className="text-lg font-semibold">{t("reviewPolicy")}</h2>
        <p className="text-sm text-muted-foreground">{t("reviewPolicyDesc")}</p>
      </div>

      <Card>
        <CardContent className="space-y-4 pt-6">
          <div className="grid gap-4 md:grid-cols-2">
            <div className="flex flex-col gap-2">
              <Label>{t("autoTriggerOnPR")}</Label>
              <Select
                value={draft.settings.reviewPolicy.autoTriggerOnPR ? "yes" : "no"}
                onValueChange={(v) => patchReview("autoTriggerOnPR", v === "yes")}
              >
                <SelectTrigger><SelectValue /></SelectTrigger>
                <SelectContent>
                  <SelectItem value="yes">{t("enabled")}</SelectItem>
                  <SelectItem value="no">{t("disabled")}</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div className="flex flex-col gap-2">
              <Label>{t("requiredReviewLayers")}</Label>
              <Select
                value={getPrimaryReviewLayerLabel(draft.settings.reviewPolicy.requiredLayers)}
                onValueChange={(v) => patchReview("requiredLayers", v === "none" ? [] : [v])}
              >
                <SelectTrigger><SelectValue /></SelectTrigger>
                <SelectContent>
                  <SelectItem value="none">{t("disabled")}</SelectItem>
                  <SelectItem value="layer1">{t("layerQuick")}</SelectItem>
                  <SelectItem value="layer2">{t("layerDeep")}</SelectItem>
                  <SelectItem value="layer3">{t("layerHuman")}</SelectItem>
                </SelectContent>
              </Select>
            </div>
          </div>
          <div className="grid gap-4 md:grid-cols-2">
            <div className="flex flex-col gap-2">
              <Label>{t("minRiskLevelForBlock")}</Label>
              <Select
                value={getMinRiskLevelForBlockValue(draft.settings.reviewPolicy.minRiskLevelForBlock)}
                onValueChange={(v) => patchReview("minRiskLevelForBlock", v === "none" ? "" : v)}
              >
                <SelectTrigger><SelectValue /></SelectTrigger>
                <SelectContent>
                  <SelectItem value="none">{t("disabled")}</SelectItem>
                  <SelectItem value="critical">{t("riskCritical")}</SelectItem>
                  <SelectItem value="high">{t("riskHigh")}</SelectItem>
                  <SelectItem value="medium">{t("riskMedium")}</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div className="flex flex-col gap-2">
              <Label>{t("requireManualApproval")}</Label>
              <Select
                value={draft.settings.reviewPolicy.requireManualApproval ? "yes" : "no"}
                onValueChange={(v) => patchReview("requireManualApproval", v === "yes")}
              >
                <SelectTrigger><SelectValue /></SelectTrigger>
                <SelectContent>
                  <SelectItem value="yes">{t("required")}</SelectItem>
                  <SelectItem value="no">{t("notRequired")}</SelectItem>
                </SelectContent>
              </Select>
            </div>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
