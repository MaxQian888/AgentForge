import type { Breakpoint } from "@/lib/responsive";
import { render, screen } from "@testing-library/react";
import { ResponsiveGrid } from "./responsive-grid";

const breakpointState = {
  breakpoint: "desktop" as Breakpoint,
  isMobile: false,
  isTablet: false,
  isDesktop: true,
};

jest.mock("@/hooks/use-breakpoint", () => ({
  useBreakpoint: () => breakpointState,
}));

describe("ResponsiveGrid", () => {
  afterEach(() => {
    breakpointState.breakpoint = "desktop";
    breakpointState.isMobile = false;
    breakpointState.isTablet = false;
    breakpointState.isDesktop = true;
  });

  it("uses the desktop column count by default", () => {
    render(
      <ResponsiveGrid
        columns={{ mobile: 1, tablet: 2, desktop: 4 }}
        data-testid="grid"
      >
        <div>One</div>
      </ResponsiveGrid>,
    );

    expect(screen.getByTestId("grid")).toHaveAttribute(
      "data-breakpoint",
      "desktop",
    );
    expect(screen.getByTestId("grid")).toHaveStyle({
      gridTemplateColumns: "repeat(4, minmax(0, 1fr))",
    });
  });

  it("switches to the matching breakpoint column count", () => {
    breakpointState.breakpoint = "tablet";
    breakpointState.isTablet = true;
    breakpointState.isDesktop = false;

    render(
      <ResponsiveGrid
        columns={{ mobile: 1, tablet: 2, desktop: 4 }}
        data-testid="grid"
      >
        <div>One</div>
      </ResponsiveGrid>,
    );

    expect(screen.getByTestId("grid")).toHaveAttribute(
      "data-breakpoint",
      "tablet",
    );
    expect(screen.getByTestId("grid")).toHaveStyle({
      gridTemplateColumns: "repeat(2, minmax(0, 1fr))",
    });
  });
});
