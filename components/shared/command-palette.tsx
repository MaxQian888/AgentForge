"use client";

import { useCallback, useEffect } from "react";
import { useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import {
  LayoutDashboard,
  FolderKanban,
  Bot,
  DollarSign,
  Shield,
  Users,
  Network,
  Timer,
  RefreshCw,
  Puzzle,
  Settings,
  ClipboardCheck,
  Brain,
  MessageCircle,
  BookOpenText,
  Plus,
} from "lucide-react";
import type { LucideIcon } from "lucide-react";
import {
  CommandDialog,
  CommandInput,
  CommandList,
  CommandEmpty,
  CommandGroup,
  CommandItem,
  CommandSeparator,
} from "@/components/ui/command";

interface CommandPaletteProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

interface NavItem {
  id: string;
  labelKey: string;
  href: string;
  icon: LucideIcon;
}

interface NavGroup {
  id: string;
  labelKey: string;
  items: NavItem[];
}

const navGroups: NavGroup[] = [
  {
    id: "workspace",
    labelKey: "nav.group.workspace",
    items: [
      { id: "dashboard", labelKey: "nav.dashboard", href: "/", icon: LayoutDashboard },
      { id: "projects", labelKey: "nav.projects", href: "/projects", icon: FolderKanban },
    ],
  },
  {
    id: "project",
    labelKey: "nav.group.project",
    items: [
      { id: "project-dashboard", labelKey: "nav.projectDashboard", href: "/project/dashboard", icon: LayoutDashboard },
      { id: "team", labelKey: "nav.team", href: "/team", icon: Users },
      { id: "agents", labelKey: "nav.agents", href: "/agents", icon: Bot },
      { id: "teams", labelKey: "nav.teams", href: "/teams", icon: Network },
      { id: "sprints", labelKey: "nav.sprints", href: "/sprints", icon: Timer },
      { id: "reviews", labelKey: "nav.reviews", href: "/reviews", icon: ClipboardCheck },
    ],
  },
  {
    id: "operations",
    labelKey: "nav.group.operations",
    items: [
      { id: "cost", labelKey: "nav.cost", href: "/cost", icon: DollarSign },
      { id: "scheduler", labelKey: "nav.scheduler", href: "/scheduler", icon: Timer },
      { id: "workflow", labelKey: "nav.workflow", href: "/workflow", icon: RefreshCw },
      { id: "memory", labelKey: "nav.memory", href: "/memory", icon: Brain },
    ],
  },
  {
    id: "configuration",
    labelKey: "nav.group.configuration",
    items: [
      { id: "roles", labelKey: "nav.roles", href: "/roles", icon: Shield },
      { id: "plugins", labelKey: "nav.plugins", href: "/plugins", icon: Puzzle },
      { id: "settings", labelKey: "nav.settings", href: "/settings", icon: Settings },
      { id: "im", labelKey: "nav.imBridge", href: "/im", icon: MessageCircle },
      { id: "docs", labelKey: "nav.docs", href: "/docs", icon: BookOpenText },
    ],
  },
];

export function CommandPalette({ open, onOpenChange }: CommandPaletteProps) {
  const router = useRouter();
  const t = useTranslations("common");

  const handleKeyDown = useCallback(
    (e: KeyboardEvent) => {
      if (e.key === "k" && (e.metaKey || e.ctrlKey)) {
        e.preventDefault();
        onOpenChange(!open);
      }
    },
    [open, onOpenChange],
  );

  useEffect(() => {
    document.addEventListener("keydown", handleKeyDown);
    return () => document.removeEventListener("keydown", handleKeyDown);
  }, [handleKeyDown]);

  const navigate = (href: string) => {
    onOpenChange(false);
    router.push(href);
  };

  const actions = [
    { id: "create-project", labelKey: "commandPalette.createProject", onSelect: () => navigate("/projects?action=create") },
    { id: "create-task", labelKey: "commandPalette.createTask", onSelect: () => navigate("/project/dashboard?action=create-task") },
    { id: "spawn-agent", labelKey: "commandPalette.spawnAgent", onSelect: () => navigate("/agents?action=spawn") },
    { id: "create-team", labelKey: "commandPalette.createTeam", onSelect: () => navigate("/teams?action=create") },
  ];

  return (
    <CommandDialog
      open={open}
      onOpenChange={onOpenChange}
      title={t("commandPalette.placeholder")}
      description={t("commandPalette.placeholder")}
      showCloseButton={false}
    >
      <CommandInput placeholder={t("commandPalette.placeholder")} />
      <CommandList>
        <CommandEmpty>{t("commandPalette.noResults")}</CommandEmpty>

        {navGroups.map((group, gi) => (
          <div key={group.id}>
            {gi > 0 && <CommandSeparator />}
            <CommandGroup heading={t(group.labelKey)}>
              {group.items.map((item) => {
                const Icon = item.icon;
                return (
                  <CommandItem
                    key={item.id}
                    value={`${group.id} ${item.id} ${t(item.labelKey)}`}
                    onSelect={() => navigate(item.href)}
                  >
                    <Icon />
                    <span>{t(item.labelKey)}</span>
                  </CommandItem>
                );
              })}
            </CommandGroup>
          </div>
        ))}

        <CommandSeparator />
        <CommandGroup heading={t("commandPalette.actions")}>
          {actions.map((action) => (
            <CommandItem
              key={action.id}
              value={`action ${action.id} ${t(action.labelKey)}`}
              onSelect={action.onSelect}
            >
              <Plus />
              <span>{t(action.labelKey)}</span>
            </CommandItem>
          ))}
        </CommandGroup>
      </CommandList>
    </CommandDialog>
  );
}
