"use client";

import { useRouter } from "next/navigation";
import { LiveArtifactChrome } from "./live-artifact-chrome";
import { SharedBody } from "./shared-body";
import { useLiveArtifactActions } from "./use-live-artifact-actions";
import type {
  BlockNoteBlock,
  ProjectionResult,
  ReviewTargetRef,
  ReviewViewOpts,
} from "./types";

export interface ReviewBlockProps {
  blockId: string;
  targetRef: ReviewTargetRef | null;
  viewOpts: ReviewViewOpts;
  projection: ProjectionResult | undefined;
  cachedOk?: BlockNoteBlock[];
}

export function ReviewBlock({
  blockId,
  targetRef,
  viewOpts,
  projection,
  cachedOk,
}: ReviewBlockProps) {
  const router = useRouter();
  const actions = useLiveArtifactActions();

  const actionBlock = {
    id: blockId,
    live_kind: "review",
    target_ref: targetRef,
    view_opts: viewOpts,
  };

  const handleOpenSource = () => {
    if (targetRef?.id) {
      router.push(`/reviews?id=${encodeURIComponent(targetRef.id)}`);
    }
    actions.openSource(actionBlock);
  };

  const handleFreeze = () => {
    void actions.freeze(actionBlock);
  };

  const handleRemove = () => actions.remove(actionBlock);

  return (
    <LiveArtifactChrome
      kind="review"
      title="Review"
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
        notFoundMessage="This live artifact is no longer available. The referenced review may have been deleted."
        forbiddenMessage="The details of this review are hidden from your current role."
      />
    </LiveArtifactChrome>
  );
}

export default ReviewBlock;
