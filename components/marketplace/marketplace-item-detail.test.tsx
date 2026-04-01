import { render, screen } from "@testing-library/react";

jest.mock("react-markdown", () => ({
  __esModule: true,
  default: ({ children }: { children: unknown }) => children,
}));

jest.mock("remark-gfm", () => ({
  __esModule: true,
  default: () => null,
}));

jest.mock("@/lib/stores/marketplace-store", () => ({
  useMarketplaceStore: (selector: (state: Record<string, unknown>) => unknown) =>
    selector({
      selectedItemReviews: [],
      uploadVersion: jest.fn(),
      deleteItem: jest.fn(),
      verifyItem: jest.fn(),
      featureItem: jest.fn(),
      sideloadItem: jest.fn(),
      uninstallLoading: false,
      sideloadLoading: false,
    }),
}));

import { MarketplaceItemDetail } from "./marketplace-item-detail";

describe("MarketplaceItemDetail", () => {
  it("renders structured markdown and yaml preview for skill items", () => {
    render(
      <MarketplaceItemDetail
        item={{
          id: "react",
          type: "skill",
          slug: "react",
          sourceType: "builtin",
          name: "React",
          author_id: "agentforge",
          author_name: "AgentForge",
          description: "Build React surfaces.",
          category: "frontend",
          tags: ["react", "nextjs"],
          license: "MIT",
          extra_metadata: {
            docsRef: "docs/role-yaml.md",
          },
          download_count: 0,
          avg_rating: 0,
          rating_count: 0,
          is_verified: true,
          is_featured: true,
          created_at: "2024-01-01T00:00:00Z",
          updated_at: "2024-01-01T00:00:00Z",
          localPath: "D:/Project/AgentForge/skills/react",
          skillPreview: {
            canonicalPath: "skills/react",
            label: "React",
            displayName: "AgentForge React",
            description: "Build React surfaces.",
            defaultPrompt: "Use React skill",
            markdownBody: "# React\n\nBuild product-facing React surfaces.",
            frontmatterYaml:
              "name: React\ndescription: Build React surfaces.\nrequires:\n  - skills/typescript",
            requires: ["skills/typescript"],
            tools: ["code_editor", "browser_preview"],
            availableParts: ["agents", "references"],
            agentConfigs: [
              {
                path: "agents/openai.yaml",
                yaml: "interface:\n  display_name: AgentForge React",
                displayName: "AgentForge React",
              },
            ],
          },
        }}
        consumption={{
          itemId: "react",
          itemType: "skill",
          status: "installed",
          consumerSurface: "role-skill-catalog",
          installed: true,
          used: false,
          localPath: "D:/Project/AgentForge/skills/react",
          provenance: {
            sourceType: "builtin",
            marketplaceItemId: "react",
            localPath: "D:/Project/AgentForge/skills/react",
          },
        }}
      />,
    );

    expect(screen.getByText("Available locally")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Open role authoring" })).toBeInTheDocument();
    expect(screen.getByText("Skill package")).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "React" })).toBeInTheDocument();
    expect(screen.getByText("Frontmatter YAML")).toBeInTheDocument();
    expect(screen.getByText("Agent YAML")).toBeInTheDocument();
    expect(screen.getByText("agents/openai.yaml")).toBeInTheDocument();
    expect(screen.getByText("Docs reference:")).toBeInTheDocument();
  });

  it("renders uninstall button for non-builtin installed items", () => {
    render(
      <MarketplaceItemDetail
        item={{
          id: "plugin-1",
          type: "plugin",
          slug: "plugin-1",
          name: "Test Plugin",
          author_id: "user-1",
          author_name: "Author",
          description: "A plugin.",
          category: "testing",
          tags: [],
          license: "MIT",
          extra_metadata: {},
          download_count: 0,
          avg_rating: 0,
          rating_count: 0,
          is_verified: false,
          is_featured: false,
          created_at: "2024-01-01T00:00:00Z",
          updated_at: "2024-01-01T00:00:00Z",
        }}
        consumption={{
          itemId: "plugin-1",
          itemType: "plugin",
          status: "installed",
          consumerSurface: "plugin-management-panel",
          installed: true,
          used: true,
          provenance: {
            sourceType: "marketplace",
            marketplaceItemId: "plugin-1",
            selectedVersion: "1.0.0",
          },
        }}
      />,
    );

    expect(screen.getByRole("button", { name: "Uninstall" })).toBeInTheDocument();
  });

  it("does not render uninstall button for builtin items", () => {
    render(
      <MarketplaceItemDetail
        item={{
          id: "react",
          type: "skill",
          slug: "react",
          sourceType: "builtin",
          name: "React",
          author_id: "agentforge",
          author_name: "AgentForge",
          description: "React skill.",
          category: "frontend",
          tags: [],
          license: "MIT",
          extra_metadata: {},
          download_count: 0,
          avg_rating: 0,
          rating_count: 0,
          is_verified: false,
          is_featured: false,
          created_at: "2024-01-01T00:00:00Z",
          updated_at: "2024-01-01T00:00:00Z",
        }}
        consumption={{
          itemId: "react",
          itemType: "skill",
          status: "installed",
          consumerSurface: "role-skill-catalog",
          installed: true,
          used: false,
          provenance: {
            sourceType: "builtin",
            marketplaceItemId: "react",
          },
        }}
      />,
    );

    expect(screen.queryByRole("button", { name: "Uninstall" })).not.toBeInTheDocument();
  });

  it("renders update banner when updateInfo has update", () => {
    render(
      <MarketplaceItemDetail
        item={{
          id: "plugin-1",
          type: "plugin",
          slug: "plugin-1",
          name: "Test Plugin",
          author_id: "user-1",
          author_name: "Author",
          description: "A plugin.",
          category: "testing",
          tags: [],
          license: "MIT",
          extra_metadata: {},
          latest_version: "2.0.0",
          download_count: 0,
          avg_rating: 0,
          rating_count: 0,
          is_verified: false,
          is_featured: false,
          created_at: "2024-01-01T00:00:00Z",
          updated_at: "2024-01-01T00:00:00Z",
        }}
        updateInfo={{
          itemId: "plugin-1",
          itemType: "plugin",
          installedVersion: "1.0.0",
          latestVersion: "2.0.0",
          hasUpdate: true,
        }}
      />,
    );

    expect(
      screen.getByText((content) => content.includes("Update available") && content.includes("v2.0.0")),
    ).toBeInTheDocument();
  });

  it("renders sideload file upload for role items", () => {
    render(
      <MarketplaceItemDetail
        item={{
          id: "role-1",
          type: "role",
          slug: "role-1",
          name: "Test Role",
          author_id: "user-1",
          author_name: "Author",
          description: "A role.",
          category: "testing",
          tags: [],
          license: "MIT",
          extra_metadata: {},
          download_count: 0,
          avg_rating: 0,
          rating_count: 0,
          is_verified: false,
          is_featured: false,
          created_at: "2024-01-01T00:00:00Z",
          updated_at: "2024-01-01T00:00:00Z",
        }}
      />,
    );

    expect(
      screen.getByText("Upload a zip package containing a valid role.yaml at its root."),
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /Side-load role/ })).toBeInTheDocument();
  });
});
