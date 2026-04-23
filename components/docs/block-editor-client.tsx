"use client";

import { useEffect, useMemo, useState, type ReactNode } from "react";
import { useTranslations } from "next-intl";
import { BlockNoteSchema, filterSuggestionItems } from "@blocknote/core";
import {
  SuggestionMenuController,
  getDefaultReactSlashMenuItems,
  useCreateBlockNote,
} from "@blocknote/react";
import { BlockNoteView } from "@blocknote/shadcn";
import "@blocknote/core/fonts/inter.css";
import "@blocknote/shadcn/style.css";
import { createEntityCardBlock } from "./blocknote-entity-card-block";
import { createFormulaBlock } from "./blocknote-formula-block";
import { createMermaidBlock } from "./blocknote-mermaid-block";
import { createLiveArtifactBlock } from "./live-blocks/live-artifact-block";
import {
  LiveArtifactProvider,
  type LiveArtifactContextValue,
} from "./live-blocks/live-artifact-context";

const docsSchema = BlockNoteSchema.create().extend({
  blockSpecs: {
    formula: createFormulaBlock(),
    mermaid: createMermaidBlock(),
    entityCard: createEntityCardBlock(),
    liveArtifact: createLiveArtifactBlock(),
  },
});

function parseContent(content: string) {
  try {
    const parsed = JSON.parse(content);
    return Array.isArray(parsed) ? parsed : undefined;
  } catch {
    return undefined;
  }
}

export interface BlockEditorClientProps {
  value: string;
  editable?: boolean;
  commentedBlockIds?: string[];
  taskCountsByBlock?: Record<string, number>;
  onCreateTasksFromSelection?: (blockIds: string[]) => void;
  onChange?: (content: string, contentText: string) => void;
  /**
   * Extra slash-menu items (live-artifact insertion entries). Pass the array
   * returned by `useLiveArtifactSlashMenu`. When present, the default slash
   * menu is augmented with these items.
   */
  extraSlashMenuItems?: readonly unknown[];
  /**
   * Elements that must be rendered alongside the editor (dialogs opened by
   * slash-menu items). Pass the `menuDialogs` node from
   * `useLiveArtifactSlashMenu`.
   */
  slashMenuDialogs?: ReactNode;
  /**
   * If provided, wraps the editor in a `LiveArtifactProvider` so embedded
   * live-artifact blocks receive projections + actions. Supply the hook
   * output from `useLiveArtifactProjections` plus `{assetId, projectId,
   * token, apiUrl}`.
   */
  liveArtifactValue?: Partial<LiveArtifactContextValue>;
}

export function BlockEditorClient({
  value,
  editable = true,
  commentedBlockIds = [],
  taskCountsByBlock = {},
  onCreateTasksFromSelection,
  onChange,
  extraSlashMenuItems,
  slashMenuDialogs,
  liveArtifactValue,
}: BlockEditorClientProps) {
  const editor = useCreateBlockNote(
    {
      schema: docsSchema,
      initialContent: parseContent(value),
    },
    [value]
  );

  useEffect(() => {
    if (!editor || !value) return;
    const parsed = parseContent(value);
    if (parsed) {
      editor.replaceBlocks(editor.document, parsed);
    }
  }, [editor, value]);

  const t = useTranslations("docs");
  const [selectedBlockIds, setSelectedBlockIds] = useState<string[]>([]);
  const blocks = useMemo(
    () =>
      editor.document.map((block) => ({
        id: String((block as { id?: unknown }).id ?? ""),
        label:
          typeof (block as { content?: unknown }).content === "string"
            ? String((block as { content?: unknown }).content ?? "")
            : JSON.stringify(block),
      })),
    [editor.document],
  );

  const hasLiveArtifacts = Boolean(
    extraSlashMenuItems && extraSlashMenuItems.length > 0,
  );
  const usesProvider = Boolean(liveArtifactValue);

  const editorBody = (
    <BlockNoteView
      editor={editor}
      editable={editable}
      slashMenu={!hasLiveArtifacts}
      onChange={() => {
        const serialized = JSON.stringify(editor.document);
        const plainText = editor.document
          .map((block) => JSON.stringify(block))
          .join("\n");
        onChange?.(serialized, plainText);
      }}
    >
      {hasLiveArtifacts ? (
        <SuggestionMenuController
          triggerCharacter="/"
          getItems={async (query: string) =>
            filterSuggestionItems(
              [
                ...getDefaultReactSlashMenuItems(editor),
                ...(extraSlashMenuItems ?? []),
              ] as never,
              query,
            )
          }
        />
      ) : null}
    </BlockNoteView>
  );

  return (
    <div className="flex flex-col gap-3">
      {editable && blocks.length > 0 ? (
        <div className="rounded-lg border border-border/60 bg-muted/30 px-3 py-2 text-xs text-muted-foreground">
          <div className="flex items-center justify-between gap-2">
            <span className="font-medium text-foreground">{t("blockEditor.selectedBlocks")}</span>
            <button
              type="button"
              className="rounded-md border px-2 py-1 text-xs hover:bg-accent"
              disabled={selectedBlockIds.length === 0}
              onClick={() => onCreateTasksFromSelection?.(selectedBlockIds)}
            >
              {t("blockEditor.createTasks")}
            </button>
          </div>
          <div className="mt-2 space-y-2">
            {blocks.map((block) => (
              <label key={block.id} className="flex items-center justify-between gap-3">
                <span className="flex items-center gap-2">
                  <input
                    type="checkbox"
                    checked={selectedBlockIds.includes(block.id)}
                    onChange={(event) =>
                      setSelectedBlockIds((current) =>
                        event.target.checked
                          ? [...current, block.id]
                          : current.filter((id) => id !== block.id),
                      )
                    }
                  />
                  <span>{block.id}</span>
                </span>
                <span className="flex items-center gap-2">
                  {taskCountsByBlock[block.id] ? (
                    <span className="rounded-full border px-2 py-0.5">
                      {t("blockEditor.taskCount", { count: taskCountsByBlock[block.id] })}
                    </span>
                  ) : null}
                  <span className="truncate">{block.label}</span>
                </span>
              </label>
            ))}
          </div>
        </div>
      ) : null}
      {commentedBlockIds.length > 0 ? (
        <div className="rounded-lg border border-border/60 bg-muted/30 px-3 py-2 text-xs text-muted-foreground">
          {t("blockEditor.inlineCommentAnchors")} {commentedBlockIds.join(", ")}
        </div>
      ) : null}
      {usesProvider ? (
        <LiveArtifactProvider value={liveArtifactValue}>
          {editorBody}
        </LiveArtifactProvider>
      ) : (
        editorBody
      )}
      {slashMenuDialogs}
    </div>
  );
}
