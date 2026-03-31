import { render } from "@testing-library/react";
import { DashboardWidgetsSkeleton } from "./dashboard-widget-skeletons";

describe("DashboardWidgetsSkeleton", () => {
  it("renders one skeleton shell for each primary dashboard widget", () => {
    const { container } = render(<DashboardWidgetsSkeleton />);

    expect(container.querySelectorAll('[data-testid="dashboard-widget-skeleton"]')).toHaveLength(4);
    expect(container.querySelectorAll('[data-slot="skeleton"]').length).toBeGreaterThanOrEqual(12);
  });
});
