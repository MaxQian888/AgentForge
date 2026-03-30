import { render, screen, waitFor } from "@testing-library/react";
import { AutomationLogViewer } from "./automation-log-viewer";
import { useAutomationStore } from "@/lib/stores/automation-store";

const fetchLogsMock = jest.fn();

describe("AutomationLogViewer", () => {
  beforeEach(() => {
    fetchLogsMock.mockReset();
    fetchLogsMock.mockResolvedValue(undefined);

    useAutomationStore.setState({
      logsByProject: {
        "project-1": [
          {
            id: "log-1",
            ruleId: "rule-1",
            eventType: "task.status_changed",
            status: "success",
            triggeredAt: "2026-03-30T10:00:00.000Z",
            detail: {},
          },
        ],
      },
      fetchLogs: fetchLogsMock,
    });
  });

  it("fetches and renders automation log entries", async () => {
    render(<AutomationLogViewer projectId="project-1" />);

    await waitFor(() => {
      expect(fetchLogsMock).toHaveBeenCalledWith("project-1");
    });

    expect(screen.getByText(/task\.status_changed/)).toBeInTheDocument();
    expect(screen.getByText(/2026-03-30T10:00:00.000Z/)).toBeInTheDocument();
  });
});
