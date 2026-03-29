"use client";

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
  Moon,
  Sun,
  Menu,
  Timer,
  RefreshCw,
  Puzzle,
  Settings,
  ClipboardCheck,
  Brain,
  MessageCircle,
  BookOpenText,
} from "lucide-react";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
  SheetTrigger,
} from "@/components/ui/sheet";
import { Separator } from "@/components/ui/separator";
import { useCallback, useState } from "react";
import type { LucideIcon } from "lucide-react";

interface NavItem {
  href: string;
  labelKey: string;
  icon: LucideIcon;
}

const navItems: NavItem[] = [
  { href: "/", labelKey: "nav.dashboard", icon: LayoutDashboard },
  { href: "/projects", labelKey: "nav.projects", icon: FolderKanban },
  { href: "/project/dashboard", labelKey: "nav.projectDashboard", icon: LayoutDashboard },
  { href: "/team", labelKey: "nav.team", icon: Users },
  { href: "/agents", labelKey: "nav.agents", icon: Bot },
  { href: "/teams", labelKey: "nav.teams", icon: Network },
  { href: "/sprints", labelKey: "nav.sprints", icon: Timer },
  { href: "/reviews", labelKey: "nav.reviews", icon: ClipboardCheck },
  { href: "/scheduler", labelKey: "nav.scheduler", icon: RefreshCw },
  { href: "/cost", labelKey: "nav.cost", icon: DollarSign },
  { href: "/memory", labelKey: "nav.memory", icon: Brain },
  { href: "/docs", labelKey: "nav.docs", icon: BookOpenText },
  { href: "/im", labelKey: "nav.imBridge", icon: MessageCircle },
  { href: "/roles", labelKey: "nav.roles", icon: Shield },
  { href: "/plugins", labelKey: "nav.plugins", icon: Puzzle },
  { href: "/settings", labelKey: "nav.settings", icon: Settings },
];

function NavLinks({ onClick }: { onClick?: () => void }) {
  const pathname = usePathname();
  const t = useTranslations("common");

  return (
    <nav className="flex flex-col gap-1 px-3">
      {navItems.map((item) => {
        const Icon = item.icon;
        const active =
          item.href === "/"
            ? pathname === "/"
            : pathname.startsWith(item.href);
        return (
          <Link
            key={item.href}
            href={item.href}
            onClick={onClick}
            className={cn(
              "flex items-center gap-3 rounded-md px-3 py-2 text-sm font-medium transition-colors",
              active
                ? "bg-accent text-accent-foreground"
                : "text-muted-foreground hover:bg-accent hover:text-accent-foreground"
            )}
          >
            <Icon className="size-4" />
            {t(item.labelKey)}
          </Link>
        );
      })}
    </nav>
  );
}

function ThemeToggle() {
  const [dark, setDark] = useState(
    () =>
      typeof document !== "undefined" &&
      document.documentElement.classList.contains("dark")
  );

  const toggle = useCallback(() => {
    document.documentElement.classList.toggle("dark");
    setDark((d) => !d);
  }, []);

  return (
    <Button variant="ghost" size="icon-sm" onClick={toggle}>
      {dark ? <Sun className="size-4" /> : <Moon className="size-4" />}
    </Button>
  );
}

export function Sidebar() {
  const t = useTranslations("common");

  return (
    <aside className="hidden w-56 shrink-0 border-r bg-sidebar md:flex md:flex-col">
      <div className="flex h-14 items-center px-4 font-semibold">
        {t("appName")}
      </div>
      <Separator />
      <div className="flex-1 py-4">
        <NavLinks />
      </div>
      <div className="border-t p-3">
        <ThemeToggle />
      </div>
    </aside>
  );
}

export function MobileSidebar() {
  const t = useTranslations("common");
  const [open, setOpen] = useState(false);

  return (
    <Sheet open={open} onOpenChange={setOpen}>
      <SheetTrigger asChild>
        <Button variant="ghost" size="icon-sm" className="md:hidden">
          <Menu className="size-5" />
        </Button>
      </SheetTrigger>
      <SheetContent side="left" className="w-56 p-0">
        <SheetHeader className="sr-only">
          <SheetTitle>{t("mobileSidebar.title")}</SheetTitle>
          <SheetDescription>
            {t("mobileSidebar.description")}
          </SheetDescription>
        </SheetHeader>
        <div className="flex h-14 items-center px-4 font-semibold">
          {t("appName")}
        </div>
        <Separator />
        <div className="py-4">
          <NavLinks onClick={() => setOpen(false)} />
        </div>
        <div className="border-t p-3">
          <ThemeToggle />
        </div>
      </SheetContent>
    </Sheet>
  );
}
