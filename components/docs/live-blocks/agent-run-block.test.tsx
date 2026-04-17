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

import { AgentRunBlock } from "./agent-run-block";

const okFixture: ProjectionResult = {
  status: "ok",
  projected_at: "2025-04-17T10:00:00Z",
  projection: [
    {
      id: "b1",
      type: "heading",
      props: { level: 2 },
      content: "Planner agent run",
    },
    {
      id: "b2",
      type: "paragraph",
      content: "Running on claude_code runtime with 4 steps.",
    },
  ],
};

const cachedOkFixture: BlockNoteBlock[] = [
  {
    id: "b1",
    type: "heading",
    props: { level: 2 },
    content: "Cached run",
  },
  { id: "b2", type: "paragraph", content: "Earlier snapshot" },
];

describe("AgentRunBlock", () => {
  it("renders the projected heading and paragraph in the ok state", () => {
    render(
      <AgentRunBlock
        blockId="block-a"
        targetRef={{ kind: "agent_run", id: "run-123" }}
        viewOpts={{ show_log_lines: 10 }}
        projection={okFixture}
      />,
    );

    expect(screen.getByText("Planner agent run")).toBeInTheDocument();
    expect(
      screen.getByText(/Running on claude_code runtime with 4 steps/i),
    ).toBeInTheDocument();
  });

  it("renders the not_found state with a remove-block button", () => {
    render(
      <AgentRunBlock
        blockId="block-a"
        targetRef={{ kind: "agent_run", id: "run-123" }}
        viewOpts={{}}
        projection={{
          status: "not_found",
          projected_at: "2025-04-17T10:00:00Z",
        }}
      />,
    );

    // Banner from chrome + body copy both mention "no longer available".
    const matches = screen.getAllByText(/no longer available/i);
    expect(matches.length).toBeGreaterThan(0);
    expect(
      screen.getByText(/referenced agent run may have been deleted/i),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: /Remove block/i }),
    ).toBeInTheDocument();
    // The target id must not leak into the DOM.
    expect(screen.queryByText(/run-123/)).not.toBeInTheDocument();
  });

  it("renders the forbidden state without leaking the target title or id", () => {
    render(
      <AgentRunBlock
        blockId="block-a"
        targetRef={{ kind: "agent_run", id: "secret-run" }}
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
    expect(screen.queryByText(/secret-run/)).not.toBeInTheDocument();
    // No title/heading from the projection either.
    expect(screen.queryByText("Planner agent run")).not.toBeInTheDocument();
  });

  it("renders degraded state with diagnostics and dims the cached projection when available", () => {
    render(
      <AgentRunBlock
        blockId="block-a"
        targetRef={{ kind: "agent_run", id: "run-123" }}
        viewOpts={{}}
        projection={{
          status: "degraded",
          diagnostics: "runtime slow",
          projected_at: "2025-04-17T10:00:00Z",
        }}
        cachedOk={cachedOkFixture}
      />,
    );

    expect(
      screen.getByText(/Live update temporarily unavailable/i),
    ).toBeInTheDocument();
    expect(screen.getByText(/runtime slow/i)).toBeInTheDocument();
    // Cached content still renders (dimmed).
    expect(screen.getByText("Cached run")).toBeInTheDocument();
  });

  it("renders temporarily-unavailable copy when degraded without a cache", () => {
    render(
      <AgentRunBlock
        blockId="block-a"
        targetRef={{ kind: "agent_run", id: "run-123" }}
        viewOpts={{}}
        projection={{
          status: "degraded",
          diagnostics: "timeout",
          projected_at: "2025-04-17T10:00:00Z",
        }}
      />,
    );

    expect(
      screen.getByText(/Live update temporarily unavailable/i),
    ).toBeInTheDocument();
    expect(
      screen.getByText(/last-known data is unavailable/i),
    ).toBeInTheDocument();
  });
});
