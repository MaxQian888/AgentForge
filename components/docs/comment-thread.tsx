"use client";

import { useTranslations } from "next-intl";
import { MessageSquareQuote, RotateCcw, CheckCircle2, Link2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import type { DocsComment } from "@/lib/stores/docs-store";

export function CommentThread({
  comment,
  replies = [],
  onResolve,
  onReopen,
  onCopyLink,
}: {
  comment: DocsComment;
  replies?: DocsComment[];
  onResolve?: (commentId: string) => void;
  onReopen?: (commentId: string) => void;
  onCopyLink?: (commentId: string) => void;
}) {
  const t = useTranslations("docs");

  return (
    <div className="rounded-xl border border-border/60 bg-card/80 p-3">
      <div className="flex items-start justify-between gap-3">
        <div className="flex items-start gap-2">
          <MessageSquareQuote className="mt-0.5 size-4 text-muted-foreground" />
          <div className="space-y-1">
            <p className="text-sm font-medium">{comment.body}</p>
            <p className="text-xs text-muted-foreground">
              {comment.anchorBlockId ? t("comments.anchor", { id: comment.anchorBlockId }) : t("comments.pageLevel")} ·{" "}
              {new Date(comment.createdAt).toLocaleString()}
            </p>
          </div>
        </div>
        <div className="flex gap-1">
          {comment.resolvedAt ? (
            <Button size="icon-sm" variant="ghost" onClick={() => onReopen?.(comment.id)}>
              <RotateCcw className="size-4" />
            </Button>
          ) : (
            <Button size="icon-sm" variant="ghost" onClick={() => onResolve?.(comment.id)}>
              <CheckCircle2 className="size-4" />
            </Button>
          )}
          <Button size="icon-sm" variant="ghost" onClick={() => onCopyLink?.(comment.id)}>
            <Link2 className="size-4" />
          </Button>
        </div>
      </div>
      {replies.length > 0 ? (
        <div className="mt-3 flex flex-col gap-2 border-l pl-4">
          {replies.map((reply) => (
            <div key={reply.id} className="rounded-lg bg-muted/30 px-3 py-2 text-sm">
              {reply.body}
            </div>
          ))}
        </div>
      ) : null}
    </div>
  );
}
