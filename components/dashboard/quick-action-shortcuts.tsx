"use client";

import { useEffect } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import type { LucideIcon } from "lucide-react";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";

export interface QuickActionShortcutItem {
  id: string;
  label: string;
  href: string;
  icon: LucideIcon;
  shortcut: string;
  onTrigger?: () => void;
  variant?: "default" | "outline" | "ghost";
}

function shouldIgnoreShortcutTarget(target: EventTarget | null) {
  if (!(target instanceof HTMLElement)) {
    return false;
  }

  const tagName = target.tagName.toLowerCase();
  return (
    tagName === "input" ||
    tagName === "textarea" ||
    tagName === "select" ||
    target.isContentEditable
  );
}

export function QuickActionShortcuts({
  actions,
  className,
}: {
  actions: QuickActionShortcutItem[];
  className?: string;
}) {
  const router = useRouter();

  useEffect(() => {
    const handleKeyDown = (event: KeyboardEvent) => {
      if (
        event.defaultPrevented ||
        event.metaKey ||
        event.ctrlKey ||
        event.altKey ||
        event.repeat ||
        shouldIgnoreShortcutTarget(document.activeElement ?? event.target)
      ) {
        return;
      }

      const action = actions.find(
        (candidate) =>
          candidate.shortcut.toLowerCase() === event.key.toLowerCase(),
      );

      if (!action) {
        return;
      }

      event.preventDefault();

      if (action.onTrigger) {
        action.onTrigger();
        return;
      }

      router.push(action.href);
    };

    document.addEventListener("keydown", handleKeyDown);
    return () => document.removeEventListener("keydown", handleKeyDown);
  }, [actions, router]);

  return (
    <div className={cn("flex flex-wrap gap-3", className)}>
      {actions.map((action) => {
        const Icon = action.icon;

        return (
          <Button
            key={action.id}
            asChild
            variant={action.variant ?? "ghost"}
            size="sm"
          >
            <Link
              href={action.href}
              onClick={(event) => {
                if (!action.onTrigger) {
                  return;
                }

                event.preventDefault();
                action.onTrigger();
              }}
            >
              <Icon className="mr-1.5 size-4" />
              <span>{action.label}</span>
              <kbd className="ml-2 rounded border bg-muted px-1 py-0.5 text-[10px] font-medium text-muted-foreground">
                {action.shortcut.toUpperCase()}
              </kbd>
            </Link>
          </Button>
        );
      })}
    </div>
  );
}
