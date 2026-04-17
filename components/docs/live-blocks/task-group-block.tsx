"use client";

import { useRouter } from "next/navigation";
import { LiveArtifactChrome } from "./live-artifact-chrome";
import { SharedBody } from "./shared-body";
import { useLiveArtifactActions } from "./use-live-artifact-actions";
import type {
  BlockNoteBlock,
  ProjectionResult,
  TaskGroupTargetRef,
  TaskGroupViewOpts,
} from "./types";

export interface TaskGroupBlockProps {
  blockId: string;
  targetRef: TaskGroupTargetRef | null;
  viewOpts: TaskGroupViewOpts;
  projection: ProjectionResult | undefined;
  cachedOk?: BlockNoteBlock[];
}

export function TaskGroupBlock({
  blockId,
  targetRef,
  viewOpts,
  projection,
  cachedOk,
}: TaskGroupBlockProps) {
  const router = useRouter();
  const actions = useLiveArtifactActions();

  const actionBlock = {
    id: blockId,
    live_kind: "task_group",
    target_ref: targetRef,
    view_opts: viewOpts,
  };

  const handleOpenSource = () => {
    const filter = targetRef?.filter;
    if (filter?.saved_view_id) {
      router.push(`/tasks?view=${encodeURIComponent(filter.saved_view_id)}`);
    } else if (filter?.inline) {
      const params = new URLSearchParams();
      const inline = filter.inline;
      if (inline.status?.length) params.set("status", inline.status.join(","));
      if (inline.assignee_id) params.set("assignee_id", inline.assignee_id);
      if (inline.labels?.length) params.set("labels", inline.labels.join(","));
      if (inline.sprint_id) params.set("sprint_id", inline.sprint_id);
      if (inline.milestone_id) params.set("milestone_id", inline.milestone_id);
      const query = params.toString();
      router.push(query ? `/tasks?${query}` : "/tasks");
    } else {
      router.push("/tasks");
    }
    actions.openSource(actionBlock);
  };

  const handleFreeze = () => {
    void actions.freeze(actionBlock);
  };

  const handleRemove = () => actions.remove(actionBlock);

  return (
    <LiveArtifactChrome
      kind="task_group"
      title="Task group"
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
        notFoundMessage="This live artifact is no longer available. The referenced saved view or filter may have been removed."
        forbiddenMessage="You do not have access to the tasks matched by this filter."
      />
    </LiveArtifactChrome>
  );
}

export default TaskGroupBlock;
