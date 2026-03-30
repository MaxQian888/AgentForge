jest.mock("@/components/ui/select", () => ({
  Select: ({
    value,
    children,
  }: {
    value?: string;
    children?: React.ReactNode;
  }) => <div data-select-value={value}>{children}</div>,
  SelectTrigger: ({ children }: { children?: React.ReactNode }) => <>{children}</>,
  SelectValue: ({ placeholder }: { placeholder?: string }) => <span>{placeholder}</span>,
  SelectContent: ({ children }: { children?: React.ReactNode }) => <>{children}</>,
  SelectItem: ({
    value,
    children,
  }: {
    value: string;
    children?: React.ReactNode;
  }) => (
    <button type="button" onClick={() => {}}>
      {children ?? value}
    </button>
  ),
}));

import { fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { FilterBar } from "./filter-bar";

describe("FilterBar", () => {
  it("supports search, filter reset, and extra toolbar content", async () => {
    const user = userEvent.setup();
    const onSearch = jest.fn();
    const onFilterChange = jest.fn();
    const onReset = jest.fn();

    render(
      <FilterBar
        searchValue="queued"
        searchPlaceholder="Search tasks"
        onSearch={onSearch}
        onReset={onReset}
        filters={[
          {
            key: "status",
            label: "Status",
            value: "active",
            onChange: onFilterChange,
            options: [
              { value: "active", label: "Active" },
              { value: "blocked", label: "Blocked" },
            ],
          },
        ]}
      >
        <button type="button">Extra action</button>
      </FilterBar>,
    );

    fireEvent.change(screen.getByPlaceholderText("Search tasks"), {
      target: { value: "agent" },
    });
    expect(onSearch).toHaveBeenCalledWith("agent");

    expect(screen.getByText("Extra action")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Reset" }));
    expect(onReset).toHaveBeenCalled();
  });

  it("hides reset when there is no active search or filter", () => {
    render(
      <FilterBar
        searchValue=""
        onSearch={jest.fn()}
        filters={[
          {
            key: "status",
            label: "Status",
            value: "all",
            onChange: jest.fn(),
            options: [{ value: "active", label: "Active" }],
          },
        ]}
      />,
    );

    expect(screen.queryByRole("button", { name: "Reset" })).not.toBeInTheDocument();
  });
});
