import { render } from "@testing-library/react";
import { MetricCardSkeleton } from "./metric-card-skeleton";

describe("MetricCardSkeleton", () => {
  it("renders the metric shell with three skeleton blocks", () => {
    const { container } = render(
      <MetricCardSkeleton className="metric-skeleton" />,
    );

    expect(container.firstChild).toHaveClass("metric-skeleton");
    expect(container.querySelectorAll('[data-slot="skeleton"]')).toHaveLength(3);
  });
});
