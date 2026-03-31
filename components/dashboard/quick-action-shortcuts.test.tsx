import { fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Plus, Rocket } from "lucide-react";
import { QuickActionShortcuts } from "./quick-action-shortcuts";

describe("QuickActionShortcuts", () => {
  it("renders action labels together with keyboard shortcut hints", () => {
    render(
      <QuickActionShortcuts
        actions={[
          {
            id: "create-task",
            label: "Create Task",
            href: "/project?id=project-1",
            icon: Plus,
            shortcut: "N",
          },
          {
            id: "spawn-agent",
            label: "Spawn Agent",
            href: "/agents",
            icon: Rocket,
            shortcut: "A",
          },
        ]}
      />,
    );

    expect(screen.getByRole("link", { name: /Create Task/i })).toHaveAttribute(
      "href",
      "/project?id=project-1",
    );
    expect(screen.getByText("N")).toBeInTheDocument();
    expect(screen.getByText("A")).toBeInTheDocument();
  });

  it("executes the matching action when its shortcut key is pressed", async () => {
    const user = userEvent.setup();
    const onTrigger = jest.fn();

    render(
      <QuickActionShortcuts
        actions={[
          {
            id: "create-task",
            label: "Create Task",
            href: "/project?id=project-1",
            icon: Plus,
            shortcut: "N",
            onTrigger,
          },
        ]}
      />,
    );

    await user.click(screen.getByRole("link", { name: /Create Task/i }));
    expect(onTrigger).toHaveBeenCalledTimes(1);

    fireEvent.keyDown(document, { key: "n" });
    expect(onTrigger).toHaveBeenCalledTimes(2);
  });

  it("does not trigger shortcuts while typing in an input", () => {
    const onTrigger = jest.fn();

    render(
      <>
        <input aria-label="search" />
        <QuickActionShortcuts
          actions={[
            {
              id: "create-task",
              label: "Create Task",
              href: "/project?id=project-1",
              icon: Plus,
              shortcut: "N",
              onTrigger,
            },
          ]}
        />
      </>,
    );

    const input = screen.getByLabelText("search");
    input.focus();

    fireEvent.keyDown(document, { key: "n" });

    expect(onTrigger).not.toHaveBeenCalled();
  });
});
