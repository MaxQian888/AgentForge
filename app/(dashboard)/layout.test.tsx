import { render, screen } from "@testing-library/react";
import type { ReactNode } from "react";
import DashboardLayout from "./layout";

jest.mock("@/components/layout/dashboard-shell", () => ({
  DashboardShell: ({ children }: { children: ReactNode }) => (
    <div data-testid="dashboard-shell">{children}</div>
  ),
}));

describe("DashboardLayout", () => {
  it("wraps dashboard routes with the dashboard shell", () => {
    render(
      <DashboardLayout>
        <span>Dashboard content</span>
      </DashboardLayout>,
    );

    expect(screen.getByTestId("dashboard-shell")).toHaveTextContent("Dashboard content");
  });
});
