import { NODE_REGISTRY, getNodeMeta, getNodesByCategory } from "./node-registry";

const ALL_NODE_TYPES = [
  "trigger", "condition", "agent_dispatch", "notification", "status_transition",
  "gate", "parallel_split", "parallel_join", "llm_agent", "function",
  "loop", "human_review", "wait_event", "sub_workflow", "http_call", "im_send",
];

describe("node-registry", () => {
  it("registers all node types", () => {
    expect(NODE_REGISTRY).toHaveLength(ALL_NODE_TYPES.length);
    const types = NODE_REGISTRY.map((n) => n.type);
    for (const t of ALL_NODE_TYPES) {
      expect(types).toContain(t);
    }
  });

  it("getNodeMeta returns correct entry for each type", () => {
    for (const t of ALL_NODE_TYPES) {
      const meta = getNodeMeta(t);
      expect(meta).toBeDefined();
      expect(meta!.type).toBe(t);
      expect(meta!.label).toBeTruthy();
      expect(meta!.icon).toBeDefined();
      expect(meta!.category).toBeTruthy();
    }
  });

  it("getNodeMeta returns undefined for unknown type", () => {
    expect(getNodeMeta("nonexistent")).toBeUndefined();
  });

  it("getNodesByCategory returns correct groupings", () => {
    const entry = getNodesByCategory("entry");
    expect(entry.map((n) => n.type)).toEqual(["trigger"]);

    const flow = getNodesByCategory("flow");
    expect(flow.map((n) => n.type)).toEqual(
      expect.arrayContaining(["parallel_split", "parallel_join", "loop", "sub_workflow"])
    );
  });

  it("every node has a valid configSchema with group fields", () => {
    for (const meta of NODE_REGISTRY) {
      for (const field of meta.configSchema) {
        expect(field.key).toBeTruthy();
        expect(field.group).toBeTruthy();
        expect(["text", "textarea", "select", "number", "boolean", "expression", "json"]).toContain(field.type);
      }
    }
  });
});
