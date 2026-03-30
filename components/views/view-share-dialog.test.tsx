import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ViewShareDialog } from "./view-share-dialog";

const updateViewMock = jest.fn();

jest.mock("@/lib/stores/saved-view-store", () => ({
  useSavedViewStore: (
    selector: (state: { updateView: typeof updateViewMock }) => unknown,
  ) =>
    selector({
      updateView: updateViewMock,
    }),
}));

describe("ViewShareDialog", () => {
  beforeEach(() => {
    updateViewMock.mockReset();
  });

  it("renders nothing when there is no selected view", () => {
    const { container } = render(
      <ViewShareDialog
        open
        onOpenChange={jest.fn()}
        projectId="project-1"
        view={null}
      />,
    );

    expect(container).toBeEmptyDOMElement();
  });

  it("parses comma-separated roles and members before saving", async () => {
    const user = userEvent.setup();
    const onOpenChange = jest.fn();
    updateViewMock.mockResolvedValue(undefined);

    render(
      <ViewShareDialog
        open
        onOpenChange={onOpenChange}
        projectId="project-1"
        view={{
          id: "view-1",
          projectId: "project-1",
          name: "Review board",
          isDefault: false,
          sharedWith: {},
          config: {},
          createdAt: "2026-03-30T00:00:00.000Z",
          updatedAt: "2026-03-30T00:00:00.000Z",
        }}
      />,
    );

    await user.type(
      screen.getByPlaceholderText("reviewer, lead"),
      "reviewer, lead, ",
    );
    await user.type(
      screen.getByPlaceholderText("member-1, member-2"),
      "member-1, member-2",
    );
    await user.click(screen.getByRole("button", { name: "Save sharing" }));

    await waitFor(() => {
      expect(updateViewMock).toHaveBeenCalledWith("project-1", "view-1", {
        sharedWith: {
          roleIds: ["reviewer", "lead"],
          memberIds: ["member-1", "member-2"],
        },
      });
    });
    expect(onOpenChange).toHaveBeenCalledWith(false);
  });
});
