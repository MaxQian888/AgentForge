"use client";

import { cn } from "@/lib/utils";

interface SectionCardProps {
  title?: React.ReactNode;
  description?: React.ReactNode;
  actions?: React.ReactNode;
  footer?: React.ReactNode;
  children?: React.ReactNode;
  className?: string;
  bodyClassName?: string;
  headerClassName?: string;
  as?: "section" | "div" | "article";
}

export function SectionCard({
  title,
  description,
  actions,
  footer,
  children,
  className,
  bodyClassName,
  headerClassName,
  as: Tag = "section",
}: SectionCardProps) {
  const hasHeader = Boolean(title || description || actions);

  return (
    <Tag
      className={cn(
        "flex flex-col rounded-lg border bg-card text-card-foreground shadow-sm",
        className,
      )}
    >
      {hasHeader && (
        <header
          className={cn(
            "flex flex-col gap-[var(--space-stack-xs)] border-b p-[var(--space-card-padding)] sm:flex-row sm:items-start sm:justify-between sm:gap-[var(--space-grid-gap)]",
            headerClassName,
          )}
        >
          <div className="min-w-0 flex-1">
            {title && (
              <h2 className="text-base font-semibold leading-tight tracking-tight">
                {title}
              </h2>
            )}
            {description && (
              <p className="mt-[var(--space-stack-xs)] text-sm text-muted-foreground">
                {description}
              </p>
            )}
          </div>
          {actions && (
            <div className="flex shrink-0 flex-wrap items-center gap-[var(--space-stack-sm)]">
              {actions}
            </div>
          )}
        </header>
      )}
      {children !== undefined && children !== null && (
        <div
          className={cn(
            "flex flex-1 flex-col gap-[var(--space-stack-md)] p-[var(--space-card-padding)]",
            bodyClassName,
          )}
        >
          {children}
        </div>
      )}
      {footer && (
        <footer className="border-t p-[var(--space-card-padding)]">
          {footer}
        </footer>
      )}
    </Tag>
  );
}
