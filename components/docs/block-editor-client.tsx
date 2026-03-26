"use client";

import { useEffect } from "react";
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
  onChange,
}: {
  value: string;
  editable?: boolean;
  commentedBlockIds?: string[];
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

  return (
    <div className="flex flex-col gap-3">
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
