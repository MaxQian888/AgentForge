import { render, screen } from "@testing-library/react";
import type { BlockNoteBlock, ProjectionResult } from "./types";

jest.mock("sonner", () => ({
  toast: { success: jest.fn(), error: jest.fn() },
}));

jest.mock("next/navigation", () => ({
  useRouter: () => ({ push: jest.fn() }),
}));

jest.mock("@/components/ui/dropdown-menu", () => {
  const React = jest.requireActual("react") as typeof import("react");
  return {
    DropdownMenu: ({ children }: { children: React.ReactNode }) => (
      <div>{children}</div>
    ),
    DropdownMenuTrigger: ({ children }: { children: React.ReactNode }) => (
      <>{children}</>
    ),
    DropdownMenuContent: ({ children }: { children: React.ReactNode }) => (
      <div role="menu">{children}</div>
    ),
    DropdownMenuItem: ({
      children,
      onSelect,
      disabled,
    }: {
      children: React.ReactNode;
      onSelect?: (event: Event) => void;
      disabled?: boolean;
    }) => (
      <button
        type="button"
        role="menuitem"
        disabled={disabled}
        onClick={() => onSelect?.(new Event("select", { cancelable: true }))}
      >
        {children}
      </button>
    ),
  };
});

import { TaskGroupBlock } from "./task-group-block";

const targetRef = {
  kind: "task_group" as const,
  filter: { saved_view_id: "view-alpha" },
};

const okFixture: ProjectionResult = {
  status: "ok",
  projected_at: "2025-04-17T10:00:00Z",
  projection: [
    { id: "h", type: "heading", props: { level: 2 }, content: "Sprint tasks" },
    { id: "p", type: "paragraph", content: "12 tasks, 3 blocked." },
  ],
};

const cachedOk: BlockNoteBlock[] = [
  { id: "h", type: "heading", props: { level: 2 }, content: "Cached tasks" },
  { id: "p", type: "paragraph", content: "Earlier run: 10 tasks." },
];

describe("TaskGroupBlock", () => {
  it("renders the ok projection", () => {
    render(
      <TaskGroupBlock
        blockId="tg-1"
        targetRef={targetRef}
        viewOpts={{}}
        projection={okFixture}
      />,
    );

    expect(screen.getByText("Sprint tasks")).toBeInTheDocument();
    expect(screen.getByText(/12 tasks, 3 blocked/i)).toBeInTheDocument();
  });

  it("renders not_found with a remove-block button", () => {
    render(
      <TaskGroupBlock
        blockId="tg-1"
        targetRef={targetRef}
        viewOpts={{}}
        projection={{
          status: "not_found",
          projected_at: "2025-04-17T10:00:00Z",
        }}
      />,
    );

    const matches = screen.getAllByText(/no longer available/i);
    expect(matches.length).toBeGreaterThan(0);
    expect(
      screen.getByRole("button", { name: /Remove block/i }),
    ).toBeInTheDocument();
    expect(screen.queryByText(/view-alpha/)).not.toBeInTheDocument();
  });

  it("renders forbidden without leaking filter or projection", () => {
    render(
      <TaskGroupBlock
        blockId="tg-1"
        targetRef={{
          kind: "task_group",
          filter: { saved_view_id: "hidden-view" },
        }}
        viewOpts={{}}
        projection={{
          status: "forbidden",
          projected_at: "2025-04-17T10:00:00Z",
        }}
      />,
    );

    expect(
      screen.getByText(/You do not have access to this live artifact/i),
    ).toBeInTheDocument();
    expect(screen.queryByText("Sprint tasks")).not.toBeInTheDocument();
    expect(screen.queryByText(/hidden-view/)).not.toBeInTheDocument();
  });

  it("renders degraded with diagnostics and dimmed cached content", () => {
    render(
      <TaskGroupBlock
        blockId="tg-1"
        targetRef={targetRef}
        viewOpts={{}}
        projection={{
          status: "degraded",
          diagnostics: "task query slow",
          projected_at: "2025-04-17T10:00:00Z",
        }}
        cachedOk={cachedOk}
      />,
    );

    expect(
      screen.getByText(/Live update temporarily unavailable/i),
    ).toBeInTheDocument();
    expect(screen.getByText(/task query slow/i)).toBeInTheDocument();
    expect(screen.getByText("Cached tasks")).toBeInTheDocument();
  });
});
