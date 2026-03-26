import { render, screen } from "@testing-library/react";

jest.mock("next/dynamic", () => ({
  __esModule: true,
  default: jest.fn(
    (
      _loader: unknown,
      options: {
        ssr?: boolean;
        loading?: () => JSX.Element;
      },
    ) =>
      function MockDynamicBlockEditor(props: {
        value: string;
        editable?: boolean;
        commentedBlockIds?: string[];
      }) {
        return (
          <div
            data-testid="dynamic-block-editor"
            data-ssr={String(options.ssr)}
            data-has-loading={String(Boolean(options.loading))}
          >
            {JSON.stringify(props)}
          </div>
        );
      },
  ),
}));

import { BlockEditor } from "./block-editor";

describe("BlockEditor", () => {
  it("configures a client-only dynamic editor and forwards props", () => {
    render(
      <BlockEditor
        value='[{"id":"block-1"}]'
        editable={false}
        commentedBlockIds={["block-1"]}
      />,
    );

    expect(screen.getByTestId("dynamic-block-editor")).toHaveAttribute("data-ssr", "false");
    expect(screen.getByTestId("dynamic-block-editor")).toHaveAttribute(
      "data-has-loading",
      "true",
    );
    expect(screen.getByTestId("dynamic-block-editor")).toHaveTextContent(
      '{"value":"[{\\"id\\":\\"block-1\\"}]","editable":false,"commentedBlockIds":["block-1"]}',
    );
  });
});
