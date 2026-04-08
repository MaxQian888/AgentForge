"use client";

import { useTranslations } from "next-intl";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { ThemeToggle } from "@/components/ui/theme-toggle";
import { useLocaleStore, SUPPORTED_LOCALES, type Locale } from "@/lib/stores/locale-store";

const LOCALE_LABELS: Record<Locale, string> = {
  en: "English",
  "zh-CN": "中文（简体）",
};

export function SectionAppearance() {
  const t = useTranslations("settings");
  const locale = useLocaleStore((s) => s.locale);
  const setLocale = useLocaleStore((s) => s.setLocale);

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
    </div>
  );
}
