import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { KnowledgeAssetTreeNode } from "@/lib/stores/knowledge-store";
import { DocsSidebarPanel } from "./docs-sidebar-panel";

const mockPageTree = jest.fn();

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

jest.mock("./page-tree", () => ({
  PageTree: (props: { nodes: KnowledgeAssetTreeNode[] }) => {
    mockPageTree(props);
    return (
      <div data-testid="page-tree">
        {props.nodes.map((node) => node.title).join(",")}
      </div>
    );
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

describe("DocsSidebarPanel", () => {
  beforeEach(() => {
    mockPageTree.mockClear();
  });

  it("filters the tree for the search query and renders favorites and recent links", async () => {
    const user = userEvent.setup();
    const onQueryChange = jest.fn();
    const tree = [
      makeNode({ id: "runbook", title: "Ops Runbook" }),
      makeNode({ id: "adr", title: "Architecture ADR" }),
    ];
    const favorites = [
      {
        assetId: "adr",
        userId: "user-1",
        createdAt: "2026-03-26T12:01:00.000Z",
      },
    ];
    const recentAccess = [
      {
        assetId: "runbook",
        userId: "user-1",
        accessedAt: "2026-03-26T12:02:00.000Z",
      },
    ];

    render(
      <DocsSidebarPanel
        query="run"
        onQueryChange={onQueryChange}
        tree={tree}
        currentPageId="runbook"
        favorites={favorites}
        recentAccess={recentAccess}
      />,
    );

    expect(screen.getByRole("link", { name: "Architecture ADR" })).toHaveAttribute(
      "href",
      "/docs?pageId=adr",
    );
    expect(screen.getByRole("link", { name: "Ops Runbook" })).toHaveAttribute(
      "href",
      "/docs?pageId=runbook",
    );
    expect(mockPageTree).toHaveBeenCalledWith(
      expect.objectContaining({
        nodes: [expect.objectContaining({ title: "Ops Runbook", children: [] })],
      }),
    );

    await user.type(screen.getByPlaceholderText("Find a page, template, or runbook"), "x");
    expect(onQueryChange).toHaveBeenCalledWith("runx");
  });

  it("shows empty states when there are no favorites or recent docs", () => {
    render(
      <DocsSidebarPanel
        query=""
        onQueryChange={jest.fn()}
        tree={[]}
        favorites={[]}
        recentAccess={[]}
      />,
    );

    expect(screen.getByText("No favorites yet.")).toBeInTheDocument();
    expect(screen.getByText("No recent docs yet.")).toBeInTheDocument();
  });
});
