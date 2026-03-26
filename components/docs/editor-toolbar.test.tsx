import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { EditorToolbar } from "./editor-toolbar";

describe("EditorToolbar", () => {
  it("wires toolbar actions and disables version saving when saving is in progress", async () => {
    const user = userEvent.setup();
    const onSaveVersion = jest.fn();
    const onSaveTemplate = jest.fn();
    const onShareVersion = jest.fn();

    const { rerender } = render(
      <EditorToolbar
        onSaveVersion={onSaveVersion}
        onSaveTemplate={onSaveTemplate}
        onShareVersion={onShareVersion}
      />,
    );

    await user.click(screen.getByRole("button", { name: /Save Version/i }));
    await user.click(screen.getByRole("button", { name: /Save as Template/i }));
    await user.click(screen.getByRole("button", { name: /Share Version/i }));

    expect(onSaveVersion).toHaveBeenCalledTimes(1);
    expect(onSaveTemplate).toHaveBeenCalledTimes(1);
    expect(onShareVersion).toHaveBeenCalledTimes(1);

    rerender(<EditorToolbar onSaveVersion={onSaveVersion} saving />);
    expect(screen.getByRole("button", { name: /Save Version/i })).toBeDisabled();

    rerender(
      <EditorToolbar
        onSaveVersion={onSaveVersion}
        onSaveTemplate={onSaveTemplate}
        readonly
      />,
    );
    expect(screen.getByRole("button", { name: /Save Version/i })).toBeDisabled();
    expect(screen.getByRole("button", { name: /Save as Template/i })).toBeDisabled();
  });
});
