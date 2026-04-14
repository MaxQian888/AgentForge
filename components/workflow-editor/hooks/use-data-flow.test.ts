import { findPredecessors, getUpstreamOutputFields } from "./use-data-flow";

describe("use-data-flow", () => {
  const nodes = [
    { id: "trigger", type: "trigger", position: { x: 0, y: 0 }, data: { label: "Start" } },
    { id: "planner", type: "llm_agent", position: { x: 0, y: 100 }, data: { label: "Planner" } },
    { id: "coder", type: "llm_agent", position: { x: 0, y: 200 }, data: { label: "Coder" } },
  ];
  const edges = [
    { id: "e1", source: "trigger", target: "planner" },
    { id: "e2", source: "planner", target: "coder" },
  ];

  it("finds direct predecessors of a node", () => {
    const preds = findPredecessors("coder", nodes, edges);
    expect(preds.map((p) => p.id)).toEqual(["planner", "trigger"]);
  });

  it("returns empty for trigger (no predecessors)", () => {
    const preds = findPredecessors("trigger", nodes, edges);
    expect(preds).toEqual([]);
  });

  it("returns known output fields for llm_agent", () => {
    const fields = getUpstreamOutputFields("planner", "llm_agent");
    expect(fields.length).toBeGreaterThan(0);
    expect(fields[0].copyTemplate).toContain("{{planner.");
  });

  it("returns output field for agent_dispatch", () => {
    const fields = getUpstreamOutputFields("n1", "agent_dispatch");
    expect(fields[0].path).toBe("output");
    expect(fields[0].copyTemplate).toBe("{{n1.output}}");
  });

  it("returns output field for function", () => {
    const fields = getUpstreamOutputFields("fn1", "function");
    expect(fields[0].copyTemplate).toBe("{{fn1.output}}");
  });

  it("returns output.* for unknown node types", () => {
    const fields = getUpstreamOutputFields("x", "unknown_type");
    expect(fields[0].path).toBe("output.*");
    expect(fields[0].copyTemplate).toBe("{{x.output}}");
  });

  it("handles cycles without infinite loop", () => {
    const cyclicEdges = [
      { id: "c1", source: "a", target: "b" },
      { id: "c2", source: "b", target: "a" },
    ];
    const cyclicNodes = [
      { id: "a", type: "trigger", position: { x: 0, y: 0 }, data: { label: "A" } },
      { id: "b", type: "trigger", position: { x: 0, y: 100 }, data: { label: "B" } },
    ];
    // Should terminate without stack overflow
    const preds = findPredecessors("a", cyclicNodes, cyclicEdges);
    expect(preds.map((p) => p.id)).toEqual(["b"]);
  });
});
