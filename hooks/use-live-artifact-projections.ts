"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import { toast } from "sonner";
import type { WSClient, WSHandler } from "@/lib/ws-client";
import {
  safeParseJson,
  type BlockNoteBlock,
  type LiveArtifactBlockRef,
  type LiveArtifactKind,
  type ProjectionResult,
  type TargetRef,
  type ViewOpts,
} from "@/components/docs/live-blocks/types";
import type {
  LiveArtifactActionBlock,
  LiveArtifactActions,
} from "@/components/docs/live-blocks/live-artifact-context";

// ---------------------------------------------------------------------------
// Option + helper types
// ---------------------------------------------------------------------------

export interface UseLiveArtifactProjectionsOptions {
  /** The current asset id. Passing an empty string disables the hook. */
  assetId: string;
  projectId: string;
  /** Asset kind — the hook short-circuits when not "wiki_page". */
  assetKind: string;
  /** The current BlockNote document (editor.document snapshot). */
  editorDocument: readonly BlockNoteBlock[];
  apiUrl: string;
  token: string;
  /**
   * The WebSocket client. Pass null if the socket is not yet connected;
   * the hook will skip subscription wiring until a client is provided.
   */
  wsClient: WSClient | null;
  /**
   * Optional callback invoked after a successful freeze with the updated
   * asset JSON returned from the backend. Useful for parents that want to
   * reload the editor.
   */
  onAssetReload?: (asset: unknown) => void;
}

export interface UseLiveArtifactProjectionsResult {
  /** Map of block id → latest projection result. */
  projections: Record<string, ProjectionResult>;
  /**
   * Re-project blocks. With no argument → all live blocks in the doc.
   * With an array → only those block ids.
   */
  refresh: (blockIds?: string[]) => Promise<void>;
  /** Action wiring to feed into LiveArtifactContext. */
  actions: LiveArtifactActions;
}

/** Live-artifact block pulled from the BlockNote doc, parsed. */
interface LiveBlockInDoc {
  blockId: string;
  liveKind: LiveArtifactKind;
  targetRef: TargetRef;
  viewOpts: ViewOpts;
}

interface ProjectRequestBlock {
  block_id: string;
  live_kind: LiveArtifactKind;
  target_ref: TargetRef;
  view_opts: ViewOpts;
}

interface ProjectResponse {
  results: Record<string, ProjectionResult>;
}

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------

const LIVE_ARTIFACT_TYPE = "live_artifact";
const LIVE_ARTIFACTS_CHANGED_EVENT = "knowledge.asset.live_artifacts_changed";
const CONNECTED_EVENT = "connected";
const ASSET_OPEN_DEBOUNCE_MS = 100;
const TTL_SCAN_INTERVAL_MS = 5_000;
const TTL_REFRESH_DEBOUNCE_MS = 1_000;

// ---------------------------------------------------------------------------
// Pure helpers (exported for test coverage)
// ---------------------------------------------------------------------------

/**
 * Walk a BlockNote doc and collect all `live_artifact` blocks, parsing their
 * JSON-string props into strongly-typed refs. Children are walked depth-first
 * in case live blocks are ever nested.
 */
export function collectLiveBlocks(
  doc: readonly BlockNoteBlock[] | undefined | null,
): LiveBlockInDoc[] {
  if (!doc || !Array.isArray(doc)) {
    return [];
  }

  const out: LiveBlockInDoc[] = [];
  const walk = (blocks: readonly BlockNoteBlock[]) => {
    for (const block of blocks) {
      if (!block || typeof block !== "object") continue;
      if (block.type === LIVE_ARTIFACT_TYPE && block.id && block.props) {
        const props = block.props as Record<string, unknown>;
        const liveKind = String(props.live_kind ?? "") as LiveArtifactKind;
        const rawTarget =
          typeof props.target_ref === "string" ? props.target_ref : "";
        const rawView =
          typeof props.view_opts === "string" ? props.view_opts : "";
        const targetRef = safeParseJson<TargetRef | null>(rawTarget, null);
        const viewOpts = safeParseJson<ViewOpts>(rawView, {} as ViewOpts);
        if (liveKind && targetRef) {
          out.push({
            blockId: block.id,
            liveKind,
            targetRef,
            viewOpts,
          });
        }
      }
      if (Array.isArray(block.children) && block.children.length > 0) {
        walk(block.children);
      }
    }
  };

  walk(doc);
  return out;
}

/** Map a LiveBlockInDoc to the projection-endpoint wire format. */
function toRequestBlock(b: LiveBlockInDoc): ProjectRequestBlock {
  return {
    block_id: b.blockId,
    live_kind: b.liveKind,
    target_ref: b.targetRef,
    view_opts: b.viewOpts,
  };
}

/** Build a stable signature of the live-block set (ids + kinds + targets). */
function signatureFor(blocks: LiveBlockInDoc[]): string {
  return blocks
    .map((b) => `${b.blockId}|${b.liveKind}|${JSON.stringify(b.targetRef)}`)
    .sort()
    .join("\n");
}

// ---------------------------------------------------------------------------
// Navigation helpers for `openSource`
// ---------------------------------------------------------------------------

function routeForTarget(ref: LiveArtifactBlockRef): string | null {
  try {
    const target = ref.target_ref;
    switch (target.kind) {
      case "agent_run":
        return `/agents/${target.id}`;
      case "review":
        return `/reviews/${target.id}`;
      case "cost_summary": {
        const p = new URLSearchParams();
        if (target.filter.range_start) p.set("range_start", target.filter.range_start);
        if (target.filter.range_end) p.set("range_end", target.filter.range_end);
        if (target.filter.runtime) p.set("runtime", target.filter.runtime);
        if (target.filter.provider) p.set("provider", target.filter.provider);
        if (target.filter.member_id) p.set("member_id", target.filter.member_id);
        const qs = p.toString();
        return qs ? `/cost?${qs}` : "/cost";
      }
      case "task_group": {
        const f = target.filter;
        if (f.saved_view_id) {
          return `/tasks?saved_view=${encodeURIComponent(f.saved_view_id)}`;
        }
        const p = new URLSearchParams();
        const inline = f.inline;
        if (inline) {
          if (inline.status?.length) p.set("status", inline.status.join(","));
          if (inline.assignee_id) p.set("assignee", inline.assignee_id);
          if (inline.labels?.length) p.set("labels", inline.labels.join(","));
          if (inline.sprint_id) p.set("sprint", inline.sprint_id);
          if (inline.milestone_id) p.set("milestone", inline.milestone_id);
        }
        const qs = p.toString();
        return qs ? `/tasks?${qs}` : "/tasks";
      }
      default:
        return null;
    }
  } catch {
    return null;
  }
}

/** Coerce the context action-block shape into a typed ref, best-effort. */
function actionBlockToRef(b: LiveArtifactActionBlock): LiveArtifactBlockRef {
  return {
    id: b.id,
    live_kind: b.live_kind as LiveArtifactKind,
    target_ref: b.target_ref as TargetRef,
    view_opts: b.view_opts as ViewOpts,
    last_rendered_at: "",
  };
}

// ---------------------------------------------------------------------------
// Hook
// ---------------------------------------------------------------------------

export function useLiveArtifactProjections(
  options: UseLiveArtifactProjectionsOptions,
): UseLiveArtifactProjectionsResult {
  const {
    assetId,
    projectId,
    assetKind,
    editorDocument,
    apiUrl,
    token,
    wsClient,
    onAssetReload,
  } = options;

  const t = useTranslations("knowledge");
  const router = useRouter();

  const enabled =
    assetKind === "wiki_page" && !!assetId && !!projectId;

  const [projections, setProjections] = useState<Record<string, ProjectionResult>>(
    {},
  );

  // Stable refs for values that the handlers close over but should not
  // trigger re-registering the WS handlers every render.
  const liveBlocks = useMemo(
    () => (enabled ? collectLiveBlocks(editorDocument) : []),
    [enabled, editorDocument],
  );
  const liveBlocksRef = useRef<LiveBlockInDoc[]>(liveBlocks);
  const apiUrlRef = useRef(apiUrl);
  const tokenRef = useRef(token);
  const assetIdRef = useRef(assetId);
  const projectIdRef = useRef(projectId);
  const wsClientRef = useRef<WSClient | null>(wsClient);
  const onAssetReloadRef = useRef(onAssetReload);

  // Sync refs in an effect so render is free of ref writes (React rules).
  useEffect(() => {
    liveBlocksRef.current = liveBlocks;
    apiUrlRef.current = apiUrl;
    tokenRef.current = token;
    assetIdRef.current = assetId;
    projectIdRef.current = projectId;
    wsClientRef.current = wsClient;
    onAssetReloadRef.current = onAssetReload;
  });

  // ------------------------------------------------------------------
  // refresh(blockIds?) — POST to the projection endpoint
  // ------------------------------------------------------------------

  const refresh = useCallback(
    async (blockIds?: string[]): Promise<void> => {
      const current = liveBlocksRef.current;
      if (!assetIdRef.current || !projectIdRef.current) return;
      if (current.length === 0) return;

      const subset =
        blockIds && blockIds.length > 0
          ? current.filter((b) => blockIds.includes(b.blockId))
          : current;
      if (subset.length === 0) return;

      const body = { blocks: subset.map(toRequestBlock) };
      const base = apiUrlRef.current.replace(/\/$/, "");
      const url = `${base}/api/v1/projects/${projectIdRef.current}/knowledge/assets/${assetIdRef.current}/live-artifacts/project`;

      try {
        const res = await fetch(url, {
          method: "POST",
          headers: {
            "Content-Type": "application/json",
            ...(tokenRef.current ? { Authorization: `Bearer ${tokenRef.current}` } : {}),
          },
          body: JSON.stringify(body),
        });
        if (!res.ok) {
          return;
        }
        const data = (await res.json()) as ProjectResponse;
        if (!data || typeof data !== "object" || !data.results) return;

        setProjections((prev) => ({ ...prev, ...data.results }));
      } catch {
        // Swallow — caller can surface network errors via status banners.
      }
    },
    [],
  );

  const refreshRef = useRef(refresh);
  useEffect(() => {
    refreshRef.current = refresh;
  }, [refresh]);

  // ------------------------------------------------------------------
  // §12.1 — Initial projection on mount + whenever the asset changes
  // ------------------------------------------------------------------

  useEffect(() => {
    if (!enabled) return;
    if (liveBlocks.length === 0) return;
    void refreshRef.current();
    // Re-fetch only when the asset identity changes; block set changes are
    // covered by the asset_open debounce path and TTL/subscription pushes.
  }, [enabled, assetId, projectId]); // eslint-disable-line react-hooks/exhaustive-deps

  // ------------------------------------------------------------------
  // §12.2 / §12.3 — WS subscription + reconnect
  // ------------------------------------------------------------------

  useEffect(() => {
    if (!enabled) return;
    const ws = wsClientRef.current;
    if (!ws) return;

    const sendAssetOpen = (blocks: LiveBlockInDoc[]) => {
      ws.sendControl({
        type: "asset_open",
        payload: {
          assetId: assetIdRef.current,
          projectId: projectIdRef.current,
          blocks: blocks.map((b) => ({
            blockId: b.blockId,
            liveKind: b.liveKind,
            targetRef: b.targetRef,
          })),
        },
      });
    };

    const sendAssetClose = (id: string) => {
      ws.sendControl({
        type: "asset_close",
        payload: { assetId: id },
      });
    };

    // Initial asset_open for the blocks we have at mount time. The
    // debounce effect (below) will resend if the set changes.
    sendAssetOpen(liveBlocksRef.current);

    const changedHandler: WSHandler = (raw) => {
      if (!raw || typeof raw !== "object") return;
      const outer = raw as { payload?: unknown; asset_id?: unknown; block_ids_affected?: unknown };
      const payload =
        outer.payload && typeof outer.payload === "object"
          ? (outer.payload as Record<string, unknown>)
          : (outer as Record<string, unknown>);
      const targetAsset =
        typeof payload.asset_id === "string"
          ? payload.asset_id
          : typeof payload.assetId === "string"
            ? payload.assetId
            : "";
      if (targetAsset !== assetIdRef.current) return;
      const affectedRaw =
        (payload.block_ids_affected as unknown) ??
        (payload.blockIdsAffected as unknown);
      const affected = Array.isArray(affectedRaw)
        ? (affectedRaw.filter((id) => typeof id === "string") as string[])
        : [];
      if (affected.length === 0) return;
      void refreshRef.current(affected);
    };

    const connectedHandler: WSHandler = () => {
      // Full re-projection on reconnect, then re-send asset_open so the
      // server re-registers our per-block subscription filters.
      void refreshRef.current();
      sendAssetOpen(liveBlocksRef.current);
    };

    ws.on(LIVE_ARTIFACTS_CHANGED_EVENT, changedHandler);
    ws.on(CONNECTED_EVENT, connectedHandler);

    const capturedAssetId = assetIdRef.current;
    return () => {
      ws.off(LIVE_ARTIFACTS_CHANGED_EVENT, changedHandler);
      ws.off(CONNECTED_EVENT, connectedHandler);
      // §12.2 — release per-(client,asset) subscription filter on close.
      if (capturedAssetId) {
        sendAssetClose(capturedAssetId);
      }
    };
  }, [enabled, assetId, projectId, wsClient]);

  // ------------------------------------------------------------------
  // §12.2 — Debounced asset_open refresh when the live-block set changes
  // ------------------------------------------------------------------

  const blocksSignature = useMemo(() => signatureFor(liveBlocks), [liveBlocks]);
  const initialSignatureRef = useRef<string | null>(null);
  useEffect(() => {
    if (!enabled) return;
    const ws = wsClientRef.current;
    if (!ws) return;
    // Skip the first run — the subscription effect already sent
    // asset_open with the initial set. Only re-send on actual changes.
    if (initialSignatureRef.current === null) {
      initialSignatureRef.current = blocksSignature;
      return;
    }
    if (initialSignatureRef.current === blocksSignature) return;
    initialSignatureRef.current = blocksSignature;

    const handle = setTimeout(() => {
      ws.sendControl({
        type: "asset_open",
        payload: {
          assetId: assetIdRef.current,
          projectId: projectIdRef.current,
          blocks: liveBlocksRef.current.map((b) => ({
            blockId: b.blockId,
            liveKind: b.liveKind,
            targetRef: b.targetRef,
          })),
        },
      });
    }, ASSET_OPEN_DEBOUNCE_MS);

    return () => clearTimeout(handle);
  }, [enabled, blocksSignature]);

  // Reset the signature tracker when the asset changes so the next doc
  // also starts from its "initial set".
  useEffect(() => {
    initialSignatureRef.current = null;
  }, [assetId]);

  // ------------------------------------------------------------------
  // §12.4 — TTL-hint driven lazy re-projection
  // ------------------------------------------------------------------

  const projectionsRef = useRef(projections);
  useEffect(() => {
    projectionsRef.current = projections;
  }, [projections]);
  const lastTtlRefreshRef = useRef<Record<string, number>>({});

  useEffect(() => {
    if (!enabled) return;
    const scan = () => {
      const now = Date.now();
      const expired: string[] = [];
      const knownIds = new Set(liveBlocksRef.current.map((b) => b.blockId));
      for (const [blockId, res] of Object.entries(projectionsRef.current)) {
        if (!knownIds.has(blockId)) continue;
        if (!res || !res.projected_at || !res.ttl_hint_ms) continue;
        const projectedAtMs = Date.parse(res.projected_at);
        if (!Number.isFinite(projectedAtMs)) continue;
        const expiresAt = projectedAtMs + res.ttl_hint_ms;
        if (expiresAt > now) continue;
        // Debounce: don't hammer if we already refreshed this block recently.
        const lastRefresh = lastTtlRefreshRef.current[blockId] ?? 0;
        if (now - lastRefresh < TTL_REFRESH_DEBOUNCE_MS) continue;
        lastTtlRefreshRef.current[blockId] = now;
        expired.push(blockId);
      }
      if (expired.length > 0) {
        void refreshRef.current(expired);
      }
    };

    const interval = setInterval(scan, TTL_SCAN_INTERVAL_MS);
    // Run once shortly after mount so tests with short TTLs + fake timers
    // don't need to wait a full scan interval.
    const kickoff = setTimeout(scan, 0);
    return () => {
      clearInterval(interval);
      clearTimeout(kickoff);
    };
  }, [enabled]);

  // ------------------------------------------------------------------
  // Actions
  // ------------------------------------------------------------------

  const openSource = useCallback(
    (block: LiveArtifactActionBlock) => {
      const ref = actionBlockToRef(block);
      const href = routeForTarget(ref);
      if (!href) return;
      router.push(href);
    },
    [router],
  );

  const freeze = useCallback(
    async (block: LiveArtifactActionBlock): Promise<unknown> => {
      if (!assetIdRef.current || !projectIdRef.current) return null;
      const base = apiUrlRef.current.replace(/\/$/, "");
      const url = `${base}/api/v1/projects/${projectIdRef.current}/knowledge/assets/${assetIdRef.current}/live-artifacts/${block.id}/freeze`;
      try {
        const res = await fetch(url, {
          method: "POST",
          headers: {
            "Content-Type": "application/json",
            ...(tokenRef.current ? { Authorization: `Bearer ${tokenRef.current}` } : {}),
          },
          body: JSON.stringify({}),
        });
        if (!res.ok) {
          const msg = await res.text().catch(() => "");
          toast.error(t("liveArtifact.freezeError"), {
            description: msg || `HTTP ${res.status}`,
          });
          return null;
        }
        const data = await res.json().catch(() => null);
        toast.success(t("liveArtifact.freezeSuccess"));
        onAssetReloadRef.current?.(data);
        return data;
      } catch (err) {
        toast.error(t("liveArtifact.freezeError"), {
          description: err instanceof Error ? err.message : String(err),
        });
        return null;
      }
    },
    [t],
  );

  // BlockNote's `editor.removeBlocks` is invoked from the chrome dropdown
  // via the block's render args; the hook owns no remove affordance.
  const remove = useCallback((_block: LiveArtifactActionBlock) => {
    void _block;
    // no-op — documented behavior
  }, []);

  const actions = useMemo<LiveArtifactActions>(
    () => ({
      openSource,
      // Wrap freeze so the context-compatible signature is `(block) => void`
      // while still returning the updated asset for callers that want it.
      freeze: (block) => {
        void freeze(block);
      },
      remove,
    }),
    [openSource, freeze, remove],
  );

  return { projections, refresh, actions };
}
