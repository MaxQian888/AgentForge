"use client";

import * as React from "react";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { cn } from "@/lib/utils";

export interface ResponsiveTabItem {
  value: string;
  label: React.ReactNode;
  /** Optional short label used inside the mobile Select fallback. */
  shortLabel?: string;
  icon?: React.ReactNode;
  content?: React.ReactNode;
  disabled?: boolean;
}

interface ResponsiveTabsProps {
  value: string;
  onValueChange: (value: string) => void;
  items: ResponsiveTabItem[];
  /** Breakpoint at which the tab bar switches to a Select. Defaults to `md`. */
  collapseAt?: "sm" | "md" | "lg";
  ariaLabel?: string;
  className?: string;
  listClassName?: string;
  contentClassName?: string;
  children?: React.ReactNode;
}

const collapseClasses = {
  sm: { hidden: "hidden sm:block", visible: "sm:hidden" },
  md: { hidden: "hidden md:block", visible: "md:hidden" },
  lg: { hidden: "hidden lg:block", visible: "lg:hidden" },
} as const;

export function ResponsiveTabs({
  value,
  onValueChange,
  items,
  collapseAt = "md",
  ariaLabel,
  className,
  listClassName,
  contentClassName,
  children,
}: ResponsiveTabsProps) {
  const collapse = collapseClasses[collapseAt];
  const selected = items.find((item) => item.value === value);

  return (
    <Tabs
      value={value}
      onValueChange={onValueChange}
      className={cn("w-full", className)}
    >
      {/* Desktop / tablet tab bar */}
      <div className={collapse.hidden}>
        <TabsList className={cn("w-full justify-start", listClassName)}>
          {items.map((item) => (
            <TabsTrigger
              key={item.value}
              value={item.value}
              disabled={item.disabled}
              className="gap-1.5"
            >
              {item.icon}
              {item.label}
            </TabsTrigger>
          ))}
        </TabsList>
      </div>

      {/* Mobile fallback — Select */}
      <div className={cn("w-full", collapse.visible)}>
        <Select value={value} onValueChange={onValueChange}>
          <SelectTrigger
            aria-label={ariaLabel ?? "Select section"}
            className="h-10 w-full"
          >
            <SelectValue
              placeholder={selected?.shortLabel ?? (selected?.label as string) ?? "Select"}
            />
          </SelectTrigger>
          <SelectContent>
            {items.map((item) => (
              <SelectItem
                key={item.value}
                value={item.value}
                disabled={item.disabled}
              >
                {item.shortLabel ?? item.label}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>

      {/* Render content panels when items include inline content. Otherwise
          consumers can place their own <TabsContent> children. */}
      {items.some((item) => item.content !== undefined)
        ? items.map((item) => (
            <TabsContent
              key={item.value}
              value={item.value}
              className={cn(
                "mt-[var(--space-stack-md)] outline-none",
                contentClassName,
              )}
            >
              {item.content}
            </TabsContent>
          ))
        : children}
    </Tabs>
  );
}
