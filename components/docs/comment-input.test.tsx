import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { CommentInput } from "./comment-input";

describe("CommentInput", () => {
  it("filters mention suggestions and inserts the selected handle", async () => {
    const user = userEvent.setup();

    render(
      <CommentInput
        onSubmit={jest.fn()}
        suggestions={["alice", "bob", "alina"]}
      />,
    );

    const input = screen.getByPlaceholderText("Write a comment…");
    await user.type(input, "Need review from @ali");

    expect(screen.getByRole("button", { name: "@alice" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "@alina" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "@bob" })).not.toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "@alice" }));
    expect(input).toHaveValue("Need review from @alice ");
  });

  it("submits non-empty comments and clears the input afterwards", async () => {
    const user = userEvent.setup();
    const onSubmit = jest.fn().mockResolvedValue(undefined);

    render(<CommentInput onSubmit={onSubmit} />);

    const input = screen.getByPlaceholderText("Write a comment…");

    await user.type(input, "   ");
    await user.click(screen.getByRole("button", { name: "Comment" }));
    expect(onSubmit).not.toHaveBeenCalled();

    await user.clear(input);
    await user.type(input, "Ship the docs workspace");
    await user.click(screen.getByRole("button", { name: "Comment" }));

    await waitFor(() => {
      expect(onSubmit).toHaveBeenCalledWith("Ship the docs workspace");
    });
    expect(input).toHaveValue("");
  });
});
