"use client";

import Link from "next/link";
import { useTranslations } from "next-intl";
import { ExternalLink } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { buildDocsHref } from "@/lib/route-hrefs";

export interface LinkedDocItem {
  id: string;
  pageId: string;
  title: string;
  linkType: string;
  updatedAt: string;
  preview?: string;
}

export function LinkedDocsPanel({
  projectId,
  taskId,
  docs,
  onAddLink,
  onRemoveLink,
}: {
  projectId: string;
  taskId: string;
  docs: LinkedDocItem[];
  onAddLink?: () => void;
  onRemoveLink?: (linkId: string) => void;
}) {
  const t = useTranslations("tasks");
  const grouped = docs.reduce<Record<string, LinkedDocItem[]>>((acc, doc) => {
    acc[doc.linkType] = acc[doc.linkType] ?? [];
    acc[doc.linkType].push(doc);
    return acc;
  }, {});

  return (
    <div className="rounded-lg border border-border/60 bg-muted/20 p-3 text-sm">
      <div className="flex items-center justify-between gap-2">
        <div>
          <div className="font-medium">{t("detail.relatedDocs")}</div>
          <div className="text-muted-foreground">
            {t("detail.relatedDocsDescription")}
          </div>
        </div>
        <Button type="button" size="sm" variant="outline" onClick={onAddLink}>
          {t("detail.addDoc")}
        </Button>
      </div>

      <div className="mt-3 space-y-3">
        {Object.entries(grouped).map(([linkType, items]) => (
          <div key={linkType} className="space-y-2">
            <div className="flex items-center gap-2">
              <Badge variant="outline">{linkType}</Badge>
              <span className="text-xs text-muted-foreground">{t("detail.linked", { count: items.length })}</span>
            </div>
            {items.map((doc) => (
              <div
                key={doc.id}
                className="rounded-md border border-border/60 bg-background px-3 py-2"
              >
                <div className="flex items-start justify-between gap-3">
                  <div className="min-w-0">
                    <div className="font-medium">{doc.title}</div>
                    <div className="text-xs text-muted-foreground">
                      {t("detail.updated", { time: new Date(doc.updatedAt).toLocaleString() })}
                    </div>
                  </div>
                  <div className="flex gap-1">
                    <Button asChild type="button" size="icon-sm" variant="ghost">
                      <Link href={buildDocsHref(doc.pageId)}>
                        <ExternalLink className="size-4" />
                      </Link>
                    </Button>
                    <Button
                      type="button"
                      size="sm"
                      variant="ghost"
                      aria-label={`Remove ${doc.title}`}
                      onClick={() => onRemoveLink?.(doc.id)}
                    >
                      {t("detail.remove")}
                    </Button>
                  </div>
                </div>
                {doc.preview ? (
                  <div className="mt-2 rounded bg-muted/40 px-2 py-1 text-xs text-muted-foreground">
                    {doc.preview}
                  </div>
                ) : null}
              </div>
            ))}
          </div>
        ))}
        {docs.length === 0 ? (
          <div className="rounded-md border border-dashed border-border/60 px-3 py-4 text-muted-foreground">
            {t("detail.noLinkedDocs", { taskId, projectId })}
          </div>
        ) : null}
      </div>
    </div>
  );
}
