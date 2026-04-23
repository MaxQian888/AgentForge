import { render, screen } from "@testing-library/react";

jest.mock("react-markdown", () => ({
  __esModule: true,
  default: ({ children }: { children: unknown }) => children,
}));

jest.mock("remark-gfm", () => ({
  __esModule: true,
  default: () => null,
}));

jest.mock("next-intl", () => ({
  useTranslations: () => (key: string, values?: Record<string, string | number>) => {
    const map: Record<string, string> = {
      "item.byAuthor": "by {author}",
      "item.verified": "Verified",
      "item.featured": "Featured",
      "item.repository": "Repository",
      "item.noDescription": "No description provided.",
      "item.blocked": "Blocked",
      "item.install": "Install",
      "update.banner": "Update available: v{latest} (installed: v{installed})",
      "update.button": "Update",
      "uninstall.button": "Uninstall",
      "uninstall.removing": "Removing...",
      "uninstall.confirm": "Uninstall {name}?",
      "uninstall.confirmDesc": "This will remove the installed files and consumption state. This action cannot be undone.",
      "install.cancel": "Cancel",
      "detail.status.notInstalled": "Not installed yet. Install it from the marketplace or use a supported local side-load flow.",
      "detail.status.manage": "Manage in workspace",
      "detail.status.available": "Available locally",
      "detail.status.installed": "Installed",
      "detail.status.needsAttention": "Needs attention",
      "detail.status.blocked": "Blocked",
      "detail.status.detailManaged": "Managed through {surface}.",
      "detail.status.detailAvailable": "Already available through {surface} in this checkout.",
      "detail.status.detailReady": "Installed successfully and ready for downstream handoff.",
      "detail.status.detailWarning": "{warning}",
      "detail.status.detailBlocked": "This item cannot be installed in the current checkout.",
      "detail.downstream.plugin": "Open plugin console",
      "detail.downstream.role": "Open roles workspace",
      "detail.downstream.workflow": "Open workflow editor",
      "detail.downstream.default": "Open role authoring",
      "detail.tab.overview": "Overview",
      "detail.tab.versions": "Versions",
      "detail.tab.reviews": "Reviews",
      "detail.license": "License: {license}",
      "detail.sourceBuiltin": "Repo-owned built-in skill",
      "detail.latest": "Latest: {version}",
      "detail.noVersion": "No published version yet.",
      "detail.installedVersion": "Installed version: {version}",
      "detail.localPath": "Local path: {path}",
      "detail.docsRef": "Docs reference: {ref}",
      "detail.skillPreviewUnavailable": "Skill preview unavailable: {error}",
      "detail.sideload.title": "Local side-load",
      "detail.sideload.pluginDesc": "Reuse the existing local plugin install seam from within the marketplace workspace.",
      "detail.sideload.pluginButton": "Side-load local plugin",
      "detail.sideload.uploadDesc": "Upload a zip package containing a valid {file} at its root.",
      "detail.sideload.uploadButton": "Side-load {type}",
      "detail.sideload.installing": "Installing...",
      "detail.moderation.title": "Moderation",
      "detail.moderation.verify": "Verify",
      "detail.moderation.feature": "Feature",
      "detail.moderation.hint": "Admin-only actions fail with an explicit permission error when the current operator is not allowed to moderate this item.",
      "detail.versionUpload.title": "Upload a new version",
      "detail.versionUpload.versionLabel": "Version",
      "detail.versionUpload.versionPlaceholder": "1.2.0",
      "detail.versionUpload.changelogLabel": "Changelog",
      "detail.versionUpload.changelogPlaceholder": "What changed in this release?",
      "detail.versionUpload.artifactLabel": "Artifact",
      "detail.versionUpload.artifactHint": "Upload a zip package whose root matches the current marketplace artifact contract for this item type.",
      "detail.versionUpload.uploading": "Uploading...",
      "detail.versionUpload.uploadButton": "Upload version",
      "detail.noReviews": "No reviews yet.",
      "detail.deleteItem": "Delete item",
      "preview.skillPackage": "Skill package",
      "preview.frontmatter": "Frontmatter YAML",
      "preview.agentYaml": "Agent YAML",
    };
    let result = map[key] ?? key;
    if (values) {
      Object.entries(values).forEach(([k, v]) => {
        result = result.replace(new RegExp(`\\{${k}\\}`, "g"), String(v));
      });
    }
    return result;
  },
}));

jest.mock("@/lib/stores/marketplace-store", () => {
  const labels: Record<string, string> = {
    plugin: "Plugin",
    skill: "Skill",
    role: "Role",
    workflow_template: "Workflow",
  };
  return {
    useMarketplaceStore: (
      selector: (state: Record<string, unknown>) => unknown,
    ) =>
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
    typeDisplayLabel: (type: string) => labels[type] ?? type,
    resolveMarketplaceConsumptionRecord: () => null,
  };
});

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
    expect(screen.getByText("Docs reference: docs/role-yaml.md")).toBeInTheDocument();
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
