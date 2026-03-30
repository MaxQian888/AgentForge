import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ViewSwitcher } from "./view-switcher";

const fetchViewsMock = jest.fn();
const selectViewMock = jest.fn();
const setDefaultViewMock = jest.fn();
const applySavedViewConfigMock = jest.fn();
const saveViewDialogMock = jest.fn();
const viewShareDialogMock = jest.fn();

const savedViewState = {
  fetchViews: fetchViewsMock,
  viewsByProject: {} as Record<string, Array<{ id: string; name: string; config: unknown }>>,
  currentViewByProject: {} as Record<string, string | null>,
  selectView: selectViewMock,
  setDefaultView: setDefaultViewMock,
};

const taskWorkspaceState = {
  applySavedViewConfig: applySavedViewConfigMock,
  viewMode: "board",
  filters: {
    status: "todo",
    priority: "high",
    assigneeId: "agent-1",
    sprintId: "sprint-1",
    search: "triage",
  },
};

jest.mock("@/lib/stores/saved-view-store", () => ({
  useSavedViewStore: (selector: (state: typeof savedViewState) => unknown) =>
    selector(savedViewState),
}));

jest.mock("@/lib/stores/task-workspace-store", () => ({
  useTaskWorkspaceStore: (
    selector: (state: typeof taskWorkspaceState) => unknown,
  ) => selector(taskWorkspaceState),
}));

jest.mock("./save-view-dialog", () => ({
  SaveViewDialog: (props: {
    open: boolean;
    projectId: string;
    config: unknown;
    onOpenChange: (open: boolean) => void;
  }) => {
    saveViewDialogMock(props);
    return props.open ? <div data-testid="save-view-dialog" /> : null;
  },
}));

jest.mock("./view-share-dialog", () => ({
  ViewShareDialog: (props: {
    open: boolean;
    projectId: string;
    view: unknown;
    onOpenChange: (open: boolean) => void;
  }) => {
    viewShareDialogMock(props);
    return props.open ? <div data-testid="view-share-dialog" /> : null;
  },
}));

describe("ViewSwitcher", () => {
  beforeEach(() => {
    fetchViewsMock.mockReset();
    fetchViewsMock.mockResolvedValue(undefined);
    selectViewMock.mockReset();
    setDefaultViewMock.mockReset();
    setDefaultViewMock.mockResolvedValue(undefined);
    applySavedViewConfigMock.mockReset();
    saveViewDialogMock.mockClear();
    viewShareDialogMock.mockClear();
    savedViewState.viewsByProject = {
      "project-1": [
        { id: "view-1", name: "Board", config: { layout: "board" } },
        { id: "view-2", name: "List", config: { layout: "list" } },
      ],
    };
    savedViewState.currentViewByProject = {
      "project-1": "view-1",
    };
  });

  it("fetches views on mount and applies a selected saved view", async () => {
    const user = userEvent.setup();

    render(<ViewSwitcher projectId="project-1" />);

    await waitFor(() => {
      expect(fetchViewsMock).toHaveBeenCalledWith("project-1");
    });

    await user.selectOptions(screen.getByRole("combobox"), "view-2");

    expect(selectViewMock).toHaveBeenCalledWith("project-1", "view-2");
    expect(applySavedViewConfigMock).toHaveBeenCalledWith({ layout: "list" });
  });

  it("opens save and share dialogs and forwards the current view config", async () => {
    const user = userEvent.setup();

    render(<ViewSwitcher projectId="project-1" />);

    expect(screen.getByRole("button", { name: "Share" })).toBeEnabled();
    expect(screen.getByRole("button", { name: "Set Default" })).toBeEnabled();
    expect(saveViewDialogMock).toHaveBeenLastCalledWith(
      expect.objectContaining({
        open: false,
        projectId: "project-1",
        config: {
          layout: "board",
          filters: [
            { field: "status", op: "eq", value: "todo" },
            { field: "priority", op: "eq", value: "high" },
            { field: "assigneeId", op: "eq", value: "agent-1" },
            { field: "sprintId", op: "eq", value: "sprint-1" },
            { field: "search", op: "contains", value: "triage" },
          ],
        },
      }),
    );

    await user.click(screen.getByRole("button", { name: "Save View" }));
    expect(await screen.findByTestId("save-view-dialog")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Share" }));
    expect(await screen.findByTestId("view-share-dialog")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Set Default" }));
    expect(setDefaultViewMock).toHaveBeenCalledWith("project-1", "view-1");
  });

  it("keeps share and default actions disabled when no saved view is selected", () => {
    savedViewState.currentViewByProject = {
      "project-1": null,
    };

    render(<ViewSwitcher projectId="project-1" />);

    expect(screen.getByRole("button", { name: "Share" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "Set Default" })).toBeDisabled();
    expect(viewShareDialogMock).toHaveBeenLastCalledWith(
      expect.objectContaining({
        view: null,
      }),
    );
  });
});
