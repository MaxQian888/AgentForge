import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectLabel,
  SelectSeparator,
  SelectTrigger,
  SelectValue,
} from "./select";

describe("Select", () => {
  it("opens grouped options and updates the selected value", async () => {
    const user = userEvent.setup();
    const handleChange = jest.fn();

    render(
      <Select defaultValue="review" onValueChange={handleChange}>
        <SelectTrigger size="sm" aria-label="Status">
          <SelectValue placeholder="Choose status" />
        </SelectTrigger>
        <SelectContent position="popper">
          <SelectGroup>
            <SelectLabel>Status</SelectLabel>
            <SelectItem value="review">In review</SelectItem>
            <SelectSeparator />
            <SelectItem value="done">Done</SelectItem>
          </SelectGroup>
        </SelectContent>
      </Select>
    );

    await user.click(screen.getByRole("combobox", { name: "Status" }));

    expect(await screen.findByText("Status")).toHaveAttribute("data-slot", "select-label");
    expect(screen.getByRole("option", { name: "In review" })).toHaveAttribute(
      "data-slot",
      "select-item",
    );
    expect(screen.getByRole("option", { name: "Done" })).toHaveAttribute(
      "data-slot",
      "select-item",
    );

    await user.click(screen.getByRole("option", { name: "Done" }));

    expect(handleChange).toHaveBeenCalledWith("done");
    expect(screen.getByRole("combobox", { name: "Status" })).toHaveTextContent("Done");
  });
});
