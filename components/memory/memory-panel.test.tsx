const searchMemory = jest.fn().mockResolvedValue(undefined);
const deleteMemory = jest.fn().mockResolvedValue(undefined);

const storeState = {
  entries: [
    {
      id: "memory-1",
      projectId: "project-1",
      scope: "project" as const,
      roleId: "",
      category: "semantic" as const,
      key: "design-note",
      content: "Keep the review queue stable.",
      metadata: "{}",
      relevanceScore: 0.9,
      accessCount: 2,
      createdAt: "2026-03-25T08:00:00.000Z",
    },
  ],
  loading: false,
  searchMemory,
  deleteMemory,
};

jest.mock("@/lib/stores/memory-store", () => ({
  useMemoryStore: (selector: (state: typeof storeState) => unknown) =>
    selector(storeState),
}));

import userEvent from "@testing-library/user-event";
import { render, screen, waitFor } from "@testing-library/react";
import { MemoryPanel } from "./memory-panel";

describe("MemoryPanel", () => {
  beforeEach(() => {
    searchMemory.mockClear();
    deleteMemory.mockClear();
  });

  it("searches on mount and when filters change, and supports deleting entries", async () => {
    const user = userEvent.setup();
    const { container } = render(<MemoryPanel projectId="project-1" />);

    await waitFor(() =>
      expect(searchMemory).toHaveBeenCalledWith("project-1", undefined, undefined, undefined),
    );

    expect(screen.getByText("design-note")).toBeInTheDocument();
    expect(screen.getByText("Keep the review queue stable.")).toBeInTheDocument();
    expect(screen.getByText("Accessed: 2x")).toBeInTheDocument();

    await user.type(screen.getByPlaceholderText("Search memory entries..."), "queue");
    await waitFor(() =>
      expect(searchMemory).toHaveBeenLastCalledWith("project-1", "queue", undefined, undefined),
    );

    await user.click(screen.getByRole("tab", { name: "Procedural" }));
    await waitFor(() =>
      expect(searchMemory).toHaveBeenLastCalledWith("project-1", "queue", undefined, "procedural"),
    );

    const buttons = container.querySelectorAll("button");
    await user.click(buttons[buttons.length - 1]);
    expect(deleteMemory).toHaveBeenCalledWith("project-1", "memory-1");
  });
});
