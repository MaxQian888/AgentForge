"use client";

import { cn } from "@/lib/utils";
import { PageHeader, type BreadcrumbEntry } from "@/components/shared/page-header";

interface SettingsLayoutProps {
  breadcrumbs?: BreadcrumbEntry[];
  title: string;
  description?: string;
  actions?: React.ReactNode;
  dirty?: boolean;
  saveBar?: React.ReactNode;
  children: React.ReactNode;
  className?: string;
}

export function SettingsLayout({
  breadcrumbs,
  title,
  description,
  actions,
  dirty,
  saveBar,
  children,
  className,
}: SettingsLayoutProps) {
  return (
    <div className={cn("flex flex-col", className)}>
      <PageHeader
        breadcrumbs={breadcrumbs}
        title={title}
        description={description}
        actions={actions}
        sticky
      />
      <div className="mx-auto w-full max-w-3xl space-y-6 pb-24">{children}</div>
      {dirty && saveBar && (
        <div className="fixed inset-x-0 bottom-0 z-20 border-t bg-background/95 px-6 py-3 backdrop-blur-sm">
          <div className="mx-auto flex max-w-3xl items-center justify-end gap-2">
            {saveBar}
          </div>
        </div>
      )}
    </div>
  );
}
