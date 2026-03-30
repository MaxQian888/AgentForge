import { TaskListView } from "./task-list-view";
import { TaskListView as TaskListViewImpl } from "./task-workspace-main";

describe("TaskListView export", () => {
  it("re-exports the list view implementation from task-workspace-main", () => {
    expect(TaskListView).toBe(TaskListViewImpl);
  });
});
