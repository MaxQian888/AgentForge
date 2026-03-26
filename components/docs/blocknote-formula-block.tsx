"use client";

import katex from "katex";
import "katex/dist/katex.min.css";
import { defaultProps } from "@blocknote/core";
import { BlockContentWrapper, createReactBlockSpec } from "@blocknote/react";

const formulaConfig = {
  type: "formula",
  propSchema: {
    textAlignment: defaultProps.textAlignment,
    formula: { default: "c = \\pm\\sqrt{a^2 + b^2}" },
  },
  content: "none" as const,
};

export const createFormulaBlock = createReactBlockSpec(
  formulaConfig as never,
  {
    render: ({ block }) => {
      const formula = String((block as { props?: { formula?: string } }).props?.formula ?? "");
      const html = katex.renderToString(formula, {
        throwOnError: false,
        displayMode: true,
      });
      return (
        <BlockContentWrapper
          blockType="formula"
          blockProps={(block as { props: Record<string, unknown> }).props as never}
          propSchema={formulaConfig.propSchema as never}
        >
          <div
            className="rounded-lg border border-emerald-500/20 bg-emerald-500/5 p-4"
            dangerouslySetInnerHTML={{ __html: html }}
          />
        </BlockContentWrapper>
      );
    },
  }
);
