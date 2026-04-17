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

import { CostSummaryBlock } from "./cost-summary-block";

const targetRef = {
  kind: "cost_summary" as const,
  filter: {
    range_start: "2025-04-01T00:00:00Z",
    range_end: "2025-04-08T00:00:00Z",
    runtime: "claude_code",
  },
};

const okFixture: ProjectionResult = {
  status: "ok",
  projected_at: "2025-04-17T10:00:00Z",
  projection: [
    { id: "h", type: "heading", props: { level: 2 }, content: "Weekly cost" },
    { id: "p", type: "paragraph", content: "Total $42.10 across 3 runtimes." },
  ],
};

const cachedOk: BlockNoteBlock[] = [
  { id: "h", type: "heading", props: { level: 2 }, content: "Cached total" },
  { id: "p", type: "paragraph", content: "Previous $40.00 snapshot." },
];

describe("CostSummaryBlock", () => {
  it("renders the ok projection heading and paragraph", () => {
    render(
      <CostSummaryBlock
        blockId="cs-1"
        targetRef={targetRef}
        viewOpts={{ group_by: "runtime" }}
        projection={okFixture}
      />,
    );

    expect(screen.getByText("Weekly cost")).toBeInTheDocument();
    expect(
      screen.getByText(/Total \$42\.10 across 3 runtimes/i),
    ).toBeInTheDocument();
  });

  it("renders not_found with a remove-block button", () => {
    render(
      <CostSummaryBlock
        blockId="cs-1"
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
  });

  it("renders forbidden without leaking filter details", () => {
    render(
      <CostSummaryBlock
        blockId="cs-1"
        targetRef={targetRef}
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
    expect(screen.queryByText("Weekly cost")).not.toBeInTheDocument();
    expect(screen.queryByText(/claude_code/)).not.toBeInTheDocument();
  });

  it("renders degraded diagnostics and shows cached projection dimmed when present", () => {
    render(
      <CostSummaryBlock
        blockId="cs-1"
        targetRef={targetRef}
        viewOpts={{}}
        projection={{
          status: "degraded",
          diagnostics: "cost aggregator slow",
          projected_at: "2025-04-17T10:00:00Z",
        }}
        cachedOk={cachedOk}
      />,
    );

    expect(
      screen.getByText(/Live update temporarily unavailable/i),
    ).toBeInTheDocument();
    expect(screen.getByText(/cost aggregator slow/i)).toBeInTheDocument();
    expect(screen.getByText("Cached total")).toBeInTheDocument();
  });
});
