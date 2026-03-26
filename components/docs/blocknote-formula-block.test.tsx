import { render, screen } from "@testing-library/react";

jest.mock("katex", () => ({
  __esModule: true,
  default: {
    renderToString: jest.fn(
      (formula: string) => `<span data-testid="katex-output">${formula}</span>`,
    ),
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
          block: { props?: { formula?: string } };
        }) => JSX.Element;
      },
    ) => ({
      config,
      render: implementation.render,
    }),
  ),
}));

import { createFormulaBlock } from "./blocknote-formula-block";
const mockRenderToString = (jest.requireMock("katex") as {
  default: { renderToString: jest.Mock };
}).default.renderToString;

describe("createFormulaBlock", () => {
  it("declares a formula block spec and renders KaTeX output safely", () => {
    const formulaBlock = createFormulaBlock as unknown as {
      config: unknown;
      render: (input: { block: { props?: { formula?: string } } }) => JSX.Element;
    };

    expect(formulaBlock.config).toEqual(
      expect.objectContaining({
        type: "formula",
        content: "none",
      }),
    );

    render(
      formulaBlock.render({
        block: {
          props: { formula: "E=mc^2" },
        },
      }),
    );

    expect(mockRenderToString).toHaveBeenCalledWith("E=mc^2", {
      throwOnError: false,
      displayMode: true,
    });
    expect(screen.getByTestId("katex-output")).toHaveTextContent("E=mc^2");
  });
});
