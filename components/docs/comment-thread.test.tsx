import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { AssetComment } from "@/lib/stores/knowledge-store";
import { CommentThread } from "./comment-thread";

function makeComment(overrides: Partial<AssetComment> = {}): AssetComment {
  return {
    id: "comment-1",
    assetId: "page-1",
    anchorBlockId: "block-1",
    parentCommentId: null,
    body: "Please update the deployment checklist.",
    mentions: "[]",
    resolvedAt: null,
    createdBy: "user-1",
    createdAt: "2026-03-26T12:00:00.000Z",
    updatedAt: "2026-03-26T12:00:00.000Z",
    deletedAt: null,
    ...overrides,
  };
}

describe("CommentThread", () => {
  it("renders comment metadata, replies, and action callbacks for active comments", async () => {
    const user = userEvent.setup();
    const onResolve = jest.fn();
    const onCopyLink = jest.fn();

    render(
      <CommentThread
        comment={makeComment()}
        replies={[
          makeComment({
            id: "reply-1",
            parentCommentId: "comment-1",
            body: "Added the checklist update.",
          }),
        ]}
        onResolve={onResolve}
        onCopyLink={onCopyLink}
      />,
    );

    expect(screen.getByText("Please update the deployment checklist.")).toBeInTheDocument();
    expect(screen.getByText(/Anchor: block-1/)).toBeInTheDocument();
    expect(screen.getByText("Added the checklist update.")).toBeInTheDocument();

    const buttons = screen.getAllByRole("button");
    await user.click(buttons[0]);
    await user.click(buttons[1]);

    expect(onResolve).toHaveBeenCalledWith("comment-1");
    expect(onCopyLink).toHaveBeenCalledWith("comment-1");
  });

  it("switches to reopen action for resolved page-level comments", async () => {
    const user = userEvent.setup();
    const onReopen = jest.fn();

    render(
      <CommentThread
        comment={makeComment({
          anchorBlockId: null,
          resolvedAt: "2026-03-26T13:00:00.000Z",
        })}
        onReopen={onReopen}
      />,
    );

    expect(screen.getByText(/Page level/)).toBeInTheDocument();

    await user.click(screen.getAllByRole("button")[0]);
    expect(onReopen).toHaveBeenCalledWith("comment-1");
  });
});
