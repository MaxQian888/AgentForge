"use client";

import { Button } from "@/components/ui/button";
import type { TaskComment } from "@/lib/stores/task-comment-store";
import { TaskCommentInput } from "./task-comment-input";

export function TaskComments({
  comments,
  mentionSuggestions = [],
  onCreateComment,
  onResolveComment,
  onReopenComment,
}: {
  comments: TaskComment[];
  mentionSuggestions?: string[];
  onCreateComment: (body: string) => void | Promise<void>;
  onResolveComment?: (commentId: string) => void;
  onReopenComment?: (commentId: string) => void;
}) {
  const roots = comments.filter((comment) => !comment.parentCommentId);

  return (
    <div className="rounded-lg border border-border/60 bg-muted/20 p-3 text-sm">
      <div className="font-medium">Task Comments</div>
      <div className="mt-1 text-muted-foreground">
        Discuss blockers, handoff details, and review follow-ups inline.
      </div>

      <div className="mt-3">
        <TaskCommentInput onSubmit={onCreateComment} suggestions={mentionSuggestions} />
      </div>

      <div className="mt-3 space-y-3">
        {roots.map((comment) => {
          const replies = comments.filter((reply) => reply.parentCommentId === comment.id);
          return (
            <div
              key={comment.id}
              className="rounded-md border border-border/60 bg-background px-3 py-2"
            >
              <div className="flex items-start justify-between gap-3">
                <div className="min-w-0">
                  <div className="font-medium">{comment.body}</div>
                  <div className="text-xs text-muted-foreground">
                    {new Date(comment.createdAt).toLocaleString()}
                  </div>
                </div>
                {comment.resolvedAt ? (
                  <Button
                    type="button"
                    size="sm"
                    variant="ghost"
                    aria-label={`Reopen ${comment.id}`}
                    onClick={() => onReopenComment?.(comment.id)}
                  >
                    Reopen
                  </Button>
                ) : (
                  <Button
                    type="button"
                    size="sm"
                    variant="ghost"
                    aria-label={`Resolve ${comment.id}`}
                    onClick={() => onResolveComment?.(comment.id)}
                  >
                    Resolve
                  </Button>
                )}
              </div>

              {replies.length > 0 ? (
                <div className="mt-3 space-y-2 border-l border-border/60 pl-3">
                  {replies.map((reply) => (
                    <div key={reply.id} className="rounded bg-muted/40 px-2 py-1 text-xs">
                      {reply.body}
                    </div>
                  ))}
                </div>
              ) : null}
            </div>
          );
        })}
        {roots.length === 0 ? (
          <div className="rounded-md border border-dashed border-border/60 px-3 py-4 text-muted-foreground">
            No task comments yet.
          </div>
        ) : null}
      </div>
    </div>
  );
}
