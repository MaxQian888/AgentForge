import { render, screen } from "@testing-library/react";
import {
  Card,
  CardAction,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from "./card";

describe("Card", () => {
  it("renders the full card layout with semantic slots", () => {
    const { container } = render(
      <Card className="shadow-lg">
        <CardHeader className="border-b">
          <CardTitle>Agent Status</CardTitle>
          <CardDescription>Live execution summary</CardDescription>
          <CardAction>Refresh</CardAction>
        </CardHeader>
        <CardContent>Active agents: 3</CardContent>
        <CardFooter>Updated just now</CardFooter>
      </Card>
    );

    expect(container.querySelector('[data-slot="card"]')).toHaveClass("shadow-lg");
    expect(screen.getByText("Agent Status")).toHaveAttribute("data-slot", "card-title");
    expect(screen.getByText("Live execution summary")).toHaveAttribute(
      "data-slot",
      "card-description",
    );
    expect(screen.getByText("Refresh")).toHaveAttribute("data-slot", "card-action");
    expect(screen.getByText("Active agents: 3")).toHaveAttribute(
      "data-slot",
      "card-content",
    );
    expect(screen.getByText("Updated just now")).toHaveAttribute(
      "data-slot",
      "card-footer",
    );
  });
});
