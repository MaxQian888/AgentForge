import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import ProjectsPage from "./page";

const fetchProjects = jest.fn();
const createProject = jest.fn();

const projectState = {
  projects: [],
  currentProject: null,
  loading: false,
  fetchProjects,
  setCurrentProject: jest.fn(),
  createProject,
};

jest.mock("@/lib/stores/project-store", () => ({
  useProjectStore: (selector?: (state: typeof projectState) => unknown) =>
    selector ? selector(projectState) : projectState,
}));

describe("ProjectsPage", () => {
  beforeEach(() => {
    fetchProjects.mockReset();
    createProject.mockReset();
  });

  it("includes a dialog description for the create project modal", async () => {
    const user = userEvent.setup();
    render(<ProjectsPage />);

    await user.click(screen.getByRole("button", { name: "New Project" }));

    expect(
      screen.getByText("Set up the project name and purpose before tasks, members, and agents are attached.")
    ).toBeInTheDocument();
  });
});
