"use client";

import { useTranslations } from "next-intl";
import { ProjectionFragment } from "./projection-fragment";
import type { BlockNoteBlock, ProjectionResult } from "./types";

export interface SharedBodyProps {
  projection: ProjectionResult | undefined;
  cachedOk?: BlockNoteBlock[];
  onRemove: () => void;
  /** Kind-specific copy shown in `not_found` state. */
  notFoundMessage: string;
  /** Kind-specific copy shown in `forbidden` state. */
  forbiddenMessage: string;
}

/**
 * Shared body renderer per projection status. Each kind block composes
 * this with kind-specific copy; the chrome itself stays kind-neutral.
 */
export function SharedBody({
  projection,
  cachedOk,
  onRemove,
  notFoundMessage,
  forbiddenMessage,
}: SharedBodyProps) {
  const t = useTranslations("docs");
  if (!projection) {
    return <Shimmer />;
  }
  if (projection.status === "ok") {
    return <ProjectionFragment blocks={projection.projection} />;
  }
  if (projection.status === "degraded") {
    if (cachedOk && cachedOk.length > 0) {
      return <ProjectionFragment blocks={cachedOk} dimmed />;
    }
    return (
      <p className="text-xs italic text-muted-foreground">
        {t("liveArtifact.sharedBody.lastKnownUnavailable")}
      </p>
    );
  }
  if (projection.status === "not_found") {
    return (
      <div className="flex items-center justify-between gap-3 text-xs text-muted-foreground">
        <span>{notFoundMessage}</span>
        <button
          type="button"
          onClick={onRemove}
          className="rounded-md border px-2 py-1 text-xs font-medium hover:bg-accent"
        >
          {t("liveArtifact.sharedBody.removeBlock")}
        </button>
      </div>
    );
  }
  return (
    <p className="text-xs italic text-muted-foreground">{forbiddenMessage}</p>
  );
}

export function Shimmer() {
  const t = useTranslations("docs");
  return (
    <div
      className="flex flex-col gap-2"
      role="status"
      aria-label={t("liveArtifact.sharedBody.loadingLabel")}
    >
      <div className="h-4 w-32 animate-pulse rounded bg-muted" />
      <div className="h-3 w-full animate-pulse rounded bg-muted" />
      <div className="h-3 w-[85%] animate-pulse rounded bg-muted" />
      <div className="h-3 w-[72%] animate-pulse rounded bg-muted" />
    </div>
  );
}
