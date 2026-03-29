import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { AddWidgetDialog } from "./add-widget-dialog";

const saveWidget = jest.fn().mockResolvedValue(undefined);

jest.mock("next-intl", () => ({
  useTranslations: () => (key: string) => {
    const translations: Record<string, string> = {
      "widget.addTitle": "Add Widget",
      "widget.addDescription": "Select a widget type to add to this dashboard.",
      "widget.cancel": "Cancel",
      "widget.add": "Add",
      "widget.throughput_chart.title": "Throughput Chart",
      "widget.throughput_chart.description": "Track completed tasks over time.",
      "widget.burndown.title": "Burndown",
      "widget.burndown.description": "Track remaining sprint work.",
    };

    return translations[key] ?? key;
  },
}));

jest.mock("@/lib/stores/dashboard-store", () => ({
  useDashboardStore: (selector: (state: { saveWidget: typeof saveWidget }) => unknown) =>
    selector({ saveWidget }),
}));

describe("AddWidgetDialog", () => {
  beforeEach(() => {
    saveWidget.mockClear();
  });

  it("shows widget metadata and saves a supported widget with default config", async () => {
    const user = userEvent.setup();

    render(
      <AddWidgetDialog
        open
        onOpenChange={jest.fn()}
        projectId="project-1"
        dashboardId="dashboard-1"
      />
    );

    expect(screen.getAllByText("Track completed tasks over time.").length).toBeGreaterThan(0);
    await user.click(screen.getByLabelText("Burndown"));
    expect(screen.getAllByText("Track remaining sprint work.").length).toBeGreaterThan(0);

    await user.click(screen.getByRole("button", { name: "Add" }));

    expect(saveWidget).toHaveBeenCalledWith("project-1", "dashboard-1", {
      widgetType: "burndown",
      config: { range: "current_sprint", groupBy: "day" },
      position: { x: 0, y: 0, w: 1, h: 1 },
    });
  });
});
