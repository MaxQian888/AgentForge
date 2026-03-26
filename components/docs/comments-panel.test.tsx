import { render, screen } from "@testing-library/react";
import type { DocsComment } from "@/lib/stores/docs-store";
import { CommentsPanel } from "./comments-panel";

const mockCommentInput = jest.fn();
const mockCommentThread = jest.fn();

jest.mock("./comment-input", () => ({
  CommentInput: (props: {
    onSubmit: (body: string) => void | Promise<void>;
    suggestions?: string[];
  }) => {
    mockCommentInput(props);
    return (
      <button type="button" onClick={() => props.onSubmit("created from test")}>
        Mock Comment Input
      </button>
    );
  },
}));

jest.mock("./comment-thread", () => ({
  CommentThread: (props: {
    comment: DocsComment;
    replies?: DocsComment[];
  }) => {
    mockCommentThread(props);
    return <div data-testid={`comment-thread-${props.comment.id}`}>{props.comment.body}</div>;
  },
}));

function makeComment(overrides: Partial<DocsComment> = {}): DocsComment {
  return {
    id: "comment-1",
    pageId: "page-1",
    anchorBlockId: null,
    parentCommentId: null,
    body: "Root comment",
    mentions: "[]",
    resolvedAt: null,
    createdBy: "user-1",
    createdAt: "2026-03-26T12:00:00.000Z",
    updatedAt: "2026-03-26T12:00:00.000Z",
    deletedAt: null,
    ...overrides,
  };
}

describe("CommentsPanel", () => {
  beforeEach(() => {
    mockCommentInput.mockClear();
    mockCommentThread.mockClear();
  });

  it("passes mention suggestions into the input and renders roots with their replies", () => {
    const onCreateComment = jest.fn();
    const onResolve = jest.fn();
    const onReopen = jest.fn();
    const onCopyLink = jest.fn();
    const root = makeComment();
    const reply = makeComment({
      id: "reply-1",
      parentCommentId: "comment-1",
      body: "Reply comment",
    });

    render(
      <CommentsPanel
        comments={[root, reply]}
        onCreateComment={onCreateComment}
        onResolve={onResolve}
        onReopen={onReopen}
        onCopyLink={onCopyLink}
        mentionSuggestions={["alice", "bob"]}
      />,
    );

    expect(screen.getByText("Comments")).toBeInTheDocument();
    expect(mockCommentInput).toHaveBeenCalledWith(
      expect.objectContaining({
        onSubmit: onCreateComment,
        suggestions: ["alice", "bob"],
      }),
    );
    expect(mockCommentThread).toHaveBeenCalledWith(
      expect.objectContaining({
        comment: root,
        replies: [reply],
        onResolve,
        onReopen,
        onCopyLink,
      }),
    );
    expect(screen.queryByTestId("comment-thread-reply-1")).not.toBeInTheDocument();
  });

  it("shows detached comments in a separate section", () => {
    const detached = makeComment({
      id: "comment-detached",
      body: "Detached note",
      anchorBlockId: "detached:block-9",
    });

    render(<CommentsPanel comments={[detached]} onCreateComment={jest.fn()} />);

    expect(screen.getByText("Detached Comments")).toBeInTheDocument();
    expect(screen.getAllByText("Detached note")).toHaveLength(2);
  });

  it("hides the comment input in readonly mode", () => {
    render(<CommentsPanel comments={[makeComment()]} onCreateComment={jest.fn()} readonly />);

    expect(screen.queryByRole("button", { name: "Mock Comment Input" })).not.toBeInTheDocument();
    expect(screen.getByText(/Shared snapshots are read-only/i)).toBeInTheDocument();
  });
});
