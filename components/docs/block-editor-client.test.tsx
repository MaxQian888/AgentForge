import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

const mockReplaceBlocks = jest.fn();

const mockEditor = {
  document: [
    { id: "block-1", type: "paragraph", content: "AgentForge docs" },
    { id: "block-2", type: "paragraph", content: "Follow-up" },
  ],
  replaceBlocks: mockReplaceBlocks,
};

jest.mock("@blocknote/core", () => ({
  BlockNoteSchema: {
    create: jest.fn(() => ({
      extend: jest.fn(() => "docs-schema"),
    })),
  },
}));

jest.mock("@blocknote/react", () => ({
  useCreateBlockNote: jest.fn(),
}));

jest.mock("@blocknote/shadcn", () => ({
  BlockNoteView: ({
    editable,
    onChange,
  }: {
    editable: boolean;
    onChange: () => void;
  }) => (
    <button type="button" onClick={onChange}>
      BlockNote editable:{String(editable)}
    </button>
  ),
}));

jest.mock("./blocknote-entity-card-block", () => ({
  createEntityCardBlock: jest.fn(() => "entity-block"),
}));

jest.mock("./blocknote-formula-block", () => ({
  createFormulaBlock: jest.fn(() => "formula-block"),
}));

jest.mock("./blocknote-mermaid-block", () => ({
  createMermaidBlock: jest.fn(() => "mermaid-block"),
}));

import { BlockEditorClient } from "./block-editor-client";

const mockUseCreateBlockNote = (jest.requireMock("@blocknote/react") as {
  useCreateBlockNote: jest.Mock;
}).useCreateBlockNote;

describe("BlockEditorClient", () => {
  beforeEach(() => {
    mockReplaceBlocks.mockClear();
    mockUseCreateBlockNote.mockReset();
    mockUseCreateBlockNote.mockReturnValue(mockEditor);
  });

  it("builds the docs schema, syncs parsed content, and emits serialized changes", async () => {
    const user = userEvent.setup();
    const onChange = jest.fn();
    const value = JSON.stringify([{ id: "server-block", type: "paragraph" }]);

    render(
      <BlockEditorClient
        value={value}
        editable={false}
        commentedBlockIds={["block-9", "block-10"]}
        onChange={onChange}
      />,
    );

    expect(mockUseCreateBlockNote).toHaveBeenCalledWith(
      {
        schema: "docs-schema",
        initialContent: [{ id: "server-block", type: "paragraph" }],
      },
      [value],
    );
    expect(mockReplaceBlocks).toHaveBeenCalledWith(mockEditor.document, [
      { id: "server-block", type: "paragraph" },
    ]);
    expect(screen.getByText("Inline comment anchors: block-9, block-10")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "BlockNote editable:false" }));

    expect(onChange).toHaveBeenCalledWith(
      JSON.stringify(mockEditor.document),
      mockEditor.document.map((block) => JSON.stringify(block)).join("\n"),
    );
  });

  it("passes undefined initial content and skips replacement for invalid JSON", () => {
    render(<BlockEditorClient value="not-json" />);

    expect(mockUseCreateBlockNote).toHaveBeenCalledWith(
      {
        schema: "docs-schema",
        initialContent: undefined,
      },
      ["not-json"],
    );
    expect(mockReplaceBlocks).not.toHaveBeenCalled();
    expect(screen.queryByText(/Inline comment anchors/)).not.toBeInTheDocument();
  });
});
