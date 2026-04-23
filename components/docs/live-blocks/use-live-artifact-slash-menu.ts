"use client";

import {
  createElement,
  Fragment,
  useCallback,
  useMemo,
  useState,
  type ReactElement,
} from "react";
import { useTranslations } from "next-intl";
import type { KnowledgeAssetKind } from "@/lib/stores/knowledge-store";
import {
  AgentRunPickerDialog,
  CostSummaryFilterDialog,
  ReviewPickerDialog,
  TaskGroupFilterDialog,
  type LiveArtifactInsertSpec,
  type LiveArtifactOnInsert,
} from "./insertion-dialogs";
import type { LiveArtifactKind } from "./types";

/**
 * Shape of slash-menu items returned to the caller. Mirrors BlockNote's
 * `DefaultReactSuggestionItem` closely: `title`, optional `subtext`,
 * `aliases`, `group`, and a required `onItemClick` callback. We don't import
 * the BlockNote type directly so this file can be unit-tested without
 * spinning up the editor.
 */
export interface LiveArtifactSlashMenuItem {
  key: LiveArtifactKind;
  title: string;
  subtext: string;
  aliases: string[];
  group: string;
  onItemClick: () => void;
}

export interface UseLiveArtifactSlashMenuResult {
  slashMenuItems: LiveArtifactSlashMenuItem[];
  menuDialogs: ReactElement | null;
  /** The kind whose dialog is currently open (or null). Useful for testing. */
  openDialog: LiveArtifactKind | null;
}

export interface UseLiveArtifactSlashMenuOptions {
  /** Asset kind currently being edited. Entries are hidden unless "wiki_page". */
  assetKind: KnowledgeAssetKind | null | undefined;
  /** Current project id, forwarded to each dialog for data scoping. */
  projectId?: string | null;
  /**
   * Called after the user confirms a dialog. The caller is expected to
   * translate the plain-object spec into a BlockNote `insertBlocks` call
   * (with `target_ref` / `view_opts` stringified into block props).
   */
  onInsert: LiveArtifactOnInsert;
}

/**
 * React hook that exposes slash-menu entries for inserting live-artifact
 * blocks plus the dialog elements the caller must render alongside the
 * editor.
 *
 * Asset-kind gate (§11.6): only wiki pages can host live-artifact blocks.
 * When `assetKind !== "wiki_page"`, the hook returns an empty items array
 * and `menuDialogs === null`, so the slash menu never advertises the
 * entries and no dialog state is held.
 */
export function useLiveArtifactSlashMenu(
  options: UseLiveArtifactSlashMenuOptions,
): UseLiveArtifactSlashMenuResult {
  const { assetKind, projectId, onInsert } = options;
  const enabled = assetKind === "wiki_page";
  const t = useTranslations("docs");

  const [openDialog, setOpenDialog] = useState<LiveArtifactKind | null>(null);

  const close = useCallback(() => setOpenDialog(null), []);

  const forward = useCallback<LiveArtifactOnInsert>(
    (spec: LiveArtifactInsertSpec) => {
      onInsert(spec);
      setOpenDialog(null);
    },
    [onInsert],
  );

  const slashMenuItems = useMemo<LiveArtifactSlashMenuItem[]>(() => {
    if (!enabled) return [];
    const group = t("liveArtifact.slashMenu.group");
    return [
      {
        key: "agent_run",
        title: t("liveArtifact.slashMenu.agentRun.title"),
        subtext: t("liveArtifact.slashMenu.agentRun.subtext"),
        aliases: ["agent", "run", "live"],
        group,
        onItemClick: () => setOpenDialog("agent_run"),
      },
      {
        key: "cost_summary",
        title: t("liveArtifact.slashMenu.costSummary.title"),
        subtext: t("liveArtifact.slashMenu.costSummary.subtext"),
        aliases: ["cost", "spend", "usage", "live"],
        group,
        onItemClick: () => setOpenDialog("cost_summary"),
      },
      {
        key: "review",
        title: t("liveArtifact.slashMenu.review.title"),
        subtext: t("liveArtifact.slashMenu.review.subtext"),
        aliases: ["review", "pr", "live"],
        group,
        onItemClick: () => setOpenDialog("review"),
      },
      {
        key: "task_group",
        title: t("liveArtifact.slashMenu.taskGroup.title"),
        subtext: t("liveArtifact.slashMenu.taskGroup.subtext"),
        aliases: ["tasks", "list", "live", "group"],
        group,
        onItemClick: () => setOpenDialog("task_group"),
      },
    ];
  }, [enabled, t]);

  const menuDialogs = useMemo<ReactElement | null>(() => {
    if (!enabled) return null;
    const onOpenChange = (next: boolean) => {
      if (!next) close();
    };
    const commonProps = {
      onOpenChange,
      projectId: projectId ?? null,
      onInsert: forward,
    };
    return createElement(
      Fragment,
      null,
      createElement(AgentRunPickerDialog, {
        key: "agent_run",
        open: openDialog === "agent_run",
        ...commonProps,
      }),
      createElement(CostSummaryFilterDialog, {
        key: "cost_summary",
        open: openDialog === "cost_summary",
        ...commonProps,
      }),
      createElement(ReviewPickerDialog, {
        key: "review",
        open: openDialog === "review",
        ...commonProps,
      }),
      createElement(TaskGroupFilterDialog, {
        key: "task_group",
        open: openDialog === "task_group",
        ...commonProps,
      }),
    );
  }, [enabled, openDialog, close, projectId, forward]);

  return { slashMenuItems, menuDialogs, openDialog };
}
