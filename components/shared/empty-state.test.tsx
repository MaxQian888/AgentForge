import { Inbox } from "lucide-react";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { EmptyState } from "./empty-state";

describe("EmptyState", () => {
  it("renders descriptive copy and invokes action callbacks", async () => {
    const user = userEvent.setup();
    const onClick = jest.fn();

    render(
      <EmptyState
        icon={Inbox}
        title="No tasks"
        description="Create your first task to get started."
        action={{ label: "Create task", onClick }}
      />,
    );

    expect(screen.getByText("No tasks")).toBeInTheDocument();
    expect(screen.getByText("Create your first task to get started.")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Create task" }));

    expect(onClick).toHaveBeenCalled();
  });

  it("renders link actions when an href is supplied", () => {
    render(
      <EmptyState
        icon={Inbox}
        title="No docs"
        action={{ label: "Open docs", href: "/docs" }}
      />,
    );

    expect(screen.getByRole("link", { name: "Open docs" })).toHaveAttribute(
      "href",
      "/docs",
    );
  });
});
