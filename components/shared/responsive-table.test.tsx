import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { BreakpointState } from "@/hooks/use-breakpoint";
import { ResponsiveTable } from "./responsive-table";

const breakpointState: BreakpointState = {
  breakpoint: "desktop",
  isDesktop: true,
  isTablet: false,
  isMobile: false,
};

jest.mock("@/hooks/use-breakpoint", () => ({
  useBreakpoint: () => breakpointState,
}));

describe("ResponsiveTable", () => {
  const columns = [
    {
      key: "name",
      header: "Name",
      renderCell: (row: { name: string }) => row.name,
    },
    {
      key: "status",
      header: "Status",
      renderCell: (row: { status: string }) => row.status,
    },
    {
      key: "owner",
      header: "Owner",
      renderCell: (row: { owner: string }) => row.owner,
      hideOnTablet: true,
    },
  ];

  const data = [
    { id: "task-1", name: "Ship review queue", status: "Running", owner: "Alex" },
  ];

  afterEach(() => {
    breakpointState.breakpoint = "desktop";
    breakpointState.isDesktop = true;
    breakpointState.isTablet = false;
    breakpointState.isMobile = false;
  });

  it("renders a full table on desktop", () => {
    render(
      <ResponsiveTable
        columns={columns}
        data={data}
        getRowId={(row) => row.id}
      />,
    );

    expect(screen.getByRole("table")).toBeInTheDocument();
    expect(screen.getByRole("columnheader", { name: "Owner" })).toBeInTheDocument();
    expect(screen.queryByText("Show more")).not.toBeInTheDocument();
  });

  it("renders stacked cards on mobile", () => {
    breakpointState.breakpoint = "mobile";
    breakpointState.isDesktop = false;
    breakpointState.isTablet = false;
    breakpointState.isMobile = true;

    render(
      <ResponsiveTable
        columns={columns}
        data={data}
        getRowId={(row) => row.id}
      />,
    );

    expect(screen.queryByRole("table")).not.toBeInTheDocument();
    expect(screen.getByText("Ship review queue")).toBeInTheDocument();
    expect(screen.getByText("Owner")).toBeInTheDocument();
    expect(screen.getByText("Alex")).toBeInTheDocument();
  });

  it("collapses secondary columns on tablet until expanded", async () => {
    const user = userEvent.setup();
    breakpointState.breakpoint = "tablet";
    breakpointState.isDesktop = false;
    breakpointState.isTablet = true;
    breakpointState.isMobile = false;

    render(
      <ResponsiveTable
        columns={columns}
        data={data}
        getRowId={(row) => row.id}
      />,
    );

    expect(screen.queryByRole("columnheader", { name: "Owner" })).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Show more" })).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Show more" }));

    expect(screen.getByText("Owner")).toBeInTheDocument();
    expect(screen.getByText("Alex")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Show less" })).toBeInTheDocument();
  });
});
