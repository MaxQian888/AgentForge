import { render, screen } from "@testing-library/react";
import { SettingsLayout } from "./settings-layout";

const pageHeaderMock = jest.fn();

jest.mock("@/components/shared/page-header", () => ({
  PageHeader: (props: {
    title: string;
    description?: string;
    sticky?: boolean;
    actions?: React.ReactNode;
  }) => {
    pageHeaderMock(props);
    return (
      <div data-testid="page-header" data-sticky={props.sticky ? "true" : "false"}>
        <span>{props.title}</span>
        {props.actions}
      </div>
    );
  },
}));

describe("SettingsLayout", () => {
  beforeEach(() => {
    pageHeaderMock.mockClear();
  });

  it("keeps the header sticky and shows the save bar when the form is dirty", () => {
    render(
      <SettingsLayout
        title="Settings"
        description="Configure project defaults."
        dirty
        saveBar={<button type="button">Save changes</button>}
      >
        <div>Settings form</div>
      </SettingsLayout>,
    );

    expect(pageHeaderMock).toHaveBeenCalledWith(
      expect.objectContaining({
        title: "Settings",
        sticky: true,
      }),
    );
    expect(screen.getByText("Settings form")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Save changes" })).toBeInTheDocument();
  });

  it("omits the save bar when the page is clean", () => {
    render(
      <SettingsLayout title="Settings">
        <div>Settings form</div>
      </SettingsLayout>,
    );

    expect(screen.queryByRole("button", { name: "Save changes" })).not.toBeInTheDocument();
  });
});
