"use client";

import { cn } from "@/lib/utils";
import { PageHeader, type BreadcrumbEntry } from "@/components/shared/page-header";

interface OverviewLayoutProps {
  breadcrumbs?: BreadcrumbEntry[];
  title: string;
  description?: string;
  actions?: React.ReactNode;
  metrics?: React.ReactNode;
  children: React.ReactNode;
  className?: string;
}

export function OverviewLayout({
  breadcrumbs,
  title,
  description,
  actions,
  metrics,
  children,
  className,
}: OverviewLayoutProps) {
  return (
    <div className={cn("flex flex-col gap-[var(--space-section-gap)]", className)}>
      <PageHeader
        breadcrumbs={breadcrumbs}
        title={title}
        description={description}
        actions={actions}
      />
      {metrics && (
        <div className="grid grid-cols-2 gap-[var(--space-grid-gap)] sm:grid-cols-3 lg:grid-cols-5">
          {metrics}
        </div>
      )}
      <div className="grid gap-[var(--space-grid-gap)] lg:grid-cols-2">
        {children}
      </div>
    </div>
  );
}
