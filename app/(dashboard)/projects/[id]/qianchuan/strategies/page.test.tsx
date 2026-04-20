import { render, screen, fireEvent } from "@testing-library/react";

jest.mock("next/navigation", () => ({
  useParams: () => ({ id: "proj-1" }),
  useRouter: () => ({ push: jest.fn() }),
}));

jest.mock("@/hooks/use-breadcrumbs", () => ({ useBreadcrumbs: jest.fn() }));

jest.mock("sonner", () => ({
  toast: { success: jest.fn(), error: jest.fn() },
}));

jest.mock("@/lib/stores/qianchuan-strategies-store", () => {
  const actual = jest.requireActual("@/lib/stores/qianchuan-strategies-store");
  return {
    ...actual,
    useQianchuanStrategiesStore: Object.assign(jest.fn(), { setState: jest.fn() }),
  };
});

import { useQianchuanStrategiesStore } from "@/lib/stores/qianchuan-strategies-store";
import QianchuanStrategiesListPage from "./page";

const baseRow = {
  id: "row-1",
  projectId: "proj-1",
  name: "my-strategy",
  description: "",
  yamlSource: "name: my-strategy",
  parsedSpec: "{}",
  version: 1,
  status: "draft" as const,
  createdBy: "user-1",
  createdAt: "2026-04-20T00:00:00Z",
  updatedAt: "2026-04-20T00:00:00Z",
  isSystem: false,
};

const systemRow = {
  ...baseRow,
  id: "row-2",
  name: "system:monitor-only",
  status: "published" as const,
  isSystem: true,
  projectId: null,
};

describe("QianchuanStrategiesListPage", () => {
  beforeEach(() => {
    jest.clearAllMocks();
  });

  it("renders empty state when no strategies", () => {
    (useQianchuanStrategiesStore as unknown as jest.Mock).mockReturnValue({
      strategies: [],
      loading: false,
      fetchList: jest.fn(),
    });
    render(<QianchuanStrategiesListPage />);
    expect(screen.getByText(/尚未创建任何策略|no strategies/i)).toBeInTheDocument();
  });

  it("renders rows with name, version, status badge", () => {
    (useQianchuanStrategiesStore as unknown as jest.Mock).mockReturnValue({
      strategies: [baseRow],
      loading: false,
      fetchList: jest.fn(),
    });
    render(<QianchuanStrategiesListPage />);
    expect(screen.getByText("my-strategy")).toBeInTheDocument();
    expect(screen.getByText("v1")).toBeInTheDocument();
    expect(screen.getAllByText(/草稿|draft/i).length).toBeGreaterThan(0);
  });

  it("shows system badge on system rows", () => {
    (useQianchuanStrategiesStore as unknown as jest.Mock).mockReturnValue({
      strategies: [systemRow],
      loading: false,
      fetchList: jest.fn(),
    });
    render(<QianchuanStrategiesListPage />);
    // Badge with literal text "system" (Source column).
    const badges = screen.getAllByText("system");
    expect(badges.length).toBeGreaterThan(0);
  });

  it("status filter narrows list", () => {
    (useQianchuanStrategiesStore as unknown as jest.Mock).mockReturnValue({
      strategies: [baseRow, { ...baseRow, id: "row-x", name: "other", status: "published" as const }],
      loading: false,
      fetchList: jest.fn(),
    });
    render(<QianchuanStrategiesListPage />);

    // Both rows visible initially.
    expect(screen.getByText("my-strategy")).toBeInTheDocument();
    expect(screen.getByText("other")).toBeInTheDocument();

    // Click the "已发布" filter chip (the first/topmost button by that label).
    const matches = screen.getAllByRole("button", { name: /已发布/ });
    fireEvent.click(matches[0]);

    expect(screen.queryByText("my-strategy")).not.toBeInTheDocument();
    expect(screen.getByText("other")).toBeInTheDocument();
  });
});
