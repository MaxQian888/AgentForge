jest.mock("./auth-store", () => ({
  useAuthStore: {
    getState: jest.fn(() => ({ accessToken: "test-token" })),
  },
}));

jest.mock("sonner", () => ({
  toast: { success: jest.fn(), error: jest.fn() },
}));

import { forwardRunEventToEmployee } from "./ws-store";
import { useEmployeeRunsStore } from "./employee-runs-store";

describe("forwardRunEventToEmployee", () => {
  const empID = "11111111-2222-3333-4444-555555555555";

  beforeEach(() => {
    useEmployeeRunsStore.setState({
      runsByEmployee: {},
      loadingByEmployee: {},
      pageByEmployee: {},
      hasMoreByEmployee: {},
      kindByEmployee: {},
    });
  });

  it("ingests a workflow.execution.completed payload tagged with actingEmployeeId", () => {
    forwardRunEventToEmployee("workflow.execution.completed", {
      executionId: "exec-123",
      workflowName: "echo-flow",
      actingEmployeeId: empID,
      status: "completed",
      startedAt: "2026-04-20T10:00:00Z",
      completedAt: "2026-04-20T10:00:30Z",
    });
    const rows = useEmployeeRunsStore.getState().runsByEmployee[empID];
    expect(rows).toHaveLength(1);
    expect(rows[0].kind).toBe("workflow");
    expect(rows[0].status).toBe("completed");
    expect(rows[0].refUrl).toBe("/workflow/runs/exec-123");
    expect(rows[0].name).toBe("echo-flow");
    expect(rows[0].durationMs).toBe(30000);
  });

  it("ignores workflow payloads without actingEmployeeId", () => {
    forwardRunEventToEmployee("workflow.execution.started", {
      executionId: "exec-no-emp",
      workflowName: "x",
      status: "running",
    });
    expect(useEmployeeRunsStore.getState().runsByEmployee).toEqual({});
  });

  it("ignores workflow payloads without an id", () => {
    forwardRunEventToEmployee("workflow.execution.started", {
      actingEmployeeId: empID,
      workflowName: "x",
      status: "running",
    });
    expect(useEmployeeRunsStore.getState().runsByEmployee[empID]).toBeUndefined();
  });

  it("ingests an agent.completed payload tagged with employeeId", () => {
    forwardRunEventToEmployee("agent.completed", {
      agentRunId: "run-abc",
      roleId: "code-reviewer",
      employeeId: empID,
      status: "completed",
      startedAt: "2026-04-20T10:00:00Z",
      completedAt: "2026-04-20T10:01:00Z",
    });
    const rows = useEmployeeRunsStore.getState().runsByEmployee[empID];
    expect(rows).toHaveLength(1);
    expect(rows[0].kind).toBe("agent");
    expect(rows[0].name).toBe("code-reviewer");
    expect(rows[0].refUrl).toBe("/agents?run=run-abc");
    expect(rows[0].durationMs).toBe(60000);
  });

  it("ignores agent payloads without employeeId", () => {
    forwardRunEventToEmployee("agent.failed", {
      agentRunId: "run-xyz",
      roleId: "code-reviewer",
      status: "failed",
    });
    expect(useEmployeeRunsStore.getState().runsByEmployee).toEqual({});
  });

  it("ignores non-workflow non-agent event types", () => {
    forwardRunEventToEmployee("task.updated", {
      actingEmployeeId: empID,
      id: "task-1",
    });
    expect(useEmployeeRunsStore.getState().runsByEmployee).toEqual({});
  });
});
