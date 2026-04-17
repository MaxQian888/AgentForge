"use client";

import { toast } from "sonner";
import { createApiClient } from "@/lib/api-client";
import {
  useLiveArtifactContext,
  type LiveArtifactActionBlock,
} from "./live-artifact-context";

/**
 * Returns memoised wrappers for Open source / Freeze / Remove around the
 * provider's actions. Each kind block supplies its own Open source
 * navigation; freeze and remove are generic.
 */
export function useLiveArtifactActions() {
  const context = useLiveArtifactContext();

  const remove = (block: LiveArtifactActionBlock) => {
    context.actions.remove(block);
  };

  const freeze = async (block: LiveArtifactActionBlock) => {
    context.actions.freeze(block);
    const { apiUrl, token, projectId, assetId } = context;
    if (!apiUrl || !token || !projectId || !assetId) return;
    try {
      const api = createApiClient(apiUrl);
      await api.post(
        `/api/v1/projects/${projectId}/knowledge/assets/${assetId}/live-artifacts/${block.id}/freeze`,
        {},
        { token },
      );
      toast.success("Block frozen");
      context.actions.onFreezeComplete?.(block);
    } catch (error) {
      toast.error(
        error instanceof Error ? error.message : "Failed to freeze block",
      );
    }
  };

  const openSource = (block: LiveArtifactActionBlock) => {
    context.actions.openSource(block);
  };

  return { openSource, freeze, remove };
}
