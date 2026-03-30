import { TaskTimelineView } from "./task-timeline-view";
import { TaskTimelineView as TaskTimelineViewImpl } from "./task-workspace-main";

describe("TaskTimelineView export", () => {
  it("re-exports the timeline view implementation from task-workspace-main", () => {
    expect(TaskTimelineView).toBe(TaskTimelineViewImpl);
  });
});
