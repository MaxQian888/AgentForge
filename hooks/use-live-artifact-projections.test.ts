import { act, renderHook, waitFor } from "@testing-library/react";
import type { WSClient, WSHandler } from "@/lib/ws-client";
import {
  useLiveArtifactProjections,
  collectLiveBlocks,
} from "./use-live-artifact-projections";
import type { BlockNoteBlock } from "@/components/docs/live-blocks/types";

// ---------------------------------------------------------------------------
// Shared mocks
// ---------------------------------------------------------------------------

const routerPushMock = jest.fn();
jest.mock("next/navigation", () => ({
  useRouter: () => ({
    push: routerPushMock,
    replace: jest.fn(),
    prefetch: jest.fn(),
    back: jest.fn(),
    pathname: "/",
    query: {},
    asPath: "/",
  }),
  usePathname: () => "/",
  useSearchParams: () => new URLSearchParams(),
}));

const toastSuccessMock = jest.fn();
const toastErrorMock = jest.fn();
jest.mock("sonner", () => ({
  toast: {
    success: (...a: unknown[]) => toastSuccessMock(...a),
    error: (...a: unknown[]) => toastErrorMock(...a),
    warning: jest.fn(),
  },
}));

// ---------------------------------------------------------------------------
// Fake WSClient
// ---------------------------------------------------------------------------

class FakeWSClient {
  handlers: Map<string, Set<WSHandler>> = new Map();
  sendControl = jest.fn();

  on(eventType: string, handler: WSHandler): void {
    if (!this.handlers.has(eventType)) this.handlers.set(eventType, new Set());
    this.handlers.get(eventType)!.add(handler);
  }
  off(eventType: string, handler: WSHandler): void {
    this.handlers.get(eventType)?.delete(handler);
  }
  emit(eventType: string, data: unknown): void {
    this.handlers.get(eventType)?.forEach((h) => h(data));
  }
}

function makeWS(): WSClient {
  // The shape of WSClient we use is narrow: on/off/sendControl.
  return new FakeWSClient() as unknown as WSClient;
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function liveBlock(
  id: string,
  opts: { liveKind?: string; target?: unknown; view?: unknown } = {},
): BlockNoteBlock {
  return {
    id,
    type: "live_artifact",
    props: {
      live_kind: opts.liveKind ?? "agent_run",
      target_ref: JSON.stringify(opts.target ?? { kind: "agent_run", id: `run-${id}` }),
      view_opts: JSON.stringify(opts.view ?? {}),
    },
  };
}

type MockFetchCall = { url: string; init: RequestInit };

function installFetchMock(
  impl: (call: MockFetchCall) => Promise<Response> | Response,
): { calls: MockFetchCall[] } {
  const calls: MockFetchCall[] = [];
  const fn: typeof fetch = (input, init) => {
    const url = typeof input === "string" ? input : String(input);
    const initNormalised: RequestInit = init ?? {};
    calls.push({ url, init: initNormalised });
    return Promise.resolve(impl({ url, init: initNormalised }));
  };
  global.fetch = fn as unknown as typeof fetch;
  return { calls };
}

function jsonResponse(body: unknown, status = 200): Response {
  // jsdom does not ship Response, so we fake just the fields our hook reads.
  const serialised = JSON.stringify(body);
  return {
    ok: status >= 200 && status < 300,
    status,
    json: async () => JSON.parse(serialised),
    text: async () => serialised,
  } as unknown as Response;
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe("collectLiveBlocks", () => {
  it("ignores non-live blocks and malformed props", () => {
    const doc: BlockNoteBlock[] = [
      { id: "p1", type: "paragraph" },
      { id: "lb1", type: "live_artifact", props: { live_kind: "agent_run", target_ref: "not json", view_opts: "{}" } },
      liveBlock("lb2"),
    ];
    const out = collectLiveBlocks(doc);
    expect(out.map((b) => b.blockId)).toEqual(["lb2"]);
  });
});

describe("useLiveArtifactProjections", () => {
  const baseOpts = (overrides: Partial<Parameters<typeof useLiveArtifactProjections>[0]> = {}) => ({
    assetId: "asset-1",
    projectId: "project-1",
    assetKind: "wiki_page" as string,
    editorDocument: [] as BlockNoteBlock[],
    apiUrl: "http://localhost:7777",
    token: "tkn",
    wsClient: makeWS(),
    ...overrides,
  });

  beforeEach(() => {
    routerPushMock.mockReset();
    toastSuccessMock.mockReset();
    toastErrorMock.mockReset();
  });

  it("fetches initial projections for all live blocks on mount", async () => {
    const { calls } = installFetchMock(() =>
      jsonResponse({
        results: {
          b1: {
            status: "ok",
            projection: [{ type: "paragraph", content: "b1" }],
            projected_at: new Date().toISOString(),
            ttl_hint_ms: 60_000,
          },
          b2: {
            status: "ok",
            projection: [{ type: "paragraph", content: "b2" }],
            projected_at: new Date().toISOString(),
          },
        },
      }),
    );

    const doc: BlockNoteBlock[] = [liveBlock("b1"), liveBlock("b2")];
    const { result } = renderHook(() =>
      useLiveArtifactProjections(baseOpts({ editorDocument: doc })),
    );

    await waitFor(() => expect(calls).toHaveLength(1));
    await waitFor(() =>
      expect(Object.keys(result.current.projections)).toEqual(
        expect.arrayContaining(["b1", "b2"]),
      ),
    );

    expect(calls[0].url).toContain(
      "/api/v1/projects/project-1/knowledge/assets/asset-1/live-artifacts/project",
    );
    expect(calls[0].init.method).toBe("POST");
    const body = JSON.parse(calls[0].init.body as string) as { blocks: Array<{ block_id: string }> };
    expect(body.blocks.map((b) => b.block_id).sort()).toEqual(["b1", "b2"]);
  });

  it("does not fetch when the doc has no live blocks", async () => {
    const { calls } = installFetchMock(() => jsonResponse({ results: {} }));
    const doc: BlockNoteBlock[] = [{ id: "p1", type: "paragraph" }];
    renderHook(() => useLiveArtifactProjections(baseOpts({ editorDocument: doc })));
    // Let any effects flush.
    await Promise.resolve();
    expect(calls).toHaveLength(0);
  });

  it("re-projects only the affected blocks on a live_artifacts_changed push", async () => {
    let response: unknown = {
      results: {
        b1: { status: "ok", projection: [], projected_at: new Date().toISOString() },
        b2: { status: "ok", projection: [], projected_at: new Date().toISOString() },
      },
    };
    const { calls } = installFetchMock(() => jsonResponse(response));

    const ws = makeWS();
    const doc: BlockNoteBlock[] = [liveBlock("b1"), liveBlock("b2")];
    renderHook(() =>
      useLiveArtifactProjections(baseOpts({ editorDocument: doc, wsClient: ws })),
    );

    await waitFor(() => expect(calls).toHaveLength(1));

    // Swap the response so we can tell the push-triggered call apart.
    response = {
      results: {
        b1: { status: "degraded", projection: [], projected_at: new Date().toISOString(), diagnostics: "updated" },
      },
    };

    await act(async () => {
      (ws as unknown as FakeWSClient).emit("knowledge.asset.live_artifacts_changed", {
        payload: { asset_id: "asset-1", block_ids_affected: ["b1"] },
      });
      // Flush the microtask queued by fetch().then(...)
      await Promise.resolve();
      await Promise.resolve();
    });

    await waitFor(() => expect(calls).toHaveLength(2));
    const body2 = JSON.parse(calls[1].init.body as string) as { blocks: Array<{ block_id: string }> };
    expect(body2.blocks.map((b) => b.block_id)).toEqual(["b1"]);
  });

  it("re-projects all blocks and re-sends asset_open on reconnect", async () => {
    const { calls } = installFetchMock(() =>
      jsonResponse({
        results: {
          b1: { status: "ok", projection: [], projected_at: new Date().toISOString() },
          b2: { status: "ok", projection: [], projected_at: new Date().toISOString() },
        },
      }),
    );

    const ws = makeWS();
    const doc: BlockNoteBlock[] = [liveBlock("b1"), liveBlock("b2")];
    renderHook(() =>
      useLiveArtifactProjections(baseOpts({ editorDocument: doc, wsClient: ws })),
    );

    await waitFor(() => expect(calls).toHaveLength(1));
    const fake = ws as unknown as FakeWSClient;
    // Initial asset_open frame sent on mount.
    const openFrames = fake.sendControl.mock.calls.filter(
      (c) => (c[0] as { type?: string }).type === "asset_open",
    );
    expect(openFrames.length).toBeGreaterThanOrEqual(1);

    fake.sendControl.mockClear();

    await act(async () => {
      fake.emit("connected", null);
      await Promise.resolve();
      await Promise.resolve();
    });

    await waitFor(() => expect(calls).toHaveLength(2));
    const reBody = JSON.parse(calls[1].init.body as string) as { blocks: Array<{ block_id: string }> };
    expect(reBody.blocks.map((b) => b.block_id).sort()).toEqual(["b1", "b2"]);

    // Re-sent asset_open after reconnect.
    const resentOpen = fake.sendControl.mock.calls.find(
      (c) => (c[0] as { type?: string }).type === "asset_open",
    );
    expect(resentOpen).toBeDefined();
  });

  it("short-circuits when assetKind is not wiki_page", async () => {
    const { calls } = installFetchMock(() => jsonResponse({ results: {} }));
    const ws = makeWS();
    const fake = ws as unknown as FakeWSClient;
    const doc: BlockNoteBlock[] = [liveBlock("b1")];
    renderHook(() =>
      useLiveArtifactProjections(
        baseOpts({ editorDocument: doc, assetKind: "template", wsClient: ws }),
      ),
    );
    await Promise.resolve();
    await Promise.resolve();
    expect(calls).toHaveLength(0);
    expect(fake.sendControl).not.toHaveBeenCalled();
  });

  it("re-projects an expired block after its TTL elapses", async () => {
    jest.useFakeTimers({ doNotFake: ["queueMicrotask"] });
    // First response: short TTL so it expires quickly.
    let callIndex = 0;
    installFetchMock(() => {
      callIndex += 1;
      return jsonResponse({
        results: {
          b1: {
            status: "ok",
            projection: [],
            projected_at: new Date(Date.now() - 10).toISOString(),
            ttl_hint_ms: 1, // expire effectively immediately
          },
        },
      });
    });

    const doc: BlockNoteBlock[] = [liveBlock("b1")];
    renderHook(() =>
      useLiveArtifactProjections(baseOpts({ editorDocument: doc, wsClient: makeWS() })),
    );

    // Flush initial fetch → json → setState chain under fake timers.
    await act(async () => {
      for (let i = 0; i < 10; i++) await Promise.resolve();
    });
    expect(callIndex).toBeGreaterThanOrEqual(1);
    const firstCount = callIndex;

    // Advance past TTL scan interval to trigger TTL refresh.
    await act(async () => {
      jest.advanceTimersByTime(6_000);
      for (let i = 0; i < 10; i++) await Promise.resolve();
    });

    expect(callIndex).toBeGreaterThan(firstCount);
    jest.useRealTimers();
  });

  it("sends asset_close and stops responding to events on unmount", async () => {
    installFetchMock(() =>
      jsonResponse({
        results: {
          b1: { status: "ok", projection: [], projected_at: new Date().toISOString() },
        },
      }),
    );
    const ws = makeWS();
    const fake = ws as unknown as FakeWSClient;
    const doc: BlockNoteBlock[] = [liveBlock("b1")];
    const { unmount } = renderHook(() =>
      useLiveArtifactProjections(baseOpts({ editorDocument: doc, wsClient: ws })),
    );

    await act(async () => {
      await Promise.resolve();
      await Promise.resolve();
    });

    fake.sendControl.mockClear();
    unmount();

    const closeFrame = fake.sendControl.mock.calls.find(
      (c) => (c[0] as { type?: string }).type === "asset_close",
    );
    expect(closeFrame).toBeDefined();
    expect((closeFrame![0] as { payload?: { assetId?: string } }).payload?.assetId).toBe(
      "asset-1",
    );

    // Handlers should be detached — emitting after unmount must not throw.
    expect(() =>
      fake.emit("knowledge.asset.live_artifacts_changed", {
        payload: { asset_id: "asset-1", block_ids_affected: ["b1"] },
      }),
    ).not.toThrow();
  });
});
