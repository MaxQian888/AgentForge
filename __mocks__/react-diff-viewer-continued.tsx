import * as React from "react";

interface DiffViewerProps {
  oldValue?: string;
  newValue?: string;
}

function ReactDiffViewer({ oldValue = "", newValue = "" }: DiffViewerProps) {
  return React.createElement(
    "div",
    { "data-testid": "diff-viewer" },
    React.createElement("pre", { "data-testid": "diff-viewer-old" }, oldValue),
    React.createElement("pre", { "data-testid": "diff-viewer-new" }, newValue),
  );
}

export const DiffMethod = {
  CHARS: "chars",
  WORDS: "words",
  WORDS_WITH_SPACE: "words-with-space",
  LINES: "lines",
  TRIMMED_LINES: "trimmed-lines",
  SENTENCES: "sentences",
  CSS: "css",
} as const;

export default ReactDiffViewer;
