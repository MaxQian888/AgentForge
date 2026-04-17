import { act, render, renderHook, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useLiveArtifactSlashMenu } from "./use-live-artifact-slash-menu";
import type { LiveArtifactInsertSpec } from "./insertion-dialogs";

jest.mock("@/lib/stores/agent-store", () => {
  const actual = jest.requireActual("@/lib/stores/agent-store");
  const state = { agents: [], fetchAgents: jest.fn(async () => {}) };
  const useAgentStore = (
    selector: (s: typeof state) => unknown = (s) => s,
  ) => selector(state);
  return { ...actual, useAgentStore };
});

jest.mock("@/lib/stores/review-store", () => {
  const actual = jest.requireActual("@/lib/stores/review-store");
  const state = { allReviews: [], fetchAllReviews: jest.fn(async () => {}) };
  const useReviewStore = (
    selector: (s: typeof state) => unknown = (s) => s,
  ) => selector(state);
  return { ...actual, useReviewStore };
});

jest.mock("@/lib/stores/member-store", () => {
  const state = { membersByProject: {}, fetchMembers: jest.fn(async () => {}) };
  const useMemberStore = (
    selector: (s: typeof state) => unknown = (s) => s,
  ) => selector(state);
  return { useMemberStore };
});

jest.mock("@/lib/stores/saved-view-store", () => {
  const actual = jest.requireActual("@/lib/stores/saved-view-store");
  const state = { viewsByProject: {}, fetchViews: jest.fn(async () => {}) };
  const useSavedViewStore = (
    selector: (s: typeof state) => unknown = (s) => s,
  ) => selector(state);
  return { ...actual, useSavedViewStore };
});

describe("useLiveArtifactSlashMenu", () => {
  it("returns four slash-menu items and a menuDialogs element for wiki pages", () => {
    const onInsert = jest.fn();
    const { result } = renderHook(() =>
      useLiveArtifactSlashMenu({ assetKind: "wiki_page", onInsert }),
    );

    expect(result.current.slashMenuItems.map((item) => item.key)).toEqual([
      "agent_run",
      "cost_summary",
      "review",
      "task_group",
    ]);
    expect(result.current.menuDialogs).not.toBeNull();

    for (const item of result.current.slashMenuItems) {
      expect(item.title).toBeTruthy();
      expect(item.subtext).toBeTruthy();
      expect(item.group).toBe("Live artifacts");
      expect(typeof item.onItemClick).toBe("function");
    }
  });

  it("returns an empty item list and null menuDialogs when assetKind !== wiki_page", () => {
    const onInsert = jest.fn();

    for (const kind of ["template", "ingested_file"] as const) {
      const { result } = renderHook(() =>
        useLiveArtifactSlashMenu({ assetKind: kind, onInsert }),
      );
      expect(result.current.slashMenuItems).toEqual([]);
      expect(result.current.menuDialogs).toBeNull();
    }
  });

  it("opens the matching dialog when a slash-menu entry is activated", () => {
    const onInsert = jest.fn();
    const { result } = renderHook(() =>
      useLiveArtifactSlashMenu({ assetKind: "wiki_page", onInsert }),
    );

    const reviewItem = result.current.slashMenuItems.find((i) => i.key === "review");
    expect(reviewItem).toBeDefined();

    act(() => {
      reviewItem?.onItemClick();
    });

    expect(result.current.openDialog).toBe("review");
  });

  it("propagates confirm events from mounted dialogs", async () => {
    const user = userEvent.setup();
    const onInsert = jest.fn();

    function Host() {
      const { slashMenuItems, menuDialogs } = useLiveArtifactSlashMenu({
        assetKind: "wiki_page",
        projectId: "project-1",
        onInsert: (spec: LiveArtifactInsertSpec) => onInsert(spec),
      });

      const reviewItem = slashMenuItems.find((item) => item.key === "review");
      return (
        <div>
          <button type="button" onClick={() => reviewItem?.onItemClick()}>
            Open review dialog
          </button>
          {menuDialogs}
        </div>
      );
    }

    // Seed a review into the mocked store
    const reviewStoreModule = jest.requireMock("@/lib/stores/review-store") as {
      useReviewStore: <T>(selector: (state: { allReviews: unknown[] }) => T) => T;
    };
    // Replace the internal state reference for this test
    const underlyingState = reviewStoreModule.useReviewStore(
      (s: { allReviews: unknown[] }) => s,
    ) as { allReviews: unknown[] };
    underlyingState.allReviews = [
      {
        id: "review-hook-000001",
        taskId: "task-1",
        prUrl: "",
        prNumber: 1,
        layer: 1,
        status: "pending",
        riskLevel: "low",
        findings: [],
        summary: "",
        recommendation: "",
        costUsd: 0,
        createdAt: "2026-04-17T00:00:00Z",
        updatedAt: "2026-04-17T00:00:00Z",
      },
    ];

    render(<Host />);

    await user.click(screen.getByRole("button", { name: "Open review dialog" }));
    await user.click(screen.getByRole("option"));
    await user.click(screen.getByRole("button", { name: "Insert" }));

    expect(onInsert).toHaveBeenCalledWith({
      live_kind: "review",
      target_ref: { kind: "review", id: "review-hook-000001" },
      view_opts: { show_findings_preview: true },
    });

    // Reset for hygiene
    underlyingState.allReviews = [];
  });
});
