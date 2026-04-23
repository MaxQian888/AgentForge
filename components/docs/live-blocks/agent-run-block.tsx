"use client";

import { useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
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
  const t = useTranslations("docs");
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
      title={t("liveArtifact.blocks.agentRun.title")}
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
        notFoundMessage={t("liveArtifact.blocks.agentRun.notFound")}
        forbiddenMessage={t("liveArtifact.blocks.agentRun.forbidden")}
      />
    </LiveArtifactChrome>
  );
}

export default AgentRunBlock;
