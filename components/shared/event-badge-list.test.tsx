import { render, screen } from "@testing-library/react";
import { EventBadgeList } from "./event-badge-list";

describe("EventBadgeList", () => {
  it("renders each event as a badge", () => {
    render(<EventBadgeList events={["task.created", "review.requested"]} />);

    expect(screen.getByText("task.created")).toBeInTheDocument();
    expect(screen.getByText("review.requested")).toBeInTheDocument();
  });
});
