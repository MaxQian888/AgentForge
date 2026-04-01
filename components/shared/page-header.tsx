"use client";

import Link from "next/link";
import { cn } from "@/lib/utils";
import {
  Breadcrumb,
  BreadcrumbItem,
  BreadcrumbLink,
  BreadcrumbList,
  BreadcrumbPage,
  BreadcrumbSeparator,
} from "@/components/ui/breadcrumb";

export interface BreadcrumbEntry {
  label: string;
  href?: string;
}

interface PageHeaderProps {
  breadcrumbs?: BreadcrumbEntry[];
  title: string;
  description?: string;
  actions?: React.ReactNode;
  sticky?: boolean;
  className?: string;
}

export function PageHeader({
  breadcrumbs,
  title,
  description,
  actions,
  sticky,
  className,
}: PageHeaderProps) {
  return (
    <div
      className={cn(
        "flex flex-col gap-[var(--space-stack-xs)] pb-[var(--space-section-gap)]",
        sticky &&
          "sticky top-0 z-10 -mx-[var(--space-page-inline)] -mt-[var(--space-page-inline)] mb-[var(--space-stack-sm)] border-b bg-background/80 px-[var(--space-page-inline)] py-[var(--space-stack-sm)] backdrop-blur-sm",
        className
      )}
    >
      {breadcrumbs && breadcrumbs.length > 0 && (
        <Breadcrumb className="animate-fade-in">
          <BreadcrumbList className="text-fluid-caption">
            {breadcrumbs.map((crumb, i) => {
              const isLast = i === breadcrumbs.length - 1;
              return (
                <span key={i} className="contents">
                  {i > 0 && <BreadcrumbSeparator />}
                  <BreadcrumbItem>
                    {isLast || !crumb.href ? (
                      <BreadcrumbPage>{crumb.label}</BreadcrumbPage>
                    ) : (
                      <BreadcrumbLink asChild>
                        <Link href={crumb.href}>{crumb.label}</Link>
                      </BreadcrumbLink>
                    )}
                  </BreadcrumbItem>
                </span>
              );
            })}
          </BreadcrumbList>
        </Breadcrumb>
      )}
      <div className="flex items-center justify-between gap-[var(--space-grid-gap)]">
        <div className="min-w-0">
          <h1 className="text-fluid-title truncate font-semibold tracking-tight">
            {title}
          </h1>
          {description && (
            <p className="mt-[var(--space-stack-xs)] text-fluid-body text-muted-foreground">
              {description}
            </p>
          )}
        </div>
        {actions && (
          <div className="flex shrink-0 items-center gap-[var(--space-stack-sm)]">
            {actions}
          </div>
        )}
      </div>
    </div>
  );
}
