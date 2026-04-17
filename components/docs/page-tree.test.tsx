import { render, screen } from "@testing-library/react";
import type { KnowledgeAssetTreeNode } from "@/lib/stores/knowledge-store";
import { PageTree } from "./page-tree";

const mockPageTreeItem = jest.fn();

jest.mock("./page-tree-item", () => ({
  PageTreeItem: (props: { node: KnowledgeAssetTreeNode; currentPageId?: string | null }) => {
    mockPageTreeItem(props);
    return <div data-testid={`page-tree-item-${props.node.id}`}>{props.node.title}</div>;
  },
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

describe("PageTree", () => {
  beforeEach(() => {
    mockPageTreeItem.mockClear();
  });

  it("renders one PageTreeItem per node and forwards callbacks", () => {
    const onMovePage = jest.fn();
    const onToggleFavorite = jest.fn();
    const onTogglePinned = jest.fn();
    const onDeletePage = jest.fn();
    const nodes = [
      makeNode({ id: "page-1", title: "Runbook" }),
      makeNode({ id: "page-2", title: "ADR" }),
    ];

    render(
      <PageTree
        nodes={nodes}
        currentPageId="page-2"
        onMovePage={onMovePage}
        onToggleFavorite={onToggleFavorite}
        onTogglePinned={onTogglePinned}
        onDeletePage={onDeletePage}
      />,
    );

    expect(screen.getByTestId("page-tree-item-page-1")).toBeInTheDocument();
    expect(screen.getByTestId("page-tree-item-page-2")).toBeInTheDocument();
    expect(mockPageTreeItem).toHaveBeenNthCalledWith(
      2,
      expect.objectContaining({
        node: nodes[1],
        currentPageId: "page-2",
        onMove: onMovePage,
        onToggleFavorite,
        onTogglePinned,
        onDelete: onDeletePage,
      }),
    );
  });

  it("shows an empty state when there are no pages", () => {
    render(<PageTree nodes={[]} />);

    expect(
      screen.getByText("No pages yet. Create the first project doc to start the tree."),
    ).toBeInTheDocument();
  });
});
