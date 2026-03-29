import React, { Children, isValidElement, type ReactElement, type ReactNode } from "react";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { RuntimeSelector } from "./runtime-selector";

const catalog = {
  defaultRuntime: "codex",
  defaultSelection: {
    runtime: "codex",
    provider: "openai",
    model: "gpt-5-codex",
  },
  runtimes: [
    {
      runtime: "codex",
      label: "Codex",
      defaultProvider: "openai",
      compatibleProviders: ["openai", "codex"],
      defaultModel: "gpt-5-codex",
      available: true,
      diagnostics: [],
    },
    {
      runtime: "claude_code",
      label: "Claude Code",
      defaultProvider: "anthropic",
      compatibleProviders: ["anthropic"],
      defaultModel: "claude-sonnet-4-5",
      available: true,
      diagnostics: [],
    },
    {
      runtime: "opencode",
      label: "OpenCode",
      defaultProvider: "opencode",
      compatibleProviders: ["opencode"],
      defaultModel: "opencode-default",
      available: false,
      diagnostics: [
        {
          code: "missing_cli",
          message: "OpenCode CLI is not installed",
          blocking: true,
        },
      ],
    },
  ],
};

jest.mock("@/components/ui/select", () => {
  function flattenOptions(children: ReactNode): Array<{ value: string; label: string; disabled: boolean }> {
    const options: Array<{ value: string; label: string; disabled: boolean }> = [];

    function visit(node: ReactNode) {
      Children.forEach(node, (child) => {
        if (!isValidElement(child)) {
          return;
        }
        const element = child as ReactElement<{ children?: ReactNode; value?: string; disabled?: boolean }>;
        if (element.props.value !== undefined) {
          options.push({
            value: element.props.value,
            label: typeof element.props.children === "string" ? element.props.children : String(element.props.value),
            disabled: Boolean(element.props.disabled),
          });
          return;
        }
        visit(element.props.children);
      });
    }

    visit(children);
    return options;
  }

  return {
    Select: ({
      value,
      onValueChange,
      disabled,
      children,
    }: {
      value?: string;
      onValueChange?: (value: string) => void;
      disabled?: boolean;
      children?: ReactNode;
    }) => {
      const options = flattenOptions(children);
      return (
        <select
          aria-label="runtime-selector-select"
          value={value}
          disabled={disabled}
          onChange={(event) => onValueChange?.(event.target.value)}
        >
          {options.map((option) => (
            <option key={option.value} value={option.value} disabled={option.disabled}>
              {option.label}
            </option>
          ))}
        </select>
      );
    },
    SelectTrigger: ({ children }: { children?: ReactNode }) => <>{children}</>,
    SelectValue: () => null,
    SelectContent: ({ children }: { children?: ReactNode }) => <>{children}</>,
    SelectItem: ({ children }: { children?: ReactNode }) => <>{children}</>,
  };
});

describe("RuntimeSelector", () => {
  it("filters providers when the runtime changes", async () => {
    const user = userEvent.setup();
    const Wrapper = () => {
      const [value, setValue] = React.useState(catalog.defaultSelection);
      return <RuntimeSelector catalog={catalog} value={value} onChange={setValue} idPrefix="test-runtime" />;
    };

    render(<Wrapper />);

    const selects = screen.getAllByLabelText("runtime-selector-select");
    await user.selectOptions(selects[0], "claude_code");

    expect((selects[1] as HTMLSelectElement).value).toBe("anthropic");
    expect(Array.from((selects[1] as HTMLSelectElement).options).map((option) => option.value)).toEqual(["anthropic"]);
  });

  it("shows diagnostics for unavailable runtimes", () => {
    render(
      <RuntimeSelector
        catalog={catalog}
        value={{ runtime: "opencode", provider: "opencode", model: "opencode-default" }}
        onChange={jest.fn()}
        idPrefix="test-runtime"
      />,
    );

    expect(screen.getByText("OpenCode CLI is not installed")).toBeInTheDocument();
    expect(screen.getByText("OpenCode is currently unavailable.")).toBeInTheDocument();
  });
});
