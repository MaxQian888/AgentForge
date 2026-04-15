import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import ProjectsPage from "./page";

type Project = import("@/lib/stores/project-store").Project;

function createMockProject(
  overrides: Partial<Project> & Pick<Project, "id" | "name">,
): Project {
  const { id, name, ...rest } = overrides;
  return {
    id,
    name,
    description: "",
    status: "active",
    taskCount: 0,
    agentCount: 0,
    createdAt: "2026-03-24T12:00:00.000Z",
    settings: {
      codingAgent: {
        runtime: "",
        provider: "",
        model: "",
      },
    },
    ...rest,
  };
}

const searchParamsState = {
  action: null as string | null,
};

const replace = jest.fn();
const fetchProjects = jest.fn();
const createProject = jest.fn();
const updateProject = jest.fn();
const deleteProject = jest.fn();

const projectState = {
  projects: [] as Project[],
  currentProject: null as Project | null,
  loading: false,
  fetchProjects,
  setCurrentProject: jest.fn(),
  createProject,
  updateProject,
  deleteProject,
};

jest.mock("next/navigation", () => ({
  usePathname: () => "/projects",
  useRouter: () => ({ replace }),
  useSearchParams: () => ({
    get: (key: string) => (key === "action" ? searchParamsState.action : null),
    toString: () =>
      searchParamsState.action
        ? `action=${encodeURIComponent(searchParamsState.action)}`
        : "",
  }),
}));

jest.mock("@/lib/stores/project-store", () => ({
  useProjectStore: (selector?: (state: typeof projectState) => unknown) =>
    selector ? selector(projectState) : projectState,
}));

jest.mock("@/components/shared/filter-bar", () => ({
  FilterBar: ({
    onSearch,
    onReset,
  }: {
    onSearch: (value: string) => void;
    onReset: () => void;
  }) => (
    <div>
      <button type="button" onClick={() => onSearch("bridge")}>
        search-bridge
      </button>
      <button type="button" onClick={onReset}>
        reset-search
      </button>
    </div>
  ),
}));

jest.mock("@/components/project/project-card", () => ({
  ProjectCard: ({
    project,
    onEdit,
    onDelete,
  }: {
    project: { id: string; name: string };
    onEdit: (project: { id: string; name: string }) => void;
    onDelete: (id: string) => void;
  }) => (
    <div>
      <span>{project.name}</span>
      <button type="button" onClick={() => onEdit(project)}>
        edit-{project.id}
      </button>
      <button type="button" onClick={() => onDelete(project.id)}>
        delete-{project.id}
      </button>
    </div>
  ),
}));

jest.mock("@/components/project/edit-project-dialog", () => ({
  EditProjectDialog: ({
    project,
    onSave,
    onClose,
  }: {
    project: { id: string; name: string };
    onSave: (id: string, input: { name: string }) => Promise<void>;
    onClose: () => void;
  }) => (
    <div data-testid="edit-project-dialog">
      <button type="button" onClick={() => void onSave(project.id, { name: `${project.name} Updated` })}>
        save-edit
      </button>
      <button type="button" onClick={onClose}>
        close-edit
      </button>
    </div>
  ),
}));

describe("ProjectsPage", () => {
  beforeEach(() => {
    searchParamsState.action = null;
    replace.mockReset();
    fetchProjects.mockReset();
    createProject.mockReset().mockResolvedValue(undefined);
    updateProject.mockReset().mockResolvedValue(undefined);
    deleteProject.mockReset();
    projectState.projects = [];
    projectState.loading = false;
  });

  it("ignores the legacy create route action until the user opens the dialog", () => {
    searchParamsState.action = "create";

    render(<ProjectsPage />);

    expect(
      screen.queryByText("Set up the project name and purpose before tasks, members, and agents are attached.")
    ).not.toBeInTheDocument();
  });

  it("includes a dialog description for the create project modal", async () => {
    const user = userEvent.setup();
    render(<ProjectsPage />);

    await user.click(screen.getByRole("button", { name: "New Project" }));

    await waitFor(() => {
      expect(
        screen.getByText("Set up the project name and purpose before tasks, members, and agents are attached.")
      ).toBeInTheDocument();
    });
  });

  it("submits the create-project dialog and fetches projects on mount", async () => {
    const user = userEvent.setup();
    createProject.mockResolvedValueOnce({ id: "project-3" });
    render(<ProjectsPage />);

    expect(fetchProjects).toHaveBeenCalledTimes(1);

    await user.click(screen.getByRole("button", { name: "New Project" }));
    await user.type(screen.getByLabelText("Name"), "Bridge");
    await user.type(screen.getByLabelText("Description"), "Sidecar workspace");
    await user.click(screen.getByRole("button", { name: "Create" }));

    await waitFor(() => {
      expect(createProject).toHaveBeenCalledWith({
        name: "Bridge",
        description: "Sidecar workspace",
      });
    });
    expect(replace).toHaveBeenCalledWith("/?project=project-3");
  });

  it("filters, edits, and deletes projects through the page callbacks", async () => {
    const user = userEvent.setup();
    projectState.projects = [
      createMockProject({
        id: "project-1",
        name: "AgentForge",
        taskCount: 5,
        agentCount: 2,
      }),
      createMockProject({
        id: "project-2",
        name: "Bridge",
        status: "paused",
        taskCount: 2,
        agentCount: 1,
      }),
    ];

    render(<ProjectsPage />);

    expect(screen.getByText("AgentForge")).toBeInTheDocument();
    expect(screen.getByText("Bridge")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "search-bridge" }));
    expect(screen.queryByText("AgentForge")).not.toBeInTheDocument();
    expect(screen.getByText("Bridge")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "edit-project-2" }));
    expect(screen.getByTestId("edit-project-dialog")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "save-edit" }));
    expect(updateProject).toHaveBeenCalledWith("project-2", { name: "Bridge Updated" });

    await user.click(screen.getByRole("button", { name: "close-edit" }));
    expect(screen.queryByTestId("edit-project-dialog")).not.toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "delete-project-2" }));
    expect(deleteProject).toHaveBeenCalledWith("project-2");

    await user.click(screen.getByRole("button", { name: "reset-search" }));
    expect(screen.getByText("AgentForge")).toBeInTheDocument();
    expect(screen.getByText("Bridge")).toBeInTheDocument();
  });
});
