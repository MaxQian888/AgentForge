"use client";

import { useEffect, useMemo, useState } from "react";
import { BlockNoteSchema } from "@blocknote/core";
import { useCreateBlockNote } from "@blocknote/react";
import { BlockNoteView } from "@blocknote/shadcn";
import "@blocknote/core/fonts/inter.css";
import "@blocknote/shadcn/style.css";
import { createEntityCardBlock } from "./blocknote-entity-card-block";
import { createFormulaBlock } from "./blocknote-formula-block";
import { createMermaidBlock } from "./blocknote-mermaid-block";

const docsSchema = BlockNoteSchema.create().extend({
  blockSpecs: {
    formula: createFormulaBlock(),
    mermaid: createMermaidBlock(),
    entityCard: createEntityCardBlock(),
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

export function BlockEditorClient({
  value,
  editable = true,
  commentedBlockIds = [],
  taskCountsByBlock = {},
  onCreateTasksFromSelection,
  onChange,
}: {
  value: string;
  editable?: boolean;
  commentedBlockIds?: string[];
  taskCountsByBlock?: Record<string, number>;
  onCreateTasksFromSelection?: (blockIds: string[]) => void;
  onChange?: (content: string, contentText: string) => void;
}) {
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

  return (
    <div className="flex flex-col gap-3">
      {editable && blocks.length > 0 ? (
        <div className="rounded-lg border border-border/60 bg-muted/30 px-3 py-2 text-xs text-muted-foreground">
          <div className="flex items-center justify-between gap-2">
            <span className="font-medium text-foreground">Selected blocks</span>
            <button
              type="button"
              className="rounded-md border px-2 py-1 text-xs hover:bg-accent"
              disabled={selectedBlockIds.length === 0}
              onClick={() => onCreateTasksFromSelection?.(selectedBlockIds)}
            >
              Create Tasks
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
                      {taskCountsByBlock[block.id]} tasks
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
          Inline comment anchors: {commentedBlockIds.join(", ")}
        </div>
      ) : null}
      <BlockNoteView
        editor={editor}
        editable={editable}
        onChange={() => {
          const serialized = JSON.stringify(editor.document);
          const plainText = editor.document
            .map((block) => JSON.stringify(block))
            .join("\n");
          onChange?.(serialized, plainText);
        }}
      />
    </div>
  );
}
