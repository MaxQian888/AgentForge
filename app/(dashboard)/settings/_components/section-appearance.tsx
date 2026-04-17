"use client";

import { useState } from "react";
import { useTranslations } from "next-intl";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Label } from "@/components/ui/label";
import { RadioGroup, RadioGroupItem } from "@/components/ui/radio-group";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { ThemeToggle } from "@/components/ui/theme-toggle";
import { useLocaleStore, SUPPORTED_LOCALES, type Locale } from "@/lib/stores/locale-store";
import {
  useAppearanceStore,
  type Density,
} from "@/lib/stores/appearance-store";
import { useA11yPreferences } from "@/hooks/use-a11y-preferences";
import { cn } from "@/lib/utils";

const LOCALE_LABELS: Record<Locale, string> = {
  en: "English",
  "zh-CN": "中文（简体）",
};

const DENSITY_OPTIONS: Array<{ value: Density; labelKey: string; descKey: string }> = [
  { value: "compact", labelKey: "density.compact", descKey: "density.compactDesc" },
  { value: "comfortable", labelKey: "density.comfortable", descKey: "density.comfortableDesc" },
  { value: "spacious", labelKey: "density.spacious", descKey: "density.spaciousDesc" },
];

function AppearancePreview({ density }: { density: Density }) {
  const t = useTranslations("settings");
  // Scope overrides to the preview only so hover-preview doesn't mutate the page.
  const scale = density === "compact" ? 0.875 : density === "spacious" ? 1.125 : 1;
  const paddingClass =
    density === "compact" ? "p-2" : density === "spacious" ? "p-5" : "p-4";
  const gapClass =
    density === "compact" ? "gap-1.5" : density === "spacious" ? "gap-4" : "gap-2.5";

  return (
    <Card aria-live="polite" data-testid="appearance-preview">
      <CardHeader>
        <CardTitle className="text-base">{t("preview.title")}</CardTitle>
        <CardDescription>{t("preview.description")}</CardDescription>
      </CardHeader>
      <CardContent>
        <div
          className={cn(
            "rounded-md border bg-card text-card-foreground transition-[padding]",
            paddingClass,
          )}
          style={{ fontSize: `${scale}rem` }}
          data-density-preview={density}
        >
          <div className={cn("flex flex-col", gapClass)}>
            <p className="font-medium">{t("preview.sampleHeading")}</p>
            <p className="text-muted-foreground">{t("preview.sampleBody")}</p>
            <div className={cn("flex flex-wrap", gapClass)}>
              <Button type="button" size="sm">
                {t("preview.sampleAction")}
              </Button>
              <Button type="button" size="sm" variant="outline">
                {t("preview.sampleSecondary")}
              </Button>
            </div>
          </div>
        </div>
      </CardContent>
    </Card>
  );
}

export function SectionAppearance() {
  const t = useTranslations("settings");
  const locale = useLocaleStore((s) => s.locale);
  const setLocale = useLocaleStore((s) => s.setLocale);

  const density = useAppearanceStore((s) => s.density);
  const setDensity = useAppearanceStore((s) => s.setDensity);
  // Subscribe to apply DOM attributes as settings change.
  useA11yPreferences();

  // Hover preview: temporarily show a different density in the preview card.
  const [hoverDensity, setHoverDensity] = useState<Density | null>(null);
  const previewDensity = hoverDensity ?? density;

  return (
    <div className="space-y-6">
      <div>
        <h2 className="text-lg font-semibold">{t("appearance")}</h2>
        <p className="text-sm text-muted-foreground">{t("appearanceDesc")}</p>
      </div>

      <Card>
        <CardHeader>
          <CardTitle className="text-base">{t("themeMode")}</CardTitle>
        </CardHeader>
        <CardContent>
          <ThemeToggle />
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle className="text-base">{t("language")}</CardTitle>
        </CardHeader>
        <CardContent>
          <Select value={locale} onValueChange={(v) => setLocale(v as Locale)}>
            <SelectTrigger className="w-full max-w-xs">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {SUPPORTED_LOCALES.map((loc) => (
                <SelectItem key={loc} value={loc}>
                  {LOCALE_LABELS[loc]}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle className="text-base">{t("density.title")}</CardTitle>
          <CardDescription>{t("density.description")}</CardDescription>
        </CardHeader>
        <CardContent>
          <RadioGroup
            value={density}
            onValueChange={(v) => setDensity(v as Density)}
            className="grid gap-3"
            aria-label={t("density.title")}
          >
            {DENSITY_OPTIONS.map((option) => (
              <div
                key={option.value}
                onMouseEnter={() => setHoverDensity(option.value)}
                onMouseLeave={() => setHoverDensity(null)}
                onFocus={() => setHoverDensity(option.value)}
                onBlur={() => setHoverDensity(null)}
                data-density-option={option.value}
                className={cn(
                  "flex items-start gap-3 rounded-md border p-3 transition-colors",
                  density === option.value
                    ? "border-primary bg-accent/40"
                    : "hover:bg-accent/30",
                )}
              >
                <RadioGroupItem
                  id={`density-${option.value}`}
                  value={option.value}
                  className="mt-0.5"
                  aria-label={t(option.labelKey)}
                />
                <Label
                  htmlFor={`density-${option.value}`}
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
        </CardContent>
      </Card>

      <AppearancePreview density={previewDensity} />
    </div>
  );
}
