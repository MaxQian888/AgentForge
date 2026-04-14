import { editorReducer } from "./context";
import type { EditorState } from "./types";
import type { Node, Edge } from "@xyflow/react";

const emptyState: EditorState = {
  name: "",
  description: "",
  nodes: [],
  edges: [],
  selectedNodeId: null,
  selectedEdgeId: null,
  undoStack: [],
  redoStack: [],
  dirty: false,
  clipboard: [],
};

function makeNode(id: string, type = "trigger"): Node {
  return { id, type, position: { x: 0, y: 0 }, data: { label: id } } as Node;
}

function makeEdge(id: string, source: string, target: string): Edge {
  return { id, source, target } as Edge;
}

describe("editorReducer", () => {
  it("LOAD initializes state from definition", () => {
    const nodes = [makeNode("n1")];
    const edges = [makeEdge("e1", "n1", "n2")];
    const next = editorReducer(emptyState, {
      type: "LOAD", name: "Test", description: "Desc", nodes, edges,
    });
    expect(next.name).toBe("Test");
    expect(next.description).toBe("Desc");
    expect(next.nodes).toEqual(nodes);
    expect(next.undoStack).toEqual([]);
    expect(next.redoStack).toEqual([]);
    expect(next.dirty).toBe(false);
  });

  it("ADD_NODE pushes undo snapshot and adds node", () => {
    const state = { ...emptyState, nodes: [makeNode("n1")] };
    const newNode = makeNode("n2", "llm_agent");
    const next = editorReducer(state, { type: "ADD_NODE", node: newNode });
    expect(next.nodes).toHaveLength(2);
    expect(next.undoStack).toHaveLength(1);
    expect(next.dirty).toBe(true);
  });

  it("DELETE_NODES removes nodes and connected edges", () => {
    const state = {
      ...emptyState,
      nodes: [makeNode("n1"), makeNode("n2"), makeNode("n3")],
      edges: [makeEdge("e1", "n1", "n2"), makeEdge("e2", "n2", "n3")],
    };
    const next = editorReducer(state, { type: "DELETE_NODES", nodeIds: ["n2"] });
    expect(next.nodes).toHaveLength(2);
    expect(next.edges).toHaveLength(0);
  });

  it("UPDATE_NODE_CONFIG merges config", () => {
    const node = { ...makeNode("n1"), data: { label: "n1", config: { a: 1 } } };
    const state = { ...emptyState, nodes: [node] };
    const next = editorReducer(state, {
      type: "UPDATE_NODE_CONFIG", nodeId: "n1", config: { b: 2 },
    });
    expect(next.nodes[0].data.config).toEqual({ a: 1, b: 2 });
    expect(next.dirty).toBe(true);
  });

  it("UNDO restores previous state and pushes to redo", () => {
    let state = emptyState;
    state = editorReducer(state, { type: "ADD_NODE", node: makeNode("n1") });
    state = editorReducer(state, { type: "ADD_NODE", node: makeNode("n2") });
    expect(state.nodes).toHaveLength(2);
    expect(state.undoStack).toHaveLength(2);

    state = editorReducer(state, { type: "UNDO" });
    expect(state.nodes).toHaveLength(1);
    expect(state.redoStack).toHaveLength(1);
  });

  it("REDO restores and pushes back to undo", () => {
    let state = emptyState;
    state = editorReducer(state, { type: "ADD_NODE", node: makeNode("n1") });
    state = editorReducer(state, { type: "UNDO" });
    expect(state.nodes).toHaveLength(0);

    state = editorReducer(state, { type: "REDO" });
    expect(state.nodes).toHaveLength(1);
  });

  it("new mutation clears redo stack", () => {
    let state = emptyState;
    state = editorReducer(state, { type: "ADD_NODE", node: makeNode("n1") });
    state = editorReducer(state, { type: "UNDO" });
    expect(state.redoStack).toHaveLength(1);

    state = editorReducer(state, { type: "ADD_NODE", node: makeNode("n2") });
    expect(state.redoStack).toHaveLength(0);
  });

  it("undo stack is capped at 50", () => {
    let state = emptyState;
    for (let i = 0; i < 55; i++) {
      state = editorReducer(state, { type: "ADD_NODE", node: makeNode(`n${i}`) });
    }
    expect(state.undoStack.length).toBeLessThanOrEqual(50);
  });

  it("COPY and PASTE duplicate nodes with new IDs", () => {
    const n1 = makeNode("n1");
    let state = { ...emptyState, nodes: [n1] };
    state = editorReducer(state, { type: "COPY", nodes: [n1] });
    expect(state.clipboard).toHaveLength(1);

    state = editorReducer(state, { type: "PASTE" });
    expect(state.nodes).toHaveLength(2);
    expect(state.nodes[1].id).not.toBe("n1");
    expect(state.nodes[1].position.x).toBe(n1.position.x + 50);
  });

  it("SELECT_NODE clears selectedEdgeId", () => {
    const state = { ...emptyState, selectedEdgeId: "e1" };
    const next = editorReducer(state, { type: "SELECT_NODE", nodeId: "n1" });
    expect(next.selectedNodeId).toBe("n1");
    expect(next.selectedEdgeId).toBeNull();
  });

  it("MARK_CLEAN sets dirty to false", () => {
    const state = { ...emptyState, dirty: true };
    const next = editorReducer(state, { type: "MARK_CLEAN" });
    expect(next.dirty).toBe(false);
  });

  it("SYNC_REACTFLOW updates nodes/edges without pushing undo", () => {
    const state = { ...emptyState, nodes: [makeNode("n1")] };
    const newNodes = [makeNode("n1"), makeNode("n2")];
    const next = editorReducer(state, { type: "SYNC_REACTFLOW", nodes: newNodes, edges: [] });
    expect(next.nodes).toHaveLength(2);
    expect(next.undoStack).toHaveLength(0);
    expect(next.dirty).toBe(false);
  });

  it("UPDATE_EDGE_CONDITION updates edge condition and label", () => {
    const edge = makeEdge("e1", "n1", "n2");
    const state = { ...emptyState, edges: [edge] };
    const next = editorReducer(state, {
      type: "UPDATE_EDGE_CONDITION", edgeId: "e1",
      condition: "{{n1.output.ok}} == true", label: "if ok",
    });
    expect((next.edges[0] as { data?: { condition?: string } }).data?.condition).toBe("{{n1.output.ok}} == true");
    expect(next.edges[0].label).toBe("if ok");
  });
});
