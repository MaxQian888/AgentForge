"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { useTranslations } from "next-intl";
import {
  ArrowLeft,
  History,
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
  SunMoon,
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
import {
  filterPaletteCommands,
  type PaletteCommandCategory,
  type PaletteCommandKind,
  type PaletteSearchCommand,
} from "@/lib/command-palette";
import { useLayoutStore } from "@/lib/stores/layout-store";
import { useTheme } from "@/lib/theme/provider";

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

interface PaletteCommand {
  id: PaletteSearchCommand["id"];
  label: PaletteSearchCommand["label"];
  value: PaletteSearchCommand["value"];
  href: PaletteSearchCommand["href"];
  icon: LucideIcon;
  kind: PaletteCommandKind;
  category: PaletteCommandCategory;
  keywords?: string[];
  onSelect?: () => void;
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
  const pathname = usePathname();
  const router = useRouter();
  const searchParams = useSearchParams();
  const t = useTranslations("common");
  const { resolvedTheme, setTheme } = useTheme();
  const recentCommands = useLayoutStore((state) => state.recentCommands);
  const recordCommand = useLayoutStore((state) => state.recordCommand);
  const [query, setQuery] = useState("");

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

  const executeCommand = (command: PaletteCommand) => {
    recordCommand({
      id: command.id,
      label: command.label,
      href: command.href,
      kind: command.kind,
    });
    onOpenChange(false);
    if (command.onSelect) {
      command.onSelect();
      return;
    }
    router.push(command.href);
  };

  const navigationGroups = useMemo(
    () =>
      navGroups.map((group) => ({
        id: group.id,
        heading: t(group.labelKey),
        commands: group.items.map<PaletteCommand>((item) => ({
          id: item.id,
          label: t(item.labelKey),
          value: `${group.id} ${item.id} ${t(item.labelKey)}`,
          href: item.href,
          icon: item.icon,
          kind: "navigation",
          category: "navigation",
          keywords: [group.id, item.id],
        })),
      })),
    [t],
  );

  const navigationCommands = useMemo(
    () => navigationGroups.flatMap((group) => group.commands),
    [navigationGroups],
  );

  const actionCommands = useMemo<PaletteCommand[]>(
    () => [
      {
        id: "create-project",
        label: t("commandPalette.createProject"),
        value: `action create-project ${t("commandPalette.createProject")}`,
        href: "/projects?action=create",
        icon: Plus,
        kind: "action",
        category: "actions",
        keywords: ["new project", "create project"],
      },
      {
        id: "create-task",
        label: t("commandPalette.createTask"),
        value: `action create-task ${t("commandPalette.createTask")}`,
        href: "/project/dashboard?action=create-task",
        icon: Plus,
        kind: "action",
        category: "actions",
        keywords: ["new task", "task"],
      },
      {
        id: "spawn-agent",
        label: t("commandPalette.spawnAgent"),
        value: `action spawn-agent ${t("commandPalette.spawnAgent")}`,
        href: "/agents?action=spawn",
        icon: Plus,
        kind: "action",
        category: "actions",
        keywords: ["new agent", "agent runtime"],
      },
      {
        id: "create-team",
        label: t("commandPalette.createTeam"),
        value: `action create-team ${t("commandPalette.createTeam")}`,
        href: "/teams?action=create",
        icon: Plus,
        kind: "action",
        category: "actions",
        keywords: ["new team", "team"],
      },
    ],
    [t],
  );

  const contextualCommands = useMemo(() => {
    const commands: PaletteCommand[] = [];

    if (pathname === "/settings") {
      commands.push({
        id: "toggle-theme",
        label: t("commandPalette.toggleTheme"),
        value: `context toggle-theme ${t("commandPalette.toggleTheme")}`,
        href: pathname,
        icon: SunMoon,
        kind: "action",
        category: "context",
        keywords: ["theme", "appearance", "dark", "light", "settings"],
        onSelect: () => setTheme(resolvedTheme === "dark" ? "light" : "dark"),
      });
    }

    if (pathname === "/agents" && searchParams.get("agent")) {
      commands.push({
        id: "clear-agent-selection",
        label: t("commandPalette.clearAgentSelection"),
        value: `context clear-agent-selection ${t("commandPalette.clearAgentSelection")}`,
        href: "/agents",
        icon: ArrowLeft,
        kind: "navigation",
        category: "context",
        keywords: ["agent detail", "clear selection", "back to agents"],
      });
    }

    if (pathname === "/teams" && searchParams.get("project")) {
      commands.push({
        id: "clear-team-project-filter",
        label: t("commandPalette.clearTeamProjectFilter"),
        value: `context clear-team-project-filter ${t("commandPalette.clearTeamProjectFilter")}`,
        href: "/teams",
        icon: ArrowLeft,
        kind: "navigation",
        category: "context",
        keywords: ["teams", "project filter", "all projects"],
      });
    }

    if (pathname === "/project/dashboard" && searchParams.get("dashboard")) {
      commands.push({
        id: "clear-dashboard-selection",
        label: t("commandPalette.clearDashboardSelection"),
        value: `context clear-dashboard-selection ${t("commandPalette.clearDashboardSelection")}`,
        href: "/project/dashboard",
        icon: ArrowLeft,
        kind: "navigation",
        category: "context",
        keywords: ["dashboard", "selection", "reset dashboard"],
      });
    }

    return commands;
  }, [pathname, resolvedTheme, searchParams, setTheme, t]);

  const commandMap = useMemo(
    () =>
      new Map(
        [...navigationCommands, ...actionCommands, ...contextualCommands].map((command) => [
          command.id,
          command,
        ]),
      ),
    [actionCommands, contextualCommands, navigationCommands],
  );

  const effectiveQuery = query.trim().startsWith(">")
    ? query.trim().slice(1).trim()
    : query;
  const actionMode = query.trim().startsWith(">");

  const filteredNavigationGroups = useMemo(
    () =>
      (actionMode ? [] : navigationGroups)
        .map((group) => ({
          ...group,
          commands: filterPaletteCommands(group.commands, effectiveQuery),
        }))
        .filter((group) => group.commands.length > 0),
    [actionMode, effectiveQuery, navigationGroups],
  );

  const filteredActionCommands = useMemo(
    () => filterPaletteCommands(actionCommands, effectiveQuery),
    [actionCommands, effectiveQuery],
  );

  const filteredContextualCommands = useMemo(
    () => filterPaletteCommands(contextualCommands, effectiveQuery),
    [contextualCommands, effectiveQuery],
  );

  const showHistory = query.trim().length === 0;
  const recentItems = showHistory
    ? recentCommands.filter((item) => item.kind === "navigation").slice(0, 5)
    : [];
  const recentCommandHistory = showHistory
    ? recentCommands.filter((item) => item.kind === "action")
    : [];

  const hasResults =
    recentItems.length > 0 ||
    recentCommandHistory.length > 0 ||
    filteredContextualCommands.length > 0 ||
    filteredNavigationGroups.length > 0 ||
    filteredActionCommands.length > 0;

  const handleRecentSelection = (recentId: string) => {
    const mapped = commandMap.get(recentId);

    if (mapped) {
      executeCommand(mapped);
      return;
    }

    const fallback = recentCommands.find((item) => item.id === recentId);
    if (!fallback) {
      return;
    }

    onOpenChange(false);
    router.push(fallback.href);
  };

  return (
    <CommandDialog
      open={open}
      onOpenChange={onOpenChange}
      title={t("commandPalette.placeholder")}
      description={t("commandPalette.placeholder")}
      commandProps={{ shouldFilter: false }}
      showCloseButton={false}
    >
      <CommandInput
        placeholder={t("commandPalette.placeholder")}
        value={query}
        onValueChange={setQuery}
      />
      <CommandList>
        {!hasResults ? <CommandEmpty>{t("commandPalette.noResults")}</CommandEmpty> : null}

        {recentItems.length > 0 ? (
          <>
            <CommandGroup heading={t("commandPalette.recent")}>
              {recentItems.map((recent) => {
                const command = commandMap.get(recent.id);
                const Icon = command?.icon ?? History;
                const value = command?.value ?? `recent ${recent.id} ${recent.label}`;

                return (
                  <CommandItem
                    key={`recent-${recent.id}`}
                    value={value}
                    onSelect={() => handleRecentSelection(recent.id)}
                  >
                    <Icon />
                    <span>{recent.label}</span>
                  </CommandItem>
                );
              })}
            </CommandGroup>
            <CommandSeparator />
          </>
        ) : null}

        {recentCommandHistory.length > 0 ? (
          <>
            <CommandGroup heading={t("commandPalette.history")}>
              {recentCommandHistory.map((recent) => {
                const command = commandMap.get(recent.id);
                const Icon = command?.icon ?? History;

                return (
                  <CommandItem
                    key={`history-${recent.id}`}
                    value={command?.value ?? `history ${recent.id} ${recent.label}`}
                    onSelect={() => handleRecentSelection(recent.id)}
                  >
                    <Icon />
                    <span>{recent.label}</span>
                  </CommandItem>
                );
              })}
            </CommandGroup>
            <CommandSeparator />
          </>
        ) : null}

        {filteredContextualCommands.length > 0 ? (
          <>
            <CommandGroup heading={t("commandPalette.context")}>
              {filteredContextualCommands.map((command) => {
                const Icon = command.icon;

                return (
                  <CommandItem
                    key={command.id}
                    value={command.value}
                    onSelect={() => executeCommand(command)}
                  >
                    <Icon />
                    <span>{command.label}</span>
                  </CommandItem>
                );
              })}
            </CommandGroup>
            <CommandSeparator />
          </>
        ) : null}

        {filteredNavigationGroups.map((group, gi) => (
          <div key={group.id}>
            {gi > 0 && <CommandSeparator />}
            <CommandGroup heading={group.heading}>
              {group.commands.map((command) => {
                const Icon = command.icon;
                return (
                  <CommandItem
                    key={command.id}
                    value={command.value}
                    onSelect={() => executeCommand(command)}
                  >
                    <Icon />
                    <span>{command.label}</span>
                  </CommandItem>
                );
              })}
            </CommandGroup>
          </div>
        ))}

        {filteredActionCommands.length > 0 ? <CommandSeparator /> : null}
        <CommandGroup heading={t("commandPalette.actions")}>
          {filteredActionCommands.map((action) => {
            const Icon = action.icon;

            return (
              <CommandItem
                key={action.id}
                value={action.value}
                onSelect={() => executeCommand(action)}
              >
                <Icon />
                <span>{action.label}</span>
              </CommandItem>
            );
          })}
        </CommandGroup>
      </CommandList>
    </CommandDialog>
  );
}
