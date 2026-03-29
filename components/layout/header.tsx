"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import { Bell, LogOut, User } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Avatar, AvatarFallback } from "@/components/ui/avatar";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover";
import { ScrollArea } from "@/components/ui/scroll-area";
import { useAuthStore } from "@/lib/stores/auth-store";
import { useNotificationStore } from "@/lib/stores/notification-store";
import { MobileSidebar } from "./sidebar";
import { LanguageSwitcher } from "./language-switcher";

export function Header() {
  const router = useRouter();
  const t = useTranslations("common");
  const [notifOpen, setNotifOpen] = useState(false);
  const { user, logout } = useAuthStore();
  const { notifications, unreadCount, markRead, markAllRead } = useNotificationStore();

  const initials = user?.name
    ? user.name
        .split(" ")
        .map((w) => w[0])
        .join("")
        .toUpperCase()
        .slice(0, 2)
    : "U";

  return (
    <header className="flex h-14 items-center gap-4 border-b bg-background px-4">
      <MobileSidebar />
      <div className="flex-1" />

      <Popover open={notifOpen} onOpenChange={setNotifOpen}>
        <PopoverTrigger asChild>
          <Button variant="ghost" size="icon-sm" className="relative">
            <Bell className="size-4" />
            {unreadCount > 0 && (
              <Badge className="absolute -right-1 -top-1 h-4 min-w-4 px-1 text-[10px]">
                {unreadCount}
              </Badge>
            )}
          </Button>
        </PopoverTrigger>
        <PopoverContent className="w-80 p-0" align="end">
          <div className="flex items-center justify-between border-b p-3">
            <span className="font-medium">{t("header.notifications")}</span>
            {unreadCount > 0 && (
              <Button
                variant="ghost"
                size="sm"
                className="h-auto px-2 py-1 text-xs"
                onClick={() => markAllRead()}
              >
                {t("header.markAllRead")}
              </Button>
            )}
          </div>
          <ScrollArea className="h-64">
            {notifications.length === 0 ? (
              <p className="p-4 text-center text-sm text-muted-foreground">
                {t("header.noNotifications")}
              </p>
            ) : (
              notifications.map((n) => (
                <button
                  key={n.id}
                  onClick={() => {
                    markRead(n.id);
                    if (n.href) {
                      setNotifOpen(false);
                      router.push(n.href);
                    }
                  }}
                  className="flex w-full flex-col gap-1 border-b p-3 text-left hover:bg-accent"
                >
                  <div className="flex items-center gap-2">
                    {!n.read && (
                      <span className="size-2 shrink-0 rounded-full bg-primary" />
                    )}
                    <span className="text-sm font-medium">{n.title}</span>
                  </div>
                  <span className="text-xs text-muted-foreground">
                    {n.message}
                  </span>
                </button>
              ))
            )}
          </ScrollArea>
        </PopoverContent>
      </Popover>

      <LanguageSwitcher />

      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button variant="ghost" size="icon-sm">
            <Avatar className="size-7">
              <AvatarFallback className="text-xs">{initials}</AvatarFallback>
            </Avatar>
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end">
          <DropdownMenuItem disabled>
            <User className="mr-2 size-4" />
            {user?.name ?? t("header.user")}
          </DropdownMenuItem>
          <DropdownMenuItem
            onClick={() => {
              void logout().catch(() => undefined);
            }}
          >
            <LogOut className="mr-2 size-4" />
            {t("header.logout")}
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
    </header>
  );
}
