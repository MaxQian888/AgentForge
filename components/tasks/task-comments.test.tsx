import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { TaskComments } from "./task-comments";

describe("TaskComments", () => {
  it("renders comments, replies, and supports comment actions", async () => {
    const user = userEvent.setup();
    const onCreate = jest.fn();
    const onResolve = jest.fn();
    const onReopen = jest.fn();

    render(
      <TaskComments
        comments={[
          {
            id: "comment-1",
            taskId: "task-1",
            body: "Need docs",
            mentions: ["alice"],
            createdBy: "user-1",
            createdAt: "2026-03-26T10:00:00.000Z",
            updatedAt: "2026-03-26T10:00:00.000Z",
            parentCommentId: null,
            resolvedAt: null,
            deletedAt: null,
          },
          {
            id: "comment-2",
            taskId: "task-1",
            body: "On it",
            mentions: [],
            createdBy: "user-2",
            createdAt: "2026-03-26T10:01:00.000Z",
            updatedAt: "2026-03-26T10:01:00.000Z",
            parentCommentId: "comment-1",
            resolvedAt: null,
            deletedAt: null,
          },
        ]}
        mentionSuggestions={["alice", "bob"]}
        onCreateComment={onCreate}
        onResolveComment={onResolve}
        onReopenComment={onReopen}
      />,
    );

    expect(screen.getByText("Task Comments")).toBeInTheDocument();
    expect(screen.getByText("Need docs")).toBeInTheDocument();
    expect(screen.getByText("On it")).toBeInTheDocument();

    await user.type(screen.getByPlaceholderText("Add a task comment…"), "@a");
    expect(screen.getByRole("button", { name: "@alice" })).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Comment" }));
    expect(onCreate).toHaveBeenCalledWith("@a");

    await user.click(screen.getByRole("button", { name: "Resolve comment-1" }));
    expect(onResolve).toHaveBeenCalledWith("comment-1");
  });
});
