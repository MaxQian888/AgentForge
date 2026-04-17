"use client";

import { lazy, Suspense } from "react";
import { defaultProps } from "@blocknote/core";
import { BlockContentWrapper, createReactBlockSpec } from "@blocknote/react";
import { useLiveArtifactContext } from "./live-artifact-context";
import { Shimmer } from "./shared-body";
import {
  safeParseJson,
  type AgentRunTargetRef,
  type AgentRunViewOpts,
  type CostSummaryTargetRef,
  type CostSummaryViewOpts,
  type LiveArtifactKind,
  type ReviewTargetRef,
  type ReviewViewOpts,
  type TargetRef,
  type TaskGroupTargetRef,
  type TaskGroupViewOpts,
  type ViewOpts,
} from "./types";

/**
 * BlockNote custom-block config for all live artifacts. The kind-specific
 * renderer is chosen at render time by `live_kind`. target_ref / view_opts
 * are persisted as JSON strings since BlockNote block props are
 * primitives.
 */
export const liveArtifactConfig = {
  type: "live_artifact",
  propSchema: {
    textAlignment: defaultProps?.textAlignment ?? { default: "left" },
    live_kind: { default: "" },
    target_ref: { default: "{}" },
    view_opts: { default: "{}" },
    last_rendered_at: { default: "" },
  },
  content: "none" as const,
};

const KNOWN_KINDS: readonly LiveArtifactKind[] = [
  "agent_run",
  "cost_summary",
  "review",
  "task_group",
];

function isKnownKind(kind: string): kind is LiveArtifactKind {
  return (KNOWN_KINDS as readonly string[]).includes(kind);
}

// ---------------------------------------------------------------------------
// Lazy-loaded kind components. BlockNote only resolves these when an
// actual live-artifact block renders on screen.
// ---------------------------------------------------------------------------

const LazyAgentRunBlock = lazy(() =>
  import("./agent-run-block").then((mod) => ({ default: mod.AgentRunBlock })),
);
const LazyCostSummaryBlock = lazy(() =>
  import("./cost-summary-block").then((mod) => ({
    default: mod.CostSummaryBlock,
  })),
);
const LazyReviewBlock = lazy(() =>
  import("./review-block").then((mod) => ({ default: mod.ReviewBlock })),
);
const LazyTaskGroupBlock = lazy(() =>
  import("./task-group-block").then((mod) => ({
    default: mod.TaskGroupBlock,
  })),
);

// ---------------------------------------------------------------------------
// Renderer chosen by discriminator
// ---------------------------------------------------------------------------

function KindRenderer({
  blockId,
  kind,
  targetRef,
  viewOpts,
}: {
  blockId: string;
  kind: string;
  targetRef: TargetRef | null;
  viewOpts: ViewOpts;
}) {
  const context = useLiveArtifactContext();
  const projection = context.projections[blockId];

  if (!isKnownKind(kind)) {
    return (
      <div
        className="rounded-md border border-amber-400/40 bg-amber-400/10 p-3 text-xs text-amber-800 dark:text-amber-200"
        role="status"
      >
        Unknown live artifact kind: {kind || "(empty)"}
      </div>
    );
  }

  switch (kind) {
    case "agent_run":
      return (
        <LazyAgentRunBlock
          blockId={blockId}
          targetRef={(targetRef as AgentRunTargetRef | null) ?? null}
          viewOpts={viewOpts as AgentRunViewOpts}
          projection={projection}
        />
      );
    case "cost_summary":
      return (
        <LazyCostSummaryBlock
          blockId={blockId}
          targetRef={(targetRef as CostSummaryTargetRef | null) ?? null}
          viewOpts={viewOpts as CostSummaryViewOpts}
          projection={projection}
        />
      );
    case "review":
      return (
        <LazyReviewBlock
          blockId={blockId}
          targetRef={(targetRef as ReviewTargetRef | null) ?? null}
          viewOpts={viewOpts as ReviewViewOpts}
          projection={projection}
        />
      );
    case "task_group":
      return (
        <LazyTaskGroupBlock
          blockId={blockId}
          targetRef={(targetRef as TaskGroupTargetRef | null) ?? null}
          viewOpts={viewOpts as TaskGroupViewOpts}
          projection={projection}
        />
      );
    default:
      return null;
  }
}

type LiveArtifactBlockLike = {
  id?: unknown;
  props?: {
    live_kind?: unknown;
    target_ref?: unknown;
    view_opts?: unknown;
    last_rendered_at?: unknown;
  };
};

export const liveArtifactRender = ({
  block,
}: {
  block: unknown;
}) => {
  const typed = block as LiveArtifactBlockLike;
  const blockId = String(typed.id ?? "");
  const props = typed.props ?? {};
  const kind = String(props.live_kind ?? "");
  const targetRef = safeParseJson<TargetRef | null>(
    typeof props.target_ref === "string" ? props.target_ref : "null",
    null,
  );
  const viewOpts = safeParseJson<ViewOpts>(
    typeof props.view_opts === "string" ? props.view_opts : "{}",
    {} as ViewOpts,
  );

  return (
    <BlockContentWrapper
      blockType="live_artifact"
      blockProps={
        (typed.props ?? {}) as Parameters<
          typeof BlockContentWrapper
        >[0]["blockProps"]
      }
      propSchema={liveArtifactConfig.propSchema as never}
    >
      <Suspense fallback={<Shimmer />}>
        <KindRenderer
          blockId={blockId}
          kind={kind}
          targetRef={targetRef}
          viewOpts={viewOpts}
        />
      </Suspense>
    </BlockContentWrapper>
  );
};

/**
 * BlockNote block spec factory for `type: "live_artifact"`. Invoked in
 * `block-editor-client.tsx` at schema-construction time — the factory
 * internally calls `createReactBlockSpec` (deferred from module load so
 * test harnesses that mock the BlockNote layer do not explode when they
 * simply import this module).
 */
export function createLiveArtifactBlock() {
  if (typeof createReactBlockSpec !== "function") {
    // Test harness without BlockNote mocks — return a sentinel so the
    // editor-schema builder does not fail to construct.
    return { config: liveArtifactConfig, render: liveArtifactRender } as never;
  }
  const spec = createReactBlockSpec(liveArtifactConfig as never, {
    render: liveArtifactRender,
  });
  return typeof spec === "function" ? spec() : spec;
}
