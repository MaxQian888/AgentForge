"use client";

interface PluginDetailSectionProps {
  title: string;
  children: React.ReactNode;
  action?: React.ReactNode;
}

export function PluginDetailSection({
  title,
  children,
  action,
}: PluginDetailSectionProps) {
  return (
    <div className="rounded-lg border border-border/60 p-3">
      <div className="flex items-center justify-between">
        <p className="font-medium text-sm">{title}</p>
        {action}
      </div>
      <div className="mt-2 text-sm text-muted-foreground">{children}</div>
    </div>
  );
}
