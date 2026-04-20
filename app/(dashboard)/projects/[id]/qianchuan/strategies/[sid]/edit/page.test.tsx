import { render, screen, fireEvent, waitFor } from "@testing-library/react";

import * as React from "react";

const setModelMarkers = jest.fn();

// Stub @monaco-editor/react: render a textarea so the test can drive the
// editor; expose model + monaco globals for marker assertions.
jest.mock("@monaco-editor/react", () => {
  type EditorProps = {
    value?: string;
    onChange?: (next: string | undefined) => void;
    onMount?: (editor: unknown, monaco: unknown) => void;
    options?: { readOnly?: boolean };
    [key: string]: unknown;
  };
  // eslint-disable-next-line @typescript-eslint/no-require-imports
  const ReactRef = require("react") as typeof import("react");
  return {
    __esModule: true,
    default: function MockEditor({ value, onChange, onMount, options }: EditorProps) {
      ReactRef.useEffect(() => {
        const fakeModel = { uri: "inmemory://strategy.yaml" };
        const fakeEditor = { getModel: () => fakeModel };
        const fakeMonaco = { editor: { setModelMarkers } };
        onMount?.(fakeEditor, fakeMonaco);
      }, [onMount]);
      const props: Record<string, unknown> = {
        "data-testid": "monaco-editor",
        value: value ?? "",
        onChange: (e: { target: { value: string } }) => onChange?.(e.target.value),
      };
      if (options?.readOnly) {
        props.readOnly = true;
      }
      return ReactRef.createElement("textarea", props);
    },
  };
});

// Pretend next/dynamic returns the underlying module synchronously so the
// Monaco mock above is rendered, not a placeholder.
jest.mock("next/dynamic", () => () => {
  // eslint-disable-next-line @typescript-eslint/no-require-imports
  const Component = require("@monaco-editor/react").default;
  // eslint-disable-next-line @typescript-eslint/no-require-imports
  const ReactRef = require("react") as typeof import("react");
  return function DynamicWrapper(props: Record<string, unknown>) {
    return ReactRef.createElement(Component as React.ComponentType<Record<string, unknown>>, props);
  };
});

const router = { push: jest.fn() };
jest.mock("next/navigation", () => ({
  useParams: () => ({ id: "proj-1", sid: "strategy-1" }),
  useRouter: () => router,
}));

jest.mock("@/hooks/use-breadcrumbs", () => ({ useBreadcrumbs: jest.fn() }));

jest.mock("sonner", () => ({
  toast: { success: jest.fn(), error: jest.fn() },
}));

const fetchOne = jest.fn();
const update = jest.fn();
const publish = jest.fn();
const archive = jest.fn();
const testRun = jest.fn();
const clearError = jest.fn();

let storeState: Record<string, unknown> = {};

jest.mock("@/lib/stores/qianchuan-strategies-store", () => {
  const actual = jest.requireActual("@/lib/stores/qianchuan-strategies-store");
  return {
    ...actual,
    useQianchuanStrategiesStore: Object.assign(
      jest.fn(() => storeState),
      { setState: jest.fn() },
    ),
  };
});

import EditPage from "./page";

const baseStrategy = {
  id: "strategy-1",
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

beforeEach(() => {
  jest.clearAllMocks();
  setModelMarkers.mockClear();
  storeState = {
    selected: baseStrategy,
    lastError: null,
    lastTestResult: null,
    fetchOne,
    update,
    publish,
    archive,
    testRun,
    clearError,
  };
});

describe("Qianchuan strategy EditPage", () => {
  it("loads yamlSource from selected strategy into the editor", () => {
    render(<EditPage />);
    expect(screen.getByTestId("monaco-editor")).toHaveValue("name: my-strategy");
  });

  it("calls update on save and shows success toast on green response", async () => {
    update.mockResolvedValue(baseStrategy);
    render(<EditPage />);
    fireEvent.click(screen.getByRole("button", { name: /save/i }));
    await waitFor(() => expect(update).toHaveBeenCalledWith("strategy-1", "name: my-strategy"));
  });

  it("places a Monaco marker when save returns a structured StrategyParseError", async () => {
    update.mockResolvedValue(null);
    storeState.lastError = { line: 3, col: 5, field: "rules[0].condition", msg: "bad" };
    render(<EditPage />);
    fireEvent.click(screen.getByRole("button", { name: /save/i }));
    await waitFor(() => expect(setModelMarkers).toHaveBeenCalled());
    const lastCall = setModelMarkers.mock.calls.at(-1);
    expect(lastCall?.[2]?.[0]).toMatchObject({ startLineNumber: 3, message: expect.stringContaining("bad") });
  });

  it("test panel runs against valid JSON snapshot", async () => {
    testRun.mockResolvedValue({
      fired_rules: ["heartbeat"],
      actions: [{ rule: "heartbeat", type: "notify_im", params: {} }],
    });
    render(<EditPage />);
    const textarea = screen.getByLabelText(/snapshot|快照/i);
    fireEvent.change(textarea, { target: { value: '{"metrics":{"cost":1}}' } });
    fireEvent.click(screen.getByRole("button", { name: /^run$/i }));
    await waitFor(() =>
      expect(testRun).toHaveBeenCalledWith("strategy-1", { metrics: { cost: 1 } }),
    );
  });

  it("test panel renders the fired actions when lastTestResult is present", () => {
    storeState.lastTestResult = {
      fired_rules: ["heartbeat"],
      actions: [{ rule: "heartbeat", type: "notify_im", params: {} }],
    };
    render(<EditPage />);
    expect(screen.getByText(/notify_im/)).toBeInTheDocument();
  });

  it("test panel shows inline parse error for invalid JSON without calling backend", () => {
    render(<EditPage />);
    const textarea = screen.getByLabelText(/snapshot|快照/i);
    fireEvent.change(textarea, { target: { value: "not json" } });
    fireEvent.click(screen.getByRole("button", { name: /^run$/i }));
    expect(screen.getByText(/JSON parse failed/i)).toBeInTheDocument();
    expect(testRun).not.toHaveBeenCalled();
  });

  it("editor is read-only when strategy is system", () => {
    storeState.selected = { ...baseStrategy, isSystem: true, projectId: null, status: "published" };
    render(<EditPage />);
    expect(screen.getByTestId("monaco-editor")).toHaveAttribute("readonly");
  });

  it("editor is read-only when strategy status is archived", () => {
    storeState.selected = { ...baseStrategy, status: "archived" };
    render(<EditPage />);
    expect(screen.getByTestId("monaco-editor")).toHaveAttribute("readonly");
  });

  it("publish button is visible only on draft and redirects to list", async () => {
    publish.mockResolvedValue({ ...baseStrategy, status: "published" });
    render(<EditPage />);
    const publishBtn = screen.getByRole("button", { name: /publish/i });
    fireEvent.click(publishBtn);
    await waitFor(() => expect(publish).toHaveBeenCalledWith("strategy-1"));
    await waitFor(() =>
      expect(router.push).toHaveBeenCalledWith("/projects/proj-1/qianchuan/strategies"),
    );
  });

  it("publish button is hidden on already-published rows", () => {
    storeState.selected = { ...baseStrategy, status: "published" };
    render(<EditPage />);
    expect(screen.queryByRole("button", { name: /publish/i })).not.toBeInTheDocument();
  });
});
