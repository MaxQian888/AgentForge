"use client";

import { useTranslations } from "next-intl";
import {
  Palette,
  Accessibility,
  Globe,
  MessageCircle,
  Settings2,
  GitBranch,
  Bot,
  DollarSign,
  ShieldCheck,
  Webhook,
  ListTree,
  FileText,
  Zap,
  Wrench,
  ScrollText,
  Terminal,
  Sparkles,
  Code2,
  MousePointerClick,
  Gem,
  Cpu,
  Wind,
} from "lucide-react";
import type { LucideIcon } from "lucide-react";
import { ScrollArea } from "@/components/ui/scroll-area";
import { cn } from "@/lib/utils";

export interface SettingsSection {
  id: string;
  labelKey: string;
  icon: LucideIcon;
  group: "app" | "project" | "runtimes";
}

export const SETTINGS_SECTIONS: SettingsSection[] = [
  // App settings
  { id: "appearance", labelKey: "nav.appearance", icon: Palette, group: "app" },
  { id: "accessibility", labelKey: "nav.accessibility", icon: Accessibility, group: "app" },
  { id: "api-connection", labelKey: "nav.apiConnection", icon: Globe, group: "app" },
  { id: "im-bridge", labelKey: "nav.imBridge", icon: MessageCircle, group: "app" },
  // Runtime configurations
  { id: "runtime-claude_code", labelKey: "nav.runtimeClaudeCode", icon: Terminal, group: "runtimes" },
  { id: "runtime-codex", labelKey: "nav.runtimeCodex", icon: Sparkles, group: "runtimes" },
  { id: "runtime-opencode", labelKey: "nav.runtimeOpenCode", icon: Code2, group: "runtimes" },
  { id: "runtime-cursor", labelKey: "nav.runtimeCursor", icon: MousePointerClick, group: "runtimes" },
  { id: "runtime-gemini", labelKey: "nav.runtimeGemini", icon: Gem, group: "runtimes" },
  { id: "runtime-qoder", labelKey: "nav.runtimeQoder", icon: Cpu, group: "runtimes" },
  { id: "runtime-iflow", labelKey: "nav.runtimeIFlow", icon: Wind, group: "runtimes" },
  // Project settings
  { id: "general", labelKey: "nav.general", icon: Settings2, group: "project" },
  { id: "repository", labelKey: "nav.repository", icon: GitBranch, group: "project" },
  { id: "coding-agent", labelKey: "nav.codingAgent", icon: Bot, group: "project" },
  { id: "budget", labelKey: "nav.budget", icon: DollarSign, group: "project" },
  { id: "review-policy", labelKey: "nav.reviewPolicy", icon: ShieldCheck, group: "project" },
  { id: "webhook", labelKey: "nav.webhook", icon: Webhook, group: "project" },
  { id: "custom-fields", labelKey: "nav.customFields", icon: ListTree, group: "project" },
  { id: "forms", labelKey: "nav.forms", icon: FileText, group: "project" },
  { id: "automations", labelKey: "nav.automations", icon: Zap, group: "project" },
  { id: "audit-log", labelKey: "nav.auditLog", icon: ScrollText, group: "project" },
  { id: "advanced", labelKey: "nav.advanced", icon: Wrench, group: "project" },
];

interface SettingsSidebarProps {
  activeSection: string;
  onSectionChange: (id: string) => void;
  hasProject: boolean;
}

function NavGroup({
  label,
  sections,
  activeSection,
  onSectionChange,
  disabled,
}: {
  label: string;
  sections: SettingsSection[];
  activeSection: string;
  onSectionChange: (id: string) => void;
  disabled?: boolean;
}) {
  const t = useTranslations("settings");
  return (
    <>
      <p className="mb-1 mt-4 px-3 text-xs font-semibold uppercase tracking-wider text-muted-foreground first:mt-0">
        {label}
      </p>
      {sections.map((section) => {
        const Icon = section.icon;
        const isActive = activeSection === section.id;
        const isDisabled = disabled;
        return (
          <button
            key={section.id}
            type="button"
            disabled={isDisabled}
            onClick={() => onSectionChange(section.id)}
            className={cn(
              "flex w-full items-center gap-2.5 rounded-md px-3 py-1.5 text-sm transition-colors",
              "hover:bg-accent/50",
              isActive && "bg-accent text-accent-foreground font-medium",
              isDisabled && "pointer-events-none opacity-40",
            )}
          >
            <Icon className="size-4 shrink-0" />
            <span className="truncate">{t(section.labelKey)}</span>
          </button>
        );
      })}
    </>
  );
}

export function SettingsSidebar({ activeSection, onSectionChange, hasProject }: SettingsSidebarProps) {
  const t = useTranslations("settings");
  const appSections = SETTINGS_SECTIONS.filter((s) => s.group === "app");
  const runtimeSections = SETTINGS_SECTIONS.filter((s) => s.group === "runtimes");
  const projectSections = SETTINGS_SECTIONS.filter((s) => s.group === "project");

  return (
    <ScrollArea className="h-full">
      <nav className="flex flex-col gap-0.5 p-3">
        <NavGroup
          label={t("nav.appSettings")}
          sections={appSections}
          activeSection={activeSection}
          onSectionChange={onSectionChange}
        />
        <NavGroup
          label={t("nav.runtimes")}
          sections={runtimeSections}
          activeSection={activeSection}
          onSectionChange={onSectionChange}
        />
        <NavGroup
          label={t("nav.projectSettings")}
          sections={projectSections}
          activeSection={activeSection}
          onSectionChange={onSectionChange}
          disabled={!hasProject}
        />
      </nav>
    </ScrollArea>
  );
}
