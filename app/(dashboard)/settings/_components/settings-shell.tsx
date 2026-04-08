"use client";

import { useTranslations } from "next-intl";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { ScrollArea } from "@/components/ui/scroll-area";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Separator } from "@/components/ui/separator";
import { ErrorBanner } from "@/components/shared/error-banner";
import { SettingsSidebar, SETTINGS_SECTIONS } from "./settings-sidebar";

interface SettingsShellProps {
  activeSection: string;
  onSectionChange: (id: string) => void;
  hasProject: boolean;
  dirty: boolean;
  saveState: "idle" | "saving" | "saved" | "error";
  saveError: string | null;
  onSave: () => void;
  onDiscard: () => void;
  children: React.ReactNode;
}

export function SettingsShell({
  activeSection,
  onSectionChange,
  hasProject,
  dirty,
  saveState,
  saveError,
  onSave,
  onDiscard,
  children,
}: SettingsShellProps) {
  const t = useTranslations("settings");

  return (
    <div className="flex h-[calc(100vh-3.5rem)] flex-col">
      {/* Header */}
      <div className="flex items-center justify-between border-b px-6 py-3">
        <h1 className="text-lg font-semibold">{t("title")}</h1>
        <div className="flex items-center gap-2 text-sm">
          {dirty ? (
            <Badge variant="secondary">{t("unsavedChanges")}</Badge>
          ) : (
            <Badge variant="outline">{t("allChangesSaved")}</Badge>
          )}
          {saveState === "saved" && (
            <span className="text-emerald-600 dark:text-emerald-400">{t("settingsSaved")}</span>
          )}
        </div>
      </div>

      <div className="flex min-h-0 flex-1">
        {/* Desktop sidebar */}
        <aside className="hidden w-56 shrink-0 border-r md:block">
          <SettingsSidebar
            activeSection={activeSection}
            onSectionChange={onSectionChange}
            hasProject={hasProject}
          />
        </aside>

        {/* Content area */}
        <div className="flex min-h-0 flex-1 flex-col">
          {/* Mobile section selector */}
          <div className="border-b p-3 md:hidden">
            <Select value={activeSection} onValueChange={onSectionChange}>
              <SelectTrigger className="w-full">
                <SelectValue placeholder={t("mobileNavLabel")} />
              </SelectTrigger>
              <SelectContent>
                {SETTINGS_SECTIONS.map((section) => {
                  const isDisabled = section.group === "project" && !hasProject;
                  return (
                    <SelectItem
                      key={section.id}
                      value={section.id}
                      disabled={isDisabled}
                    >
                      {t(section.labelKey)}
                    </SelectItem>
                  );
                })}
              </SelectContent>
            </Select>
          </div>

          {/* Scrollable content */}
          <ScrollArea className="flex-1">
            <div className="mx-auto w-full max-w-3xl p-6">
              {children}
            </div>
          </ScrollArea>

          {/* Save bar */}
          {dirty && (
            <>
              <Separator />
              <div className="shrink-0 border-t bg-background/95 px-6 py-3 backdrop-blur-sm">
                <div className="mx-auto flex max-w-3xl items-center justify-between gap-3">
                  <div className="flex-1">
                    {saveError && <ErrorBanner message={saveError} />}
                  </div>
                  <div className="flex items-center gap-2">
                    <Button variant="outline" onClick={onDiscard}>
                      {t("discardChanges")}
                    </Button>
                    <Button disabled={saveState === "saving"} onClick={onSave}>
                      {saveState === "saving" ? t("savingSettings") : t("saveSettings")}
                    </Button>
                  </div>
                </div>
              </div>
            </>
          )}
        </div>
      </div>
    </div>
  );
}
