"use client";

import { useTranslations } from "next-intl";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Label } from "@/components/ui/label";
import { RadioGroup, RadioGroupItem } from "@/components/ui/radio-group";
import { Switch } from "@/components/ui/switch";
import {
  useAppearanceStore,
  type MotionPreference,
} from "@/lib/stores/appearance-store";
import { useA11yPreferences } from "@/hooks/use-a11y-preferences";

const MOTION_OPTIONS: Array<{
  value: MotionPreference;
  labelKey: string;
  descKey: string;
}> = [
  { value: "system", labelKey: "a11y.motionSystem", descKey: "a11y.motionSystemDesc" },
  { value: "reduce", labelKey: "a11y.motionReduce", descKey: "a11y.motionReduceDesc" },
  { value: "allow", labelKey: "a11y.motionAllow", descKey: "a11y.motionAllowDesc" },
];

export function SectionAccessibility() {
  const t = useTranslations("settings");
  const motionPreference = useAppearanceStore((s) => s.motionPreference);
  const setMotionPreference = useAppearanceStore((s) => s.setMotionPreference);
  const highContrast = useAppearanceStore((s) => s.highContrast);
  const setHighContrast = useAppearanceStore((s) => s.setHighContrast);
  const screenReaderMode = useAppearanceStore((s) => s.screenReaderMode);
  const setScreenReaderMode = useAppearanceStore((s) => s.setScreenReaderMode);
  const resetAppearance = useAppearanceStore((s) => s.resetAppearance);

  const {
    systemPrefersReducedMotion,
    systemPrefersHighContrast,
    reducedMotionActive,
    highContrast: highContrastActive,
  } = useA11yPreferences();

  return (
    <div className="space-y-6">
      <div>
        <h2 className="text-lg font-semibold">{t("a11y.title")}</h2>
        <p className="text-sm text-muted-foreground">{t("a11y.description")}</p>
      </div>

      <Card>
        <CardHeader>
          <CardTitle className="text-base">{t("a11y.motionTitle")}</CardTitle>
          <CardDescription>{t("a11y.motionDesc")}</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <RadioGroup
            value={motionPreference}
            onValueChange={(v) => setMotionPreference(v as MotionPreference)}
            aria-label={t("a11y.motionTitle")}
            className="grid gap-3"
          >
            {MOTION_OPTIONS.map((option) => (
              <div
                key={option.value}
                className="flex items-start gap-3 rounded-md border p-3"
              >
                <RadioGroupItem
                  id={`motion-${option.value}`}
                  value={option.value}
                  className="mt-0.5"
                  aria-label={t(option.labelKey)}
                />
                <Label
                  htmlFor={`motion-${option.value}`}
                  className="flex cursor-pointer flex-col gap-0.5"
                >
                  <span className="text-sm font-medium">{t(option.labelKey)}</span>
                  <span className="text-xs text-muted-foreground">
                    {t(option.descKey)}
                  </span>
                </Label>
              </div>
            ))}
          </RadioGroup>
          <div className="flex items-center gap-2 text-xs text-muted-foreground">
            <Badge variant="outline" data-testid="motion-system-status">
              {systemPrefersReducedMotion
                ? t("a11y.systemDetected")
                : t("a11y.systemNotDetected")}
            </Badge>
            <Badge
              variant={reducedMotionActive ? "default" : "secondary"}
              data-testid="motion-effective"
            >
              {reducedMotionActive ? t("a11y.enabled") : t("a11y.disabled")}
            </Badge>
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle className="text-base">{t("a11y.contrastTitle")}</CardTitle>
          <CardDescription>{t("a11y.contrastDesc")}</CardDescription>
        </CardHeader>
        <CardContent className="space-y-3">
          <div className="flex items-center justify-between gap-3">
            <Label htmlFor="high-contrast-toggle" className="text-sm font-medium">
              {t("a11y.contrastTitle")}
            </Label>
            <Switch
              id="high-contrast-toggle"
              checked={highContrast}
              onCheckedChange={setHighContrast}
              aria-label={t("a11y.contrastTitle")}
            />
          </div>
          {systemPrefersHighContrast ? (
            <Badge variant="outline">{t("a11y.contrastSystemDetected")}</Badge>
          ) : null}
          <Badge
            variant={highContrastActive ? "default" : "secondary"}
            data-testid="contrast-effective"
          >
            {highContrastActive ? t("a11y.enabled") : t("a11y.disabled")}
          </Badge>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle className="text-base">{t("a11y.screenReaderTitle")}</CardTitle>
          <CardDescription>{t("a11y.screenReaderDesc")}</CardDescription>
        </CardHeader>
        <CardContent>
          <div className="flex items-center justify-between gap-3">
            <Label htmlFor="sr-toggle" className="text-sm font-medium">
              {t("a11y.screenReaderTitle")}
            </Label>
            <Switch
              id="sr-toggle"
              checked={screenReaderMode}
              onCheckedChange={setScreenReaderMode}
              aria-label={t("a11y.screenReaderTitle")}
            />
          </div>
        </CardContent>
      </Card>

      <div className="flex justify-end">
        <Button variant="outline" size="sm" onClick={resetAppearance}>
          {t("a11y.resetAll")}
        </Button>
      </div>
    </div>
  );
}
