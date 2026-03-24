import { render, screen } from "@testing-library/react";
import {
  Table,
  TableBody,
  TableCaption,
  TableCell,
  TableFooter,
  TableHead,
  TableHeader,
  TableRow,
} from "./table";

describe("Table", () => {
  it("renders header, body, footer, and caption slots", () => {
    const { container } = render(
      <Table className="min-w-80">
        <TableCaption>Agent costs</TableCaption>
        <TableHeader>
          <TableRow>
            <TableHead>Name</TableHead>
            <TableHead>Cost</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          <TableRow data-state="selected">
            <TableCell>Planner</TableCell>
            <TableCell>$12.00</TableCell>
          </TableRow>
        </TableBody>
        <TableFooter>
          <TableRow>
            <TableCell>Total</TableCell>
            <TableCell>$12.00</TableCell>
          </TableRow>
        </TableFooter>
      </Table>
    );

    expect(container.querySelector('[data-slot="table"]')).toHaveClass("min-w-80");
    expect(screen.getByText("Agent costs")).toHaveAttribute("data-slot", "table-caption");
    expect(screen.getByText("Name")).toHaveAttribute("data-slot", "table-head");
    expect(screen.getByText("Planner")).toHaveAttribute("data-slot", "table-cell");
    expect(container.querySelector('[data-slot="table-footer"]')).toBeInTheDocument();
    expect(container.querySelector('[data-slot="table-row"][data-state="selected"]')).toBeInTheDocument();
  });
});
