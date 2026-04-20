import { render, screen } from "@testing-library/react";
import type { WorkflowNodeData, WorkflowNodeExecution } from "@/lib/stores/workflow-store";

// --- Mocks ---

const mockAccessToken = "test-token";
jest.mock("@/lib/stores/auth-store", () => ({
  useAuthStore: Object.assign(
    () => ({ accessToken: mockAccessToken }),
    { getState: () => ({ accessToken: mockAccessToken }) }
  ),
}));

// Mutable ref that tests can reassign before render.
const outboundRef = { set: new Set<string>() };
const vcsRef = { set: new Set<string>() };

jest.mock("@/lib/stores/workflow-store", () => {
  const makeState = () => ({
    outboundDeliveryFailedExecIds: outboundRef.set,
    markOutboundDeliveryFailed: jest.fn(),
    vcsDeliveryFailedReviewIds: vcsRef.set,
    markVCSDeliveryFailed: jest.fn(),
    definitions: [],
    resolveReview: jest.fn().mockResolvedValue(true),
    sendExternalEvent: jest.fn().mockResolvedValue(true),
  });
  const hook = Object.assign(
    (selectorFn?: (s: ReturnType<typeof makeState>) => unknown) => {
      const state = makeState();
      return selectorFn ? selectorFn(state) : state;
    },
    { getState: () => makeState() }
  );
  return { useWorkflowStore: hook };
});

jest.mock("@/lib/api-client", () => ({
  createApiClient: () => ({
    get: jest.fn().mockResolvedValue({
      data: {
        execution: {
          id: "exec-1",
          workflowId: "wf-1",
          projectId: "proj-1",
          status: "completed",
          startedAt: "2026-04-20T10:00:00Z",
          completedAt: "2026-04-20T10:01:00Z",
          currentNodes: [],
        },
        nodeExecutions: [] as WorkflowNodeExecution[],
      },
    }),
    post: jest.fn().mockResolvedValue({ data: {} }),
  }),
}));

jest.mock("sonner", () => ({ toast: { success: jest.fn(), error: jest.fn() } }));

import { WorkflowExecutionView } from "./workflow-execution-view";

const defaultNodes: WorkflowNodeData[] = [
  { id: "n1", type: "start", label: "Start", position: { x: 0, y: 0 }, config: {} },
];

describe("WorkflowExecutionView", () => {
  beforeEach(() => {
    jest.clearAllMocks();
    outboundRef.set = new Set<string>();
  });

  it("renders without outbound delivery badge normally", async () => {
    render(<WorkflowExecutionView executionId="exec-1" nodes={defaultNodes} />);
    expect(await screen.findByText("Execution")).toBeInTheDocument();
    expect(screen.queryByText("回帖失败")).not.toBeInTheDocument();
  });

  it("renders outbound delivery failed badge when execution is in failed set", async () => {
    outboundRef.set = new Set(["exec-1"]);
    render(<WorkflowExecutionView executionId="exec-1" nodes={defaultNodes} />);
    expect(await screen.findByText("回帖失败")).toBeInTheDocument();
  });

  it("does not render badge for a different execution id", async () => {
    outboundRef.set = new Set(["exec-other"]);
    render(<WorkflowExecutionView executionId="exec-1" nodes={defaultNodes} />);
    expect(await screen.findByText("Execution")).toBeInTheDocument();
    expect(screen.queryByText("回帖失败")).not.toBeInTheDocument();
  });
});
