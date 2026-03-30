jest.mock("next-intl", () => ({
  useTranslations: () => (key: string) => {
    const map: Record<string, string> = {
      "comments.placeholder": "Add a comment",
      "comments.submit": "Send",
    };
    return map[key] ?? key;
  },
}));

import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { TaskCommentInput } from "./task-comment-input";

describe("TaskCommentInput", () => {
  it("shows mention suggestions and submits non-empty comments", async () => {
    const user = userEvent.setup();
    const onSubmit = jest.fn().mockResolvedValue(undefined);

    render(
      <TaskCommentInput onSubmit={onSubmit} suggestions={["alice", "bob"]} />,
    );

    const input = screen.getByPlaceholderText("Add a comment");
    await user.type(input, "@ali");
    await user.click(screen.getByRole("button", { name: "@alice" }));
    expect(input).toHaveValue("@alice");

    await user.type(input, " please review");
    await user.click(screen.getByRole("button", { name: "Send" }));

    await waitFor(() => {
      expect(onSubmit).toHaveBeenCalledWith("@alice please review");
    });
    expect(input).toHaveValue("");
  });

  it("does not submit blank comments", async () => {
    const user = userEvent.setup();
    const onSubmit = jest.fn();

    render(<TaskCommentInput onSubmit={onSubmit} />);

    await user.click(screen.getByRole("button", { name: "Send" }));
    expect(onSubmit).not.toHaveBeenCalled();
  });
});
