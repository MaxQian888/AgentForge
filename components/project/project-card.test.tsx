jest.mock("next/link", () => ({
  __esModule: true,
  default: ({
    href,
    children,
    ...props
  }: React.AnchorHTMLAttributes<HTMLAnchorElement> & { href: string }) => (
    <a href={href} {...props}>
      {children}
    </a>
  ),
}));

jest.mock("next-intl", () => ({
  useTranslations: () => (
    key: string,
    values?: Record<string, string | number>,
  ) => {
    const map: Record<string, string> = {
      "status.active": "Active",
      noDescription: "No description",
      "deleteProject.title": "Delete project",
      "deleteProject.confirm": "Delete project",
    };
    if (key === "taskCount") {
      return `Tasks: ${values?.count ?? 0}`;
    }
    if (key === "agentCount") {
      return `Agents: ${values?.count ?? 0}`;
    }
    if (key === "card.created") {
      return `Created ${values?.date ?? ""}`;
    }
    if (key === "deleteProject.description") {
      return `Delete ${values?.name ?? ""}`;
    }
    return map[key] ?? key;
  },
}));

jest.mock("@/components/shared/confirm-dialog", () => ({
  ConfirmDialog: ({
    open,
    title,
    description,
    confirmLabel,
    onConfirm,
    onCancel,
  }: {
    open: boolean;
    title: string;
    description: string;
    confirmLabel: string;
    onConfirm: () => void;
    onCancel: () => void;
  }) =>
    open ? (
      <div data-testid="confirm-dialog">
        <div>{title}</div>
        <div>{description}</div>
        <button type="button" onClick={onConfirm}>
          {confirmLabel}
        </button>
        <button type="button" onClick={onCancel}>
          Close dialog
        </button>
      </div>
    ) : null,
}));

import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ProjectCard } from "./project-card";
import type { Project } from "@/lib/stores/project-store";

const project: Project = {
  id: "project-1",
  name: "Release Ops",
  description: "",
  status: "active",
  taskCount: 12,
  agentCount: 4,
  createdAt: "2026-03-30T00:00:00.000Z",
  repoUrl: "https://github.com/acme/release-ops",
  defaultBranch: "main",
  settings: {
    codingAgent: {
      runtime: "codex",
      provider: "openai",
      model: "gpt-5.4",
    },
  },
};

describe("ProjectCard", () => {
  it("renders project metadata and links to the project bootstrap dashboard", () => {
    render(
      <ProjectCard project={project} onEdit={jest.fn()} onDelete={jest.fn()} />,
    );

    expect(screen.getByRole("link")).toHaveAttribute(
      "href",
      "/?project=project-1",
    );
    expect(screen.getByText("Release Ops")).toBeInTheDocument();
    expect(screen.getByText("No description")).toBeInTheDocument();
    expect(screen.getByText("Tasks: 12")).toBeInTheDocument();
    expect(screen.getByText("Agents: 4")).toBeInTheDocument();
  });

  it("routes edit and delete actions through their callbacks", async () => {
    const user = userEvent.setup();
    const onEdit = jest.fn();
    const onDelete = jest.fn();

    const { container } = render(
      <ProjectCard project={project} onEdit={onEdit} onDelete={onDelete} />,
    );

    const actionButtons = container.querySelectorAll("button");
    expect(actionButtons).toHaveLength(2);

    await user.click(actionButtons[0]);
    expect(onEdit).toHaveBeenCalledWith(project);

    await user.click(actionButtons[1]);
    const dialog = screen.getByTestId("confirm-dialog");
    expect(within(dialog).getByText("Delete Release Ops")).toBeInTheDocument();

    await user.click(
      within(dialog).getByRole("button", { name: "Delete project" }),
    );
    expect(onDelete).toHaveBeenCalledWith("project-1");
  });
});
