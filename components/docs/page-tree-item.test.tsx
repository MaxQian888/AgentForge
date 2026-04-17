import { fireEvent, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { KnowledgeAssetTreeNode } from "@/lib/stores/knowledge-store";
import { PageTreeItem } from "./page-tree-item";

jest.mock("next/link", () => ({
  __esModule: true,
  default: ({
    href,
    children,
    ...props
  }: {
    href: string;
    children: React.ReactNode;
  }) => (
    <a href={href} {...props}>
      {children}
    </a>
  ),
}));

function makeNode(overrides: Partial<KnowledgeAssetTreeNode> = {}): KnowledgeAssetTreeNode {
  return {
    id: "page-1",
    projectId: "project-1",
    kind: "wiki_page",
    spaceId: "space-1",
    parentId: null,
    title: "Runbook",
    contentJson: "[]",
    contentText: "",
    path: "/runbook",
    sortOrder: 0,
    templateCategory: undefined,
    isPinned: false,
    createdBy: "user-1",
    updatedBy: "user-1",
    createdAt: "2026-03-26T12:00:00.000Z",
    updatedAt: "2026-03-26T12:00:00.000Z",
    deletedAt: null,
    version: 1,
    children: [],
    ...overrides,
  };
}

describe("PageTreeItem", () => {
  it("handles expand, drag-drop, and item actions", async () => {
    const user = userEvent.setup();
    const onMove = jest.fn();
    const onToggleFavorite = jest.fn();
    const onTogglePinned = jest.fn();
    const onDelete = jest.fn();
    const dataTransfer = {
      getData: jest.fn().mockReturnValue("dragged-page"),
      setData: jest.fn(),
    };

    render(
      <PageTreeItem
        node={makeNode({
          id: "page-1",
          sortOrder: 4,
          children: [makeNode({ id: "page-2", title: "Child page", parentId: "page-1" })],
        })}
        currentPageId="page-1"
        onMove={onMove}
        onToggleFavorite={onToggleFavorite}
        onTogglePinned={onTogglePinned}
        onDelete={onDelete}
      />,
    );

    const row = screen.getByRole("link", { name: "Runbook" }).closest("div");
    expect(row).toHaveClass("bg-accent");

    const buttons = within(row as HTMLElement).getAllByRole("button");
    await user.click(buttons[0]);
    expect(screen.queryByRole("link", { name: "Child page" })).not.toBeInTheDocument();

    await user.click(buttons[1]);
    await user.click(buttons[2]);
    await user.click(buttons[3]);

    expect(onToggleFavorite).toHaveBeenCalledWith("page-1", true);
    expect(onTogglePinned).toHaveBeenCalledWith("page-1", true);
    expect(onDelete).toHaveBeenCalledWith("page-1");

    const container = screen.getByRole("link", { name: "Runbook" }).closest("[draggable='true']");
    fireEvent.dragStart(container as HTMLElement, { dataTransfer });
    expect(dataTransfer.setData).toHaveBeenCalledWith("text/page-id", "page-1");

    fireEvent.drop(container as HTMLElement, { dataTransfer });
    expect(onMove).toHaveBeenCalledWith("dragged-page", null, 4);
  });
});
