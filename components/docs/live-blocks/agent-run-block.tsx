"use client";

import { useRouter } from "next/navigation";
import { LiveArtifactChrome } from "./live-artifact-chrome";
import { SharedBody } from "./shared-body";
import { useLiveArtifactActions } from "./use-live-artifact-actions";
import type {
  AgentRunTargetRef,
  AgentRunViewOpts,
  BlockNoteBlock,
  ProjectionResult,
} from "./types";

export interface AgentRunBlockProps {
  blockId: string;
  targetRef: AgentRunTargetRef | null;
  viewOpts: AgentRunViewOpts;
  projection: ProjectionResult | undefined;
  cachedOk?: BlockNoteBlock[];
}

export function AgentRunBlock({
  blockId,
  targetRef,
  viewOpts,
  projection,
  cachedOk,
}: AgentRunBlockProps) {
  const router = useRouter();
  const actions = useLiveArtifactActions();

  const actionBlock = {
    id: blockId,
    live_kind: "agent_run",
    target_ref: targetRef,
    view_opts: viewOpts,
  };

  const handleOpenSource = () => {
    if (targetRef?.id) {
      router.push(`/agents?id=${encodeURIComponent(targetRef.id)}`);
    }
    actions.openSource(actionBlock);
  };

  const handleFreeze = () => {
    void actions.freeze(actionBlock);
  };

  const handleRemove = () => actions.remove(actionBlock);

  return (
    <LiveArtifactChrome
      kind="agent_run"
      title="Agent run"
      status={projection?.status}
      diagnostics={projection?.diagnostics}
      onOpenSource={handleOpenSource}
      onFreeze={handleFreeze}
      onRemove={handleRemove}
    >
      <SharedBody
        projection={projection}
        cachedOk={cachedOk}
        onRemove={handleRemove}
        notFoundMessage="This live artifact is no longer available. The referenced agent run may have been deleted."
        forbiddenMessage="The details of this agent run are hidden from your current role."
      />
    </LiveArtifactChrome>
  );
}

export default AgentRunBlock;
