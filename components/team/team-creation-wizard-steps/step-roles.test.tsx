jest.mock("next-intl", () => ({
  useTranslations: () => (
    key: string,
    values?: Record<string, number>,
  ) => {
    const map: Record<string, string> = {
      "wizard.loadingRoles": "Loading roles...",
      "wizard.noRoles": "No roles available.",
      "wizard.rolesHint": "Select role templates",
      "wizard.noDescription": "No description",
    };
    if (key === "wizard.skillCount") {
      return `${values?.count ?? 0} skills`;
    }
    return map[key] ?? key;
  },
}));

const fetchRolesMock = jest.fn();
const roleStoreState = {
  roles: [] as Array<Record<string, unknown>>,
  loading: false,
  fetchRoles: fetchRolesMock,
};

jest.mock("@/lib/stores/role-store", () => ({
  useRoleStore: () => roleStoreState,
}));

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { StepRoles } from "./step-roles";

describe("StepRoles", () => {
  beforeEach(() => {
    fetchRolesMock.mockReset();
    roleStoreState.roles = [];
    roleStoreState.loading = false;
  });

  it("shows loading and empty states based on the role store", () => {
    roleStoreState.loading = true;
    const { rerender } = render(<StepRoles selectedRoleIds={[]} onChange={jest.fn()} />);
    expect(screen.getByText("Loading roles...")).toBeInTheDocument();

    roleStoreState.loading = false;
    rerender(<StepRoles selectedRoleIds={[]} onChange={jest.fn()} />);
    expect(screen.getByText("No roles available.")).toBeInTheDocument();
  });

  it("fetches roles and toggles selected cards", async () => {
    const user = userEvent.setup();
    const onChange = jest.fn();
    roleStoreState.roles = [
      {
        metadata: {
          id: "frontend",
          name: "Frontend Developer",
          description: "Builds UI",
        },
        capabilities: {
          skills: [{ path: "skills/react" }],
          tools: ["Read"],
        },
      },
    ];

    render(<StepRoles selectedRoleIds={[]} onChange={onChange} />);

    expect(fetchRolesMock).not.toHaveBeenCalled();
    expect(screen.getByText("Select role templates")).toBeInTheDocument();
    expect(screen.getByText("Frontend Developer")).toBeInTheDocument();
    expect(screen.getByText("2 skills")).toBeInTheDocument();

    await user.click(screen.getByText("Frontend Developer"));
    expect(onChange).toHaveBeenCalledWith(["frontend"]);
  });
});
