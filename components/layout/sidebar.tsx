"use client";

import { useState, useCallback } from "react";
import Link from "next/link";
import { usePathname } from "next/navigation";
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
  ChevronRight,
  Search,
} from "lucide-react";
import type { LucideIcon } from "lucide-react";
import {
  Sidebar,
  SidebarContent,
  SidebarFooter,
  SidebarGroup,
  SidebarGroupContent,
  SidebarGroupLabel,
  SidebarHeader,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
  SidebarRail,
} from "@/components/ui/sidebar";
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from "@/components/ui/collapsible";
import { ThemeToggle } from "@/components/ui/theme-toggle";
import { useLayoutStore } from "@/lib/stores/layout-store";

interface NavItem {
  href: string;
  labelKey: string;
  icon: LucideIcon;
}

interface NavGroup {
  id: string;
  labelKey: string;
  items: NavItem[];
  defaultOpen: boolean;
}

const navGroups: NavGroup[] = [
  {
    id: "workspace",
    labelKey: "nav.group.workspace",
    defaultOpen: true,
    items: [
      { href: "/", labelKey: "nav.dashboard", icon: LayoutDashboard },
      { href: "/projects", labelKey: "nav.projects", icon: FolderKanban },
    ],
  },
  {
    id: "project",
    labelKey: "nav.group.project",
    defaultOpen: true,
    items: [
      { href: "/project/dashboard", labelKey: "nav.projectDashboard", icon: LayoutDashboard },
      { href: "/team", labelKey: "nav.team", icon: Users },
      { href: "/agents", labelKey: "nav.agents", icon: Bot },
      { href: "/teams", labelKey: "nav.teams", icon: Network },
      { href: "/sprints", labelKey: "nav.sprints", icon: Timer },
      { href: "/reviews", labelKey: "nav.reviews", icon: ClipboardCheck },
    ],
  },
  {
    id: "operations",
    labelKey: "nav.group.operations",
    defaultOpen: true,
    items: [
      { href: "/cost", labelKey: "nav.cost", icon: DollarSign },
      { href: "/scheduler", labelKey: "nav.scheduler", icon: RefreshCw },
      { href: "/memory", labelKey: "nav.memory", icon: Brain },
    ],
  },
  {
    id: "configuration",
    labelKey: "nav.group.configuration",
    defaultOpen: false,
    items: [
      { href: "/roles", labelKey: "nav.roles", icon: Shield },
      { href: "/plugins", labelKey: "nav.plugins", icon: Puzzle },
      { href: "/settings", labelKey: "nav.settings", icon: Settings },
      { href: "/im", labelKey: "nav.imBridge", icon: MessageCircle },
      { href: "/docs", labelKey: "nav.docs", icon: BookOpenText },
    ],
  },
];

function getStoredGroupState(groupId: string, defaultOpen: boolean): boolean {
  if (typeof window === "undefined") return defaultOpen;
  const stored = localStorage.getItem(`sidebar-group-${groupId}`);
  return stored !== null ? stored === "true" : defaultOpen;
}

function NavGroupSection({ group }: { group: NavGroup }) {
  const pathname = usePathname();
  const t = useTranslations("common");
  const [open, setOpen] = useState(() =>
    getStoredGroupState(group.id, group.defaultOpen)
  );

  const handleToggle = useCallback(
    (value: boolean) => {
      setOpen(value);
      localStorage.setItem(`sidebar-group-${group.id}`, String(value));
    },
    [group.id]
  );

  return (
    <Collapsible open={open} onOpenChange={handleToggle}>
      <SidebarGroup>
        <SidebarGroupLabel asChild>
          <CollapsibleTrigger className="flex w-full items-center gap-1 [&[data-state=open]>svg]:rotate-90">
            <ChevronRight className="size-3 shrink-0 text-muted-foreground transition-transform duration-200" />
            <span>{t(group.labelKey)}</span>
          </CollapsibleTrigger>
        </SidebarGroupLabel>
        <CollapsibleContent>
          <SidebarGroupContent>
            <SidebarMenu>
              {group.items.map((item) => {
                const Icon = item.icon;
                const active =
                  item.href === "/"
                    ? pathname === "/"
                    : pathname.startsWith(item.href);
                return (
                  <SidebarMenuItem key={item.href}>
                    <SidebarMenuButton
                      asChild
                      isActive={active}
                      tooltip={t(item.labelKey)}
                    >
                      <Link href={item.href}>
                        <Icon />
                        <span>{t(item.labelKey)}</span>
                      </Link>
                    </SidebarMenuButton>
                  </SidebarMenuItem>
                );
              })}
            </SidebarMenu>
          </SidebarGroupContent>
        </CollapsibleContent>
      </SidebarGroup>
    </Collapsible>
  );
}

export function AppSidebar() {
  const t = useTranslations("common");
  const openCommandPalette = useLayoutStore((s) => s.openCommandPalette);

  return (
    <Sidebar collapsible="icon">
      <SidebarHeader>
        <SidebarMenu>
          <SidebarMenuItem>
            <SidebarMenuButton size="lg" asChild tooltip={t("appName")}>
              <Link href="/">
                <div className="flex aspect-square size-8 items-center justify-center rounded-lg bg-sidebar-primary text-sidebar-primary-foreground">
                  <Bot className="size-4" />
                </div>
                <span className="font-semibold">{t("appName")}</span>
              </Link>
            </SidebarMenuButton>
          </SidebarMenuItem>
          <SidebarMenuItem>
            <SidebarMenuButton
              tooltip={t("quickSearch")}
              onClick={openCommandPalette}
              className="text-muted-foreground"
            >
              <Search />
              <span className="flex-1 text-sm">{t("quickSearch")}</span>
              <kbd className="pointer-events-none hidden rounded border bg-muted px-1.5 py-0.5 text-[10px] font-medium text-muted-foreground group-data-[collapsible=icon]:hidden sm:inline-block">
                ⌘K
              </kbd>
            </SidebarMenuButton>
          </SidebarMenuItem>
        </SidebarMenu>
      </SidebarHeader>

      <SidebarContent>
        {navGroups.map((group) => (
          <NavGroupSection key={group.id} group={group} />
        ))}
      </SidebarContent>

      <SidebarFooter>
        <SidebarMenu>
          <SidebarMenuItem>
            <div className="group-data-[collapsible=icon]:hidden px-2 py-1">
              <ThemeToggle />
            </div>
          </SidebarMenuItem>
        </SidebarMenu>
      </SidebarFooter>

      <SidebarRail />
    </Sidebar>
  );
}
