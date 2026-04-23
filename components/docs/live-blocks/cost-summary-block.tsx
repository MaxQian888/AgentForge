"use client";

import { useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import { LiveArtifactChrome } from "./live-artifact-chrome";
import { SharedBody } from "./shared-body";
import { useLiveArtifactActions } from "./use-live-artifact-actions";
import type {
  BlockNoteBlock,
  CostSummaryTargetRef,
  CostSummaryViewOpts,
  ProjectionResult,
} from "./types";

export interface CostSummaryBlockProps {
  blockId: string;
  targetRef: CostSummaryTargetRef | null;
  viewOpts: CostSummaryViewOpts;
  projection: ProjectionResult | undefined;
  cachedOk?: BlockNoteBlock[];
}

export function CostSummaryBlock({
  blockId,
  targetRef,
  viewOpts,
  projection,
  cachedOk,
}: CostSummaryBlockProps) {
  const t = useTranslations("docs");
  const router = useRouter();
  const actions = useLiveArtifactActions();

  const actionBlock = {
    id: blockId,
    live_kind: "cost_summary",
    target_ref: targetRef,
    view_opts: viewOpts,
  };

  const handleOpenSource = () => {
    const filter = targetRef?.filter;
    if (filter) {
      const params = new URLSearchParams();
      if (filter.range_start) params.set("range_start", filter.range_start);
      if (filter.range_end) params.set("range_end", filter.range_end);
      if (filter.runtime) params.set("runtime", filter.runtime);
      if (filter.provider) params.set("provider", filter.provider);
      if (filter.member_id) params.set("member_id", filter.member_id);
      const query = params.toString();
      router.push(query ? `/cost?${query}` : "/cost");
    } else {
      router.push("/cost");
    }
    actions.openSource(actionBlock);
  };

  const handleFreeze = () => {
    void actions.freeze(actionBlock);
  };

  const handleRemove = () => actions.remove(actionBlock);

  return (
    <LiveArtifactChrome
      kind="cost_summary"
      title={t("liveArtifact.blocks.costSummary.title")}
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
        notFoundMessage={t("liveArtifact.blocks.costSummary.notFound")}
        forbiddenMessage={t("liveArtifact.blocks.costSummary.forbidden")}
      />
    </LiveArtifactChrome>
  );
}

export default CostSummaryBlock;
