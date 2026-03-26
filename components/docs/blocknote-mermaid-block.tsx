"use client";

import { useEffect, useState } from "react";
import mermaid from "mermaid";
import { defaultProps } from "@blocknote/core";
import { BlockContentWrapper, createReactBlockSpec } from "@blocknote/react";

const mermaidConfig = {
  type: "mermaid",
  propSchema: {
    textAlignment: defaultProps.textAlignment,
    chart: { default: "graph TD; Start-->Docs; Docs-->Done;" },
  },
  content: "none" as const,
};

function MermaidPreview({ chart }: { chart: string }) {
  const [svg, setSvg] = useState("");

  useEffect(() => {
    let cancelled = false;
    mermaid.initialize({ startOnLoad: false, theme: "default" });
    void mermaid
      .render(`diagram-${Math.random().toString(36).slice(2)}`, chart || "graph TD;A-->B")
      .then((result) => {
        if (!cancelled) {
          setSvg(result.svg);
        }
      })
      .catch(() => {
        if (!cancelled) {
          setSvg("");
        }
      });
    return () => {
      cancelled = true;
    };
  }, [chart]);

  if (!svg) {
    return <pre className="overflow-x-auto rounded-md bg-muted/60 p-3 text-xs">{chart}</pre>;
  }

  return <div dangerouslySetInnerHTML={{ __html: svg }} />;
}

export const createMermaidBlock = createReactBlockSpec(
  mermaidConfig as never,
  {
    render: ({ block }) => {
      const chart = String((block as { props?: { chart?: string } }).props?.chart ?? "");
      return (
        <BlockContentWrapper
          blockType="mermaid"
          blockProps={(block as { props: Record<string, unknown> }).props as never}
          propSchema={mermaidConfig.propSchema as never}
        >
          <div className="rounded-lg border border-sky-500/20 bg-sky-500/5 p-4">
            <MermaidPreview chart={chart} />
          </div>
        </BlockContentWrapper>
      );
    },
  }
);
