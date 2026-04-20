jest.mock("@/components/ui/tabs", () => ({
  Tabs: ({
    value,
    onValueChange,
    children,
  }: {
    value: string;
    onValueChange: (value: string) => void;
    children?: React.ReactNode;
  }) => (
    <div data-testid="tabs-root" data-value={value} data-onchange={Boolean(onValueChange)}>
      {children}
    </div>
  ),
  TabsList: ({ children }: { children?: React.ReactNode }) => (
    <div role="tablist">{children}</div>
  ),
  TabsTrigger: ({
    value,
    children,
    disabled,
  }: {
    value: string;
    children?: React.ReactNode;
    disabled?: boolean;
  }) => (
    <button type="button" data-value={value} disabled={disabled} role="tab">
      {children}
    </button>
  ),
  TabsContent: ({
    value,
    children,
  }: {
    value: string;
    children?: React.ReactNode;
  }) => (
    <div data-testid={`tabs-content-${value}`}>{children}</div>
  ),
}));

jest.mock("@/components/ui/select", () => ({
  Select: ({
    value,
    onValueChange,
    children,
  }: {
    value: string;
    onValueChange: (value: string) => void;
    children?: React.ReactNode;
  }) => (
    <div data-testid="mobile-select" data-value={value} data-onchange={Boolean(onValueChange)}>
      {children}
    </div>
  ),
  SelectTrigger: ({
    children,
    "aria-label": ariaLabel,
  }: {
    children?: React.ReactNode;
    "aria-label"?: string;
  }) => <button type="button" aria-label={ariaLabel}>{children}</button>,
  SelectValue: ({ placeholder }: { placeholder?: string }) => (
    <span data-testid="select-value">{placeholder}</span>
  ),
  SelectContent: ({ children }: { children?: React.ReactNode }) => <>{children}</>,
  SelectItem: ({
    value,
    children,
    disabled,
  }: {
    value: string;
    children?: React.ReactNode;
    disabled?: boolean;
  }) => (
    <button type="button" data-mobile-option={value} disabled={disabled}>
      {children ?? value}
    </button>
  ),
}));

import { render, screen } from "@testing-library/react";
import { ResponsiveTabs } from "./responsive-tabs";

describe("ResponsiveTabs", () => {
  it("renders a tab list plus a mobile Select fallback", () => {
    render(
      <ResponsiveTabs
        value="overview"
        onValueChange={jest.fn()}
        ariaLabel="Section"
        items={[
          { value: "overview", label: "Overview" },
          { value: "settings", label: "Settings", disabled: true },
        ]}
      />,
    );

    expect(screen.getByRole("tablist")).toBeInTheDocument();
    expect(screen.getByRole("tab", { name: "Overview" })).toBeEnabled();
    expect(screen.getByRole("tab", { name: "Settings" })).toBeDisabled();

    expect(screen.getByTestId("mobile-select")).toHaveAttribute(
      "data-value",
      "overview",
    );
    expect(screen.getByRole("button", { name: "Section" })).toBeInTheDocument();
  });

  it("renders inline content panels when items declare content", () => {
    render(
      <ResponsiveTabs
        value="overview"
        onValueChange={jest.fn()}
        items={[
          { value: "overview", label: "Overview", content: <p>overview body</p> },
          { value: "runs", label: "Runs", content: <p>runs body</p> },
        ]}
      />,
    );

    expect(screen.getByTestId("tabs-content-overview")).toHaveTextContent(
      "overview body",
    );
    expect(screen.getByTestId("tabs-content-runs")).toHaveTextContent(
      "runs body",
    );
  });

  it("renders children when items do not declare inline content", () => {
    render(
      <ResponsiveTabs
        value="overview"
        onValueChange={jest.fn()}
        items={[
          { value: "overview", label: "Overview" },
          { value: "runs", label: "Runs" },
        ]}
      >
        <div data-testid="child-panel">external panel</div>
      </ResponsiveTabs>,
    );

    expect(screen.getByTestId("child-panel")).toBeInTheDocument();
  });
});
