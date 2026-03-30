import { render } from "@testing-library/react";
import { SidebarMenuSkeleton } from "./sidebar";

describe("SidebarMenuSkeleton", () => {
  afterEach(() => {
    jest.restoreAllMocks();
  });

  it("does not call Math.random during render", () => {
    const randomSpy = jest.spyOn(Math, "random");

    render(<SidebarMenuSkeleton />);

    expect(randomSpy).not.toHaveBeenCalled();
  });
});
