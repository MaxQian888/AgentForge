import { TaskCalendarView } from "./task-calendar-view";
import { TaskCalendarView as TaskCalendarViewImpl } from "./task-workspace-main";

describe("TaskCalendarView export", () => {
  it("re-exports the calendar view implementation from task-workspace-main", () => {
    expect(TaskCalendarView).toBe(TaskCalendarViewImpl);
  });
});
