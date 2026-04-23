import { render, screen, waitFor } from "@testing-library/react";
import { RoadmapView } from "./roadmap-view";
import { useMilestoneStore } from "@/lib/stores/milestone-store";

jest.mock("next-intl", () => ({
  useTranslations: (ns: string) => (key: string, values?: Record<string, string | number>) => {
    const map: Record<string, string> = {
      "milestones.roadmap.noTargetDate": "No target date",
      "milestones.roadmap.complete": "{rate}% complete",
      "milestones.roadmap.sprints": "Sprints",
      "milestones.roadmap.tasks": "Tasks",
      "milestones.status.planned": "Planned",
      "milestones.status.in_progress": "In Progress",
      "milestones.status.completed": "Completed",
      "milestones.status.missed": "Missed",
    };
    const result = map[`${ns}.${key}`] ?? `${ns}.${key}`;
    if (values) {
      return result.replace(/{(\w+)}/g, (_m, k) => String(values[k] ?? ""));
    }
    return result;
  },
}));

const fetchMilestonesMock = jest.fn();

describe("RoadmapView", () => {
  beforeEach(() => {
    fetchMilestonesMock.mockReset();
    fetchMilestonesMock.mockResolvedValue(undefined);

    useMilestoneStore.setState({
      milestonesByProject: {
        "project-1": [
          {
            id: "milestone-1",
            projectId: "project-1",
            name: "Release 2.0",
            targetDate: "2026-04-30",
            status: "in_progress",
            description: "",
            createdAt: "",
            updatedAt: "",
            metrics: {
              totalTasks: 4,
              completedTasks: 3,
              totalSprints: 1,
              completionRate: 75,
            },
          },
        ],
      },
      fetchMilestones: fetchMilestonesMock,
    });
  });

  it("fetches milestones and renders matching sprints and tasks", async () => {
    render(
      <RoadmapView
        projectId="project-1"
        tasks={[
          { id: "task-1", title: "Ship release checklist", milestoneId: "milestone-1" } as never,
          { id: "task-2", title: "Unlinked task", milestoneId: "milestone-2" } as never,
        ]}
        sprints={[
          { id: "sprint-1", name: "Sprint 1", milestoneId: "milestone-1" } as never,
          { id: "sprint-2", name: "Sprint 2", milestoneId: "milestone-2" } as never,
        ]}
      />,
    );

    await waitFor(() => {
      expect(fetchMilestonesMock).toHaveBeenCalledWith("project-1");
    });

    expect(screen.getByText("Release 2.0")).toBeInTheDocument();
    expect(screen.getByText(/75% complete/)).toBeInTheDocument();
    expect(screen.getByText("Sprint 1")).toBeInTheDocument();
    expect(screen.getByText("Ship release checklist")).toBeInTheDocument();
    expect(screen.queryByText("Sprint 2")).not.toBeInTheDocument();
    expect(screen.queryByText("Unlinked task")).not.toBeInTheDocument();
  });
});
