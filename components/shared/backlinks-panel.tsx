"use client";

import Link from "next/link";
import { Badge } from "@/components/ui/badge";
import { buildDocsHref } from "@/lib/route-hrefs";

export interface BacklinkItem {
  linkId: string;
  entityId: string;
  entityType: string;
  title: string;
}

export function BacklinksPanel({
  items,
}: {
  items: BacklinkItem[];
}) {
  return (
    <div className="rounded-xl border border-border/60 bg-card/70 p-4">
      <div>
        <h2 className="text-base font-semibold">Backlinks</h2>
        <p className="text-sm text-muted-foreground">
          References pointing to this item through mention links.
        </p>
      </div>
      <div className="mt-3 space-y-2">
        {items.map((item) => (
          <Link
            key={item.linkId}
            href={item.entityType === "task" ? `/project?taskId=${item.entityId}` : buildDocsHref(item.entityId)}
            className="flex items-center justify-between rounded-lg border border-border/60 bg-background px-3 py-2 hover:bg-accent/40"
          >
            <span className="font-medium">{item.title}</span>
            <Badge variant="outline">{item.entityType}</Badge>
          </Link>
        ))}
        {items.length === 0 ? (
          <div className="rounded-lg border border-dashed border-border/60 px-3 py-4 text-sm text-muted-foreground">
            No backlinks yet.
          </div>
        ) : null}
      </div>
    </div>
  );
}
