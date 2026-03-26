"use client";

import { useMemo } from "react";
import type { DocsComment } from "@/lib/stores/docs-store";
import { CommentInput } from "./comment-input";
import { CommentThread } from "./comment-thread";

export function CommentsPanel({
  comments,
  onCreateComment,
  onResolve,
  onReopen,
  onCopyLink,
  mentionSuggestions = [],
}: {
  comments: DocsComment[];
  onCreateComment: (body: string) => void | Promise<void>;
  onResolve?: (commentId: string) => void;
  onReopen?: (commentId: string) => void;
  onCopyLink?: (commentId: string) => void;
  mentionSuggestions?: string[];
}) {
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
        <h2 className="text-base font-semibold">Comments</h2>
        <p className="text-sm text-muted-foreground">
          Page-level and inline discussions stay next to the draft.
        </p>
      </div>

      <CommentInput onSubmit={onCreateComment} suggestions={mentionSuggestions} />

      <div className="flex flex-col gap-3">
        {roots.map((comment) => (
          <CommentThread
            key={comment.id}
            comment={comment}
            replies={comments.filter((reply) => reply.parentCommentId === comment.id)}
            onResolve={onResolve}
            onReopen={onReopen}
            onCopyLink={onCopyLink}
          />
        ))}
      </div>

      {detached.length > 0 ? (
        <div className="rounded-lg border border-dashed border-border/70 p-3">
          <h3 className="text-sm font-medium">Detached Comments</h3>
          <p className="mb-2 text-xs text-muted-foreground">
            These comments reference blocks that no longer exist in the current draft.
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
