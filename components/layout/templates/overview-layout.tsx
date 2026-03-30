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
    <div className={cn("flex flex-col gap-4", className)}>
      <PageHeader
        breadcrumbs={breadcrumbs}
        title={title}
        description={description}
        actions={actions}
      />
      {metrics && (
        <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-5">
          {metrics}
        </div>
      )}
      <div className="grid gap-4 lg:grid-cols-2">{children}</div>
    </div>
  );
}
