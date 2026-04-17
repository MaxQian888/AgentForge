import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

// Inline the shadcn/ui dropdown-menu with a simple implementation so the
// test does not depend on Radix portals / pointer APIs.
jest.mock("@/components/ui/dropdown-menu", () => {
  const React = jest.requireActual("react") as typeof import("react");
  return {
    DropdownMenu: ({ children }: { children: React.ReactNode }) => (
      <div>{children}</div>
    ),
    DropdownMenuTrigger: ({
      children,
    }: {
      children: React.ReactNode;
      asChild?: boolean;
    }) => <>{children}</>,
    DropdownMenuContent: ({ children }: { children: React.ReactNode }) => (
      <div role="menu">{children}</div>
    ),
    DropdownMenuItem: (props: {
      children: React.ReactNode;
      onSelect?: (event: Event) => void;
      disabled?: boolean;
      variant?: "default" | "destructive";
    }) => (
      <button
        type="button"
        role="menuitem"
        disabled={props.disabled}
        aria-disabled={props.disabled}
        onClick={() => {
          const event = new Event("select", { cancelable: true });
          props.onSelect?.(event);
        }}
      >
        {props.children}
      </button>
    ),
  };
});

import { LiveArtifactChrome } from "./live-artifact-chrome";

describe("LiveArtifactChrome", () => {
  const baseProps = {
    kind: "agent_run",
    title: "Agent run",
    onOpenSource: jest.fn(),
    onFreeze: jest.fn(),
    onRemove: jest.fn(),
  };

  beforeEach(() => {
    baseProps.onOpenSource.mockReset();
    baseProps.onFreeze.mockReset();
    baseProps.onRemove.mockReset();
  });

  it("renders the kind pill and invokes the open-source action", async () => {
    const user = userEvent.setup();
    render(
      <LiveArtifactChrome {...baseProps} status="ok">
        <div>body</div>
      </LiveArtifactChrome>,
    );

    expect(screen.getByText("Agent run")).toBeInTheDocument();
    expect(screen.getByText("body")).toBeInTheDocument();

    await user.click(screen.getByRole("menuitem", { name: /Open source/i }));
    expect(baseProps.onOpenSource).toHaveBeenCalledTimes(1);
  });

  it("invokes freeze only when status is ok", async () => {
    const user = userEvent.setup();
    const { rerender } = render(
      <LiveArtifactChrome {...baseProps} status="ok">
        <div>body</div>
      </LiveArtifactChrome>,
    );

    await user.click(screen.getByRole("menuitem", { name: /Freeze/i }));
    expect(baseProps.onFreeze).toHaveBeenCalledTimes(1);

    rerender(
      <LiveArtifactChrome {...baseProps} status="degraded">
        <div>body</div>
      </LiveArtifactChrome>,
    );

    const freezeItem = screen.getByRole("menuitem", { name: /Freeze/i });
    expect(freezeItem).toBeDisabled();
  });

  it("renders a status banner in degraded state and passes diagnostics through", () => {
    render(
      <LiveArtifactChrome
        {...baseProps}
        status="degraded"
        diagnostics="cost query timeout"
      >
        <div>body</div>
      </LiveArtifactChrome>,
    );

    expect(
      screen.getByText(/Live update temporarily unavailable/i),
    ).toBeInTheDocument();
    expect(screen.getByText(/cost query timeout/i)).toBeInTheDocument();
  });

  it("renders the 'no longer available' banner in not_found state", () => {
    render(
      <LiveArtifactChrome {...baseProps} status="not_found">
        <div>body</div>
      </LiveArtifactChrome>,
    );

    expect(
      screen.getByText(/This live artifact is no longer available/i),
    ).toBeInTheDocument();
  });

  it("renders the 'no access' banner in forbidden state without leaking target details", () => {
    render(
      <LiveArtifactChrome {...baseProps} status="forbidden">
        <div>body</div>
      </LiveArtifactChrome>,
    );

    expect(
      screen.getByText(/You do not have access to this live artifact/i),
    ).toBeInTheDocument();
  });

  it("invokes remove when the Remove item is clicked", async () => {
    const user = userEvent.setup();
    render(
      <LiveArtifactChrome {...baseProps} status="ok">
        <div>body</div>
      </LiveArtifactChrome>,
    );

    await user.click(screen.getByRole("menuitem", { name: /Remove/i }));
    expect(baseProps.onRemove).toHaveBeenCalledTimes(1);
  });
});
