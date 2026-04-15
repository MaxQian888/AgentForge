import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

jest.mock("react-markdown", () => ({
  __esModule: true,
  default: ({ children }: { children: unknown }) => children,
}));

jest.mock("remark-gfm", () => ({
  __esModule: true,
  default: () => null,
}));

import SkillsPage from "./page";

const fetchSkills = jest.fn().mockResolvedValue(undefined);
const fetchSkillDetail = jest.fn().mockResolvedValue(undefined);
const verifySkills = jest.fn().mockResolvedValue({ ok: false, results: [] });
const syncMirrors = jest.fn().mockResolvedValue({ updatedTargets: [], results: [] });
const selectSkill = jest.fn();
const setFilters = jest.fn();

const baseState = {
  items: [],
  selectedSkill: null,
  loading: false,
  detailLoading: false,
  actionLoading: false,
  error: null,
  filters: {
    family: "all",
    status: "all",
    query: "",
  },
  fetchSkills,
  fetchSkillDetail,
  verifySkills,
  syncMirrors,
  selectSkill,
  setFilters,
};

const useSkillsStore = jest.fn((selector?: (state: typeof baseState) => unknown) =>
  typeof selector === "function" ? selector(baseState) : baseState,
);

jest.mock("@/lib/stores/skills-store", () => ({
  useSkillsStore: (...args: unknown[]) => useSkillsStore(...(args as [])),
}));

jest.mock("@/hooks/use-breadcrumbs", () => ({
  useBreadcrumbs: jest.fn(),
}));

describe("SkillsPage", () => {
  beforeEach(() => {
    jest.clearAllMocks();
    Object.assign(baseState, {
      items: [],
      selectedSkill: null,
      loading: false,
      detailLoading: false,
      actionLoading: false,
      error: null,
      filters: {
        family: "all",
        status: "all",
        query: "",
      },
    });
  });

  it("loads the governed skills inventory on mount", async () => {
    render(<SkillsPage />);

    await waitFor(() => {
      expect(fetchSkills).toHaveBeenCalled();
    });
  });

  it("renders selected skill preview, diagnostics, and handoff actions", () => {
    Object.assign(baseState, {
      items: [
        {
          id: "react",
          family: "built-in-runtime",
          sourceType: "repo-authored",
          canonicalRoot: "skills/react",
          previewAvailable: true,
          bundle: { member: true, category: "frontend", tags: ["react"], docsRef: "docs/role-yaml.md", featured: true },
          health: {
            status: "healthy",
            issues: [],
          },
          consumerSurfaces: [
            { id: "role-skill-catalog", status: "available", label: "Role Skill Catalog", href: "/roles" },
            { id: "marketplace-built-ins", status: "available", label: "Marketplace Built-ins", href: "/marketplace" },
          ],
        },
      ],
      selectedSkill: {
        id: "react",
        family: "built-in-runtime",
        sourceType: "repo-authored",
        canonicalRoot: "skills/react",
        docsRef: "docs/role-yaml.md",
        previewAvailable: true,
        bundle: { member: true, category: "frontend", tags: ["react"], docsRef: "docs/role-yaml.md", featured: true },
        health: {
          status: "healthy",
          issues: [
            {
              code: "preview_unavailable",
              message: "preview unavailable for demo",
              targetPath: "skills/react",
            },
          ],
        },
        consumerSurfaces: [
          { id: "role-skill-catalog", status: "available", label: "Role Skill Catalog", href: "/roles" },
          { id: "marketplace-built-ins", status: "available", label: "Marketplace Built-ins", href: "/marketplace" },
        ],
        supportedActions: ["verify-internal", "verify-builtins", "open-roles", "open-marketplace"],
        blockedActions: [{ id: "sync-mirrors", reason: "only workflow-mirror skills can sync mirrors" }],
        preview: {
          canonicalPath: "skills/react",
          label: "React",
          displayName: "AgentForge React",
          description: "Build React surfaces.",
          defaultPrompt: "Use React safely",
          markdownBody: "# React\n\nBuild React surfaces.",
          frontmatterYaml: "name: React",
          requires: ["skills/typescript"],
          tools: ["browser_preview"],
          availableParts: ["agents"],
          referenceCount: 0,
          scriptCount: 0,
          assetCount: 0,
          agentConfigs: [
            {
              path: "agents/openai.yaml",
              yaml: "interface:\n  display_name: AgentForge React",
              displayName: "AgentForge React",
            },
          ],
        },
      },
    });

    render(<SkillsPage />);

    expect(screen.getByText("Skills")).toBeInTheDocument();
    expect(screen.getByText("React")).toBeInTheDocument();
    expect(screen.getByText("preview unavailable for demo")).toBeInTheDocument();
    expect(screen.getByText("Role Skill Catalog")).toBeInTheDocument();
    expect(screen.getByText("Marketplace Built-ins")).toBeInTheDocument();
    expect(screen.getByText("only workflow-mirror skills can sync mirrors")).toBeInTheDocument();
    expect(screen.getByText("Skill package")).toBeInTheDocument();
  });

  it("triggers verify and sync actions from the workspace", async () => {
    const user = userEvent.setup();
    Object.assign(baseState, {
      items: [
        {
          id: "openspec-propose",
          family: "workflow-mirror",
          sourceType: "repo-authored",
          canonicalRoot: ".codex/skills/openspec-propose",
          previewAvailable: true,
          bundle: { member: false },
          health: { status: "drifted", issues: [] },
          consumerSurfaces: [],
        },
      ],
      selectedSkill: {
        id: "openspec-propose",
        family: "workflow-mirror",
        sourceType: "repo-authored",
        canonicalRoot: ".codex/skills/openspec-propose",
        previewAvailable: true,
        bundle: { member: false },
        health: { status: "drifted", issues: [] },
        consumerSurfaces: [],
        supportedActions: ["verify-internal", "sync-mirrors"],
        blockedActions: [],
        preview: {
          canonicalPath: ".codex/skills/openspec-propose",
          label: "openspec-propose",
          markdownBody: "# openspec-propose",
          frontmatterYaml: "name: openspec-propose",
          requires: [],
          tools: [],
          availableParts: [],
          referenceCount: 0,
          scriptCount: 0,
          assetCount: 0,
          agentConfigs: [],
        },
      },
    });

    render(<SkillsPage />);

    await user.click(screen.getByRole("button", { name: "Verify Internal Skills" }));
    await user.click(screen.getByRole("button", { name: "Sync Mirrors" }));

    expect(verifySkills).toHaveBeenCalled();
    expect(syncMirrors).toHaveBeenCalled();
  });
});
