"use client";

import { cn } from "@/lib/utils";

interface WorkspaceLayoutProps {
  sidebar?: React.ReactNode;
  rail?: React.ReactNode;
  sidebarWidth?: string;
  railWidth?: string;
  flush?: boolean;
  children: React.ReactNode;
  className?: string;
}

export function WorkspaceLayout({
  sidebar,
  rail,
  sidebarWidth = "260px",
  railWidth = "320px",
  flush,
  children,
  className,
}: WorkspaceLayoutProps) {
  const hasSidebar = !!sidebar;
  const hasRail = !!rail;

  return (
    <div
      className={cn(
        "h-full w-full",
        flush && "-m-6 h-[calc(100vh-var(--header-height))]",
        className
      )}
      style={
        (hasSidebar || hasRail)
          ? {
              display: "grid",
              gridTemplateColumns: hasSidebar && hasRail
                ? `${sidebarWidth} minmax(0,1fr) ${railWidth}`
                : hasSidebar
                  ? `${sidebarWidth} minmax(0,1fr)`
                  : `minmax(0,1fr) ${railWidth}`,
            }
          : undefined
      }
    >
      {hasSidebar && (
        <aside className="h-full overflow-y-auto border-r bg-background">
          {sidebar}
        </aside>
      )}
      <main className="h-full overflow-y-auto">{children}</main>
      {hasRail && (
        <aside className="h-full overflow-y-auto border-l bg-background">
          {rail}
        </aside>
      )}
    </div>
  );
}
