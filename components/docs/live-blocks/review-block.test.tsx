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

import { ReviewBlock } from "./review-block";

const okFixture: ProjectionResult = {
  status: "ok",
  projected_at: "2025-04-17T10:00:00Z",
  projection: [
    { id: "h", type: "heading", props: { level: 2 }, content: "Review status" },
    {
      id: "p",
      type: "paragraph",
      content: "pending_human — linked task: T-42 deploy verification",
    },
  ],
};

const cachedOk: BlockNoteBlock[] = [
  { id: "h", type: "heading", props: { level: 2 }, content: "Cached state" },
  { id: "p", type: "paragraph", content: "Last known: in_progress" },
];

describe("ReviewBlock", () => {
  it("renders the ok projection", () => {
    render(
      <ReviewBlock
        blockId="rv-1"
        targetRef={{ kind: "review", id: "rev-77" }}
        viewOpts={{}}
        projection={okFixture}
      />,
    );

    expect(screen.getByText("Review status")).toBeInTheDocument();
    expect(
      screen.getByText(/pending_human — linked task: T-42/i),
    ).toBeInTheDocument();
  });

  it("renders not_found with a remove-block button", () => {
    render(
      <ReviewBlock
        blockId="rv-1"
        targetRef={{ kind: "review", id: "rev-77" }}
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
    expect(screen.queryByText(/rev-77/)).not.toBeInTheDocument();
  });

  it("renders forbidden without leaking target id or projection", () => {
    render(
      <ReviewBlock
        blockId="rv-1"
        targetRef={{ kind: "review", id: "rev-secret" }}
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
    expect(screen.queryByText("Review status")).not.toBeInTheDocument();
    expect(screen.queryByText(/rev-secret/)).not.toBeInTheDocument();
  });

  it("renders degraded state with diagnostics and dimmed cached content", () => {
    render(
      <ReviewBlock
        blockId="rv-1"
        targetRef={{ kind: "review", id: "rev-77" }}
        viewOpts={{}}
        projection={{
          status: "degraded",
          diagnostics: "review service slow",
          projected_at: "2025-04-17T10:00:00Z",
        }}
        cachedOk={cachedOk}
      />,
    );

    expect(
      screen.getByText(/Live update temporarily unavailable/i),
    ).toBeInTheDocument();
    expect(screen.getByText(/review service slow/i)).toBeInTheDocument();
    expect(screen.getByText("Cached state")).toBeInTheDocument();
  });
});
