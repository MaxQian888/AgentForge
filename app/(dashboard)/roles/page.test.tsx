import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import RolesPage from "./page";

const roleStoreState = {
  roles: [{ metadata: { id: "role-1" } }],
  skillCatalog: [{ id: "skill-1" }],
  loading: false,
  skillCatalogLoading: false,
  error: null as string | null,
  fetchRoles: jest.fn(),
  fetchSkillCatalog: jest.fn(),
  createRole: jest.fn().mockResolvedValue(undefined),
  updateRole: jest.fn().mockResolvedValue(undefined),
  deleteRole: jest.fn().mockResolvedValue(undefined),
  previewRole: jest.fn(),
  sandboxRole: jest.fn(),
};

jest.mock("@/hooks/use-breadcrumbs", () => ({
  useBreadcrumbs: jest.fn(),
}));

jest.mock("@/lib/stores/role-store", () => ({
  useRoleStore: () => roleStoreState,
}));

jest.mock("@/components/roles/role-workspace", () => ({
  RoleWorkspace: ({
    roles,
    skillCatalog,
    onCreateRole,
    onDeleteRole,
  }: {
    roles: Array<{ metadata: { id: string } }>;
    skillCatalog: Array<{ id: string }>;
    onCreateRole: (input: { name: string }) => Promise<void>;
    onDeleteRole: (role: { metadata: { id: string } }) => Promise<void>;
  }) => (
    <div>
      <div data-testid="roles-count">{roles.length}</div>
      <div data-testid="skill-count">{skillCatalog.length}</div>
      <button type="button" onClick={() => void onCreateRole({ name: "Operations" })}>
        create-role
      </button>
      <button
        type="button"
        onClick={() => void onDeleteRole({ metadata: { id: "role-1" } })}
      >
        delete-role
      </button>
    </div>
  ),
}));

describe("RolesPage", () => {
  beforeEach(() => {
    roleStoreState.fetchRoles.mockReset();
    roleStoreState.fetchSkillCatalog.mockReset();
    roleStoreState.createRole.mockReset().mockResolvedValue(undefined);
    roleStoreState.deleteRole.mockReset().mockResolvedValue(undefined);
  });

  it("loads the roles and skill catalog on mount", () => {
    render(<RolesPage />);

    expect(roleStoreState.fetchRoles).toHaveBeenCalledTimes(1);
    expect(roleStoreState.fetchSkillCatalog).toHaveBeenCalledTimes(1);
    expect(screen.getByTestId("roles-count")).toHaveTextContent("1");
    expect(screen.getByTestId("skill-count")).toHaveTextContent("1");
  });

  it("wires create and delete callbacks through the role workspace", async () => {
    const user = userEvent.setup();

    render(<RolesPage />);

    await user.click(screen.getByRole("button", { name: "create-role" }));
    await user.click(screen.getByRole("button", { name: "delete-role" }));

    expect(roleStoreState.createRole).toHaveBeenCalledWith({ name: "Operations" });
    expect(roleStoreState.deleteRole).toHaveBeenCalledWith("role-1");
  });
});
