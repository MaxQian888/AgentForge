"use client";

import { useMemo } from "react";
import { useTranslations } from "next-intl";
import type { AssetComment } from "@/lib/stores/knowledge-store";
import { CommentInput } from "./comment-input";
import { CommentThread } from "./comment-thread";

export function CommentsPanel({
  comments,
  onCreateComment,
  onResolve,
  onReopen,
  onCopyLink,
  mentionSuggestions = [],
  readonly = false,
}: {
  comments: AssetComment[];
  onCreateComment: (body: string) => void | Promise<void>;
  onResolve?: (commentId: string) => void;
  onReopen?: (commentId: string) => void;
  onCopyLink?: (commentId: string) => void;
  mentionSuggestions?: string[];
  readonly?: boolean;
}) {
  const t = useTranslations("docs");
  const { roots, detached } = useMemo(() => {
    const rootComments = comments.filter((comment) => !comment.parentCommentId);
    const detachedComments = comments.filter(
      (comment) => comment.anchorBlockId && comment.anchorBlockId.startsWith("detached:")
    );
    return { roots: rootComments, detached: detachedComments };
  }, [comments]);

  return (
    <div className="flex flex-col gap-4 rounded-xl border border-border/60 bg-card/70 p-4">
      <div>
        <h2 className="text-base font-semibold">{t("comments.title")}</h2>
        <p className="text-sm text-muted-foreground">
          {t("comments.desc")}
        </p>
      </div>

      {readonly ? (
        <p className="rounded-lg border border-dashed border-border/70 px-3 py-2 text-sm text-muted-foreground">
          {t("comments.readonlyHint")}
        </p>
      ) : (
        <CommentInput onSubmit={onCreateComment} suggestions={mentionSuggestions} />
      )}

      <div className="flex flex-col gap-3">
        {roots.map((comment) => (
          <CommentThread
            key={comment.id}
            comment={comment}
            replies={comments.filter((reply) => reply.parentCommentId === comment.id)}
            onResolve={readonly ? undefined : onResolve}
            onReopen={readonly ? undefined : onReopen}
            onCopyLink={onCopyLink}
          />
        ))}
      </div>

      {detached.length > 0 ? (
        <div className="rounded-lg border border-dashed border-border/70 p-3">
          <h3 className="text-sm font-medium">{t("comments.detachedTitle")}</h3>
          <p className="mb-2 text-xs text-muted-foreground">
            {t("comments.detachedDesc")}
          </p>
          <div className="flex flex-col gap-2">
            {detached.map((comment) => (
              <CommentThread key={comment.id} comment={comment} />
            ))}
          </div>
        </div>
      ) : null}
    </div>
  );
}
