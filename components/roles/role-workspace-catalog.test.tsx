jest.mock("next-intl", () => ({
  useTranslations: () => (key: string) => {
    const map: Record<string, string> = {
      roleLibrary: "Role Library",
      roleLibraryDesc: "Manage reusable roles",
      newRole: "New Role",
      loadingRoles: "Loading roles...",
      emptyLibrary: "No roles yet.",
    };
    return map[key] ?? key;
  },
}));

jest.mock("./role-card", () => ({
  RoleCard: ({
    role,
    onEdit,
    onDelete,
  }: {
    role: { metadata: { name: string } };
    onEdit: () => void;
    onDelete: () => void;
  }) => (
    <div>
      <span>{role.metadata.name}</span>
      <button type="button" onClick={onEdit}>
        Edit {role.metadata.name}
      </button>
      <button type="button" onClick={onDelete}>
        Delete {role.metadata.name}
      </button>
    </div>
  ),
}));

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { RoleWorkspaceCatalog } from "./role-workspace-catalog";

const frontendRole = {
  metadata: { id: "frontend", name: "Frontend Developer" },
} as never;

describe("RoleWorkspaceCatalog", () => {
  it("renders loading and empty states", () => {
    const { rerender } = render(
      <RoleWorkspaceCatalog
        roles={[]}
        skillCatalog={[]}
        loading
        error={null}
        onCreateNew={jest.fn()}
        onEditRole={jest.fn()}
        onDeleteRole={jest.fn()}
      />,
    );

    expect(screen.getByText("Loading roles...")).toBeInTheDocument();

    rerender(
      <RoleWorkspaceCatalog
        roles={[]}
        skillCatalog={[]}
        loading={false}
        error={null}
        onCreateNew={jest.fn()}
        onEditRole={jest.fn()}
        onDeleteRole={jest.fn()}
      />,
    );

    expect(screen.getByText("No roles yet.")).toBeInTheDocument();
  });

  it("renders header title and buttons in separate rows", () => {
    render(
      <RoleWorkspaceCatalog
        roles={[]}
        skillCatalog={[]}
        loading={false}
        error={null}
        onCreateNew={jest.fn()}
        onEditRole={jest.fn()}
        onDeleteRole={jest.fn()}
      />,
    );

    const title = screen.getByText("Role Library");
    const newRoleButton = screen.getByRole("button", { name: "New Role" });
    // Title and button must NOT share the same parent element — they are in separate rows
    expect(title.parentElement).not.toBe(newRoleButton.parentElement);
  });

  it("renders role cards and routes create, edit, and delete actions", async () => {
    const user = userEvent.setup();
    const onCreateNew = jest.fn();
    const onEditRole = jest.fn();
    const onDeleteRole = jest.fn();

    render(
      <RoleWorkspaceCatalog
        roles={[frontendRole]}
        skillCatalog={[]}
        loading={false}
        error="Catalog unavailable"
        onCreateNew={onCreateNew}
        onEditRole={onEditRole}
        onDeleteRole={onDeleteRole}
      />,
    );

    expect(screen.getByText("Role Library")).toBeInTheDocument();
    expect(screen.getByText("Catalog unavailable")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "New Role" }));
    await user.click(
      screen.getByRole("button", { name: "Edit Frontend Developer" }),
    );
    await user.click(
      screen.getByRole("button", { name: "Delete Frontend Developer" }),
    );

    expect(onCreateNew).toHaveBeenCalled();
    expect(onEditRole).toHaveBeenCalledWith(frontendRole);
    expect(onDeleteRole).toHaveBeenCalledWith(frontendRole);
  });
});
