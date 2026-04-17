import { isWorkflowExportPayload } from "./use-editor-actions";

describe("isWorkflowExportPayload", () => {
  it("accepts minimal valid payloads", () => {
    expect(
      isWorkflowExportPayload({
        name: "wf",
        nodes: [],
        edges: [],
      })
    ).toBe(true);
  });

  it("accepts full exported payloads", () => {
    expect(
      isWorkflowExportPayload({
        version: 1,
        name: "My Workflow",
        description: "Desc",
        nodes: [{ id: "n1", type: "trigger", label: "", position: { x: 0, y: 0 } }],
        edges: [{ id: "e1", source: "n1", target: "n2" }],
        exportedAt: "2026-04-16T00:00:00.000Z",
      })
    ).toBe(true);
  });

  it("rejects null / primitives", () => {
    expect(isWorkflowExportPayload(null)).toBe(false);
    expect(isWorkflowExportPayload(undefined)).toBe(false);
    expect(isWorkflowExportPayload("string")).toBe(false);
    expect(isWorkflowExportPayload(42)).toBe(false);
  });

  it("rejects objects missing nodes/edges arrays", () => {
    expect(isWorkflowExportPayload({ name: "x", nodes: [] })).toBe(false);
    expect(isWorkflowExportPayload({ name: "x", edges: [] })).toBe(false);
    expect(isWorkflowExportPayload({ nodes: [], edges: [] })).toBe(false);
  });

  it("rejects objects where nodes/edges are not arrays", () => {
    expect(
      isWorkflowExportPayload({ name: "x", nodes: "nope", edges: [] })
    ).toBe(false);
    expect(
      isWorkflowExportPayload({ name: "x", nodes: [], edges: {} })
    ).toBe(false);
  });
});
