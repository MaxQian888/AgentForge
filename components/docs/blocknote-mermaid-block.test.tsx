import { render, screen, waitFor } from "@testing-library/react";

jest.mock("mermaid", () => ({
  __esModule: true,
  default: {
    initialize: jest.fn(),
    render: jest.fn(),
  },
}));

jest.mock("@blocknote/core", () => ({
  defaultProps: {
    textAlignment: { default: "left" },
  },
}));

jest.mock("@blocknote/react", () => ({
  BlockContentWrapper: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="block-content-wrapper">{children}</div>
  ),
  createReactBlockSpec: jest.fn(
    (
      config: unknown,
      implementation: {
        render: (input: {
          block: { props?: { chart?: string } };
        }) => React.ReactElement;
      },
    ) => ({
      config,
      render: implementation.render,
    }),
  ),
}));

import { createMermaidBlock } from "./blocknote-mermaid-block";
const mermaidMock = jest.requireMock("mermaid") as {
  default: { initialize: jest.Mock; render: jest.Mock };
};
const mockInitialize = mermaidMock.default.initialize;
const mockRenderDiagram = mermaidMock.default.render;

describe("createMermaidBlock", () => {
  beforeEach(() => {
    mockInitialize.mockClear();
    mockRenderDiagram.mockReset();
  });

  it("declares the mermaid block spec and renders diagram SVG when Mermaid succeeds", async () => {
    mockRenderDiagram.mockResolvedValue({ svg: "<svg><title>Docs graph</title></svg>" });

    const mermaidBlock = createMermaidBlock as unknown as {
      config: unknown;
      render: (input: { block: { props?: { chart?: string } } }) => React.ReactElement;
    };

    expect(mermaidBlock.config).toEqual(
      expect.objectContaining({
        type: "mermaid",
        content: "none",
      }),
    );

    const { container } = render(
      mermaidBlock.render({
        block: {
          props: { chart: "graph TD; Docs-->Done;" },
        },
      }),
    );

    expect(mockInitialize).toHaveBeenCalledWith({ startOnLoad: false, theme: "default" });
    await waitFor(() => {
      expect(container.querySelector("svg")).toBeInTheDocument();
    });
    expect(screen.getByText("Docs graph")).toBeInTheDocument();
  });

  it("falls back to source text when Mermaid rendering fails", async () => {
    mockRenderDiagram.mockRejectedValue(new Error("render failed"));

    const mermaidBlock = createMermaidBlock as unknown as {
      render: (input: { block: { props?: { chart?: string } } }) => React.ReactElement;
    };

    render(
      mermaidBlock.render({
        block: {
          props: { chart: "graph TD; A-->B" },
        },
      }),
    );

    await waitFor(() => {
      expect(screen.getByText("graph TD; A-->B")).toBeInTheDocument();
    });
  });
});
