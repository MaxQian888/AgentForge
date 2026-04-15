import { render, screen } from "@testing-library/react";
import { isPlaywrightHarnessEnabled } from "@/lib/playwright-harness";
import PlaywrightLayout from "./layout";

const notFoundMock = jest.fn(() => {
  throw new Error("NEXT_NOT_FOUND");
});

jest.mock("next/navigation", () => ({
  notFound: () => notFoundMock(),
}));

jest.mock("@/lib/playwright-harness", () => ({
  isPlaywrightHarnessEnabled: jest.fn(),
}));

describe("PlaywrightLayout", () => {
  beforeEach(() => {
    jest.clearAllMocks();
  });

  it("returns notFound when the harness is disabled", () => {
    (isPlaywrightHarnessEnabled as jest.Mock).mockReturnValue(false);

    expect(() =>
      render(
        <PlaywrightLayout>
          <div>Harness</div>
        </PlaywrightLayout>,
      ),
    ).toThrow("NEXT_NOT_FOUND");
    expect(notFoundMock).toHaveBeenCalled();
  });

  it("renders the internal navigation when the harness is enabled", () => {
    (isPlaywrightHarnessEnabled as jest.Mock).mockReturnValue(true);

    render(
      <PlaywrightLayout>
        <div>Harness</div>
      </PlaywrightLayout>,
    );

    expect(
      screen.getByRole("heading", { name: "Playwright Harness" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "Workflow Templates" }),
    ).toHaveAttribute("href", "/playwright/workflow-template");
  });
});
