"use client";

import { cn } from "@/lib/utils";
import type { BlockNoteBlock } from "./types";

/**
 * Render a BlockNote JSON fragment read-only as plain React.
 *
 * This is a minimal renderer tuned for the live-artifact projection
 * output — the projectors currently emit `heading`, `paragraph`,
 * `callout`, and `table` blocks. Unknown block types degrade to a
 * neutral fallback so partial outputs still render.
 */
export function ProjectionFragment({
  blocks,
  dimmed = false,
  className,
}: {
  blocks: BlockNoteBlock[] | undefined;
  dimmed?: boolean;
  className?: string;
}) {
  if (!blocks || blocks.length === 0) {
    return (
      <p
        className={cn(
          "text-sm italic text-muted-foreground",
          dimmed && "opacity-60",
          className,
        )}
      >
        No content to display.
      </p>
    );
  }
  return (
    <div
      className={cn(
        "flex flex-col gap-2 text-sm leading-relaxed",
        dimmed && "opacity-60",
        className,
      )}
    >
      {blocks.map((block, idx) => (
        <ProjectionBlock
          key={String(block.id ?? idx)}
          block={block}
        />
      ))}
    </div>
  );
}

function ProjectionBlock({ block }: { block: BlockNoteBlock }) {
  const type = String(block.type ?? "");
  switch (type) {
    case "heading":
      return <ProjectionHeading block={block} />;
    case "paragraph":
      return <ProjectionParagraph block={block} />;
    case "callout":
      return <ProjectionCallout block={block} />;
    case "table":
      return <ProjectionTable block={block} />;
    default:
      return (
        <div className="text-xs text-muted-foreground">
          [unsupported block: {type}]
        </div>
      );
  }
}

function extractText(content: unknown): string {
  if (typeof content === "string") return content;
  if (Array.isArray(content)) {
    return content
      .map((node) => {
        if (!node) return "";
        if (typeof node === "string") return node;
        if (typeof node === "object") {
          const obj = node as { text?: unknown; content?: unknown };
          if (typeof obj.text === "string") return obj.text;
          if (obj.content !== undefined) return extractText(obj.content);
        }
        return "";
      })
      .join("");
  }
  if (content && typeof content === "object") {
    const obj = content as { text?: unknown };
    if (typeof obj.text === "string") return obj.text;
  }
  return "";
}

function ProjectionHeading({ block }: { block: BlockNoteBlock }) {
  const level = Number(
    (block.props as { level?: unknown } | undefined)?.level ?? 2,
  );
  const text = extractText(block.content);
  if (level <= 1) {
    return <h1 className="text-xl font-semibold">{text}</h1>;
  }
  if (level === 2) {
    return <h2 className="text-lg font-semibold">{text}</h2>;
  }
  return <h3 className="text-base font-semibold">{text}</h3>;
}

function ProjectionParagraph({ block }: { block: BlockNoteBlock }) {
  const text = extractText(block.content);
  return <p className="text-sm text-foreground/90">{text}</p>;
}

function ProjectionCallout({ block }: { block: BlockNoteBlock }) {
  const text = extractText(block.content);
  return (
    <div className="rounded-md border border-border bg-muted/40 px-3 py-2 text-sm">
      {text}
    </div>
  );
}

function ProjectionTable({ block }: { block: BlockNoteBlock }) {
  const content = block.content as
    | {
        rows?: Array<{
          cells?: Array<unknown>;
        }>;
      }
    | undefined;
  const rows = Array.isArray(content?.rows) ? content!.rows! : [];
  if (rows.length === 0) {
    return (
      <pre className="overflow-x-auto rounded bg-muted/40 p-2 text-xs">
        {JSON.stringify(block, null, 2)}
      </pre>
    );
  }
  const [header, ...body] = rows;
  return (
    <div className="overflow-x-auto">
      <table className="w-full border-collapse text-xs">
        {header?.cells ? (
          <thead>
            <tr>
              {header.cells.map((cell, idx) => (
                <th
                  key={idx}
                  className="border border-border/60 bg-muted/50 px-2 py-1 text-left font-medium"
                >
                  {extractText(cell)}
                </th>
              ))}
            </tr>
          </thead>
        ) : null}
        <tbody>
          {body.map((row, rIdx) => (
            <tr key={rIdx}>
              {(row.cells ?? []).map((cell, cIdx) => (
                <td
                  key={cIdx}
                  className="border border-border/60 px-2 py-1"
                >
                  {extractText(cell)}
                </td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
