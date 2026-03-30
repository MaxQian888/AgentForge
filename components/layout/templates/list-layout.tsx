"use client";

import { cn } from "@/lib/utils";
import { PageHeader, type BreadcrumbEntry } from "@/components/shared/page-header";

interface ListLayoutProps {
  breadcrumbs?: BreadcrumbEntry[];
  title: string;
  description?: string;
  actions?: React.ReactNode;
  toolbar?: React.ReactNode;
  children: React.ReactNode;
  className?: string;
}

export function ListLayout({
  breadcrumbs,
  title,
  description,
  actions,
  toolbar,
  children,
  className,
}: ListLayoutProps) {
  return (
    <div className={cn("flex flex-col gap-2", className)}>
      <PageHeader
        breadcrumbs={breadcrumbs}
        title={title}
        description={description}
        actions={actions}
      />
      {toolbar && <div className="pb-1">{toolbar}</div>}
      <div className="flex-1">{children}</div>
    </div>
  );
}
