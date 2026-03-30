jest.mock("next-intl", () => ({
  useTranslations: () => (key: string) => {
    const map: Record<string, string> = {
      "docPicker.title": "Link a document",
      "docPicker.description": "Choose a document to attach",
      "docPicker.empty": "No documents available",
      "docPicker.close": "Close",
    };
    return map[key] ?? key;
  },
}));

import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { DocLinkPicker } from "./doc-link-picker";

describe("DocLinkPicker", () => {
  it("renders docs, supports selection, and can close the picker", async () => {
    const user = userEvent.setup();
    const onPick = jest.fn();
    const onOpenChange = jest.fn();

    render(
      <DocLinkPicker
        open
        onOpenChange={onOpenChange}
        docs={[{ id: "doc-1", title: "Runbook", path: "/docs/runbook" }]}
        onPick={onPick}
      />,
    );

    expect(screen.getByText("Link a document")).toBeInTheDocument();
    expect(screen.getByText("/docs/runbook")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: /Runbook/i }));
    const dialog = screen.getByRole("dialog");
    await user.click(within(dialog).getAllByRole("button", { name: "Close" })[0]);

    expect(onPick).toHaveBeenCalledWith("doc-1");
    expect(onOpenChange).toHaveBeenCalledWith(false);
  });

  it("renders the empty state when there are no documents", () => {
    render(
      <DocLinkPicker
        open
        onOpenChange={jest.fn()}
        docs={[]}
        onPick={jest.fn()}
      />,
    );

    expect(screen.getByText("No documents available")).toBeInTheDocument();
  });
});
