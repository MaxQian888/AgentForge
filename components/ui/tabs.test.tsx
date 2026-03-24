import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Tabs, TabsContent, TabsList, TabsTrigger, tabsListVariants } from "./tabs";

describe("Tabs", () => {
  it("switches tab panels and supports the line variant", async () => {
    const user = userEvent.setup();

    render(
      <Tabs defaultValue="overview" orientation="vertical">
        <TabsList variant="line">
          <TabsTrigger value="overview">Overview</TabsTrigger>
          <TabsTrigger value="agents">Agents</TabsTrigger>
        </TabsList>
        <TabsContent value="overview">Overview content</TabsContent>
        <TabsContent value="agents">Agents content</TabsContent>
      </Tabs>
    );

    const overviewTab = screen.getByRole("tab", { name: "Overview" });
    const agentsTab = screen.getByRole("tab", { name: "Agents" });

    expect(screen.getByText("Overview content")).toHaveAttribute("data-slot", "tabs-content");
    await user.click(agentsTab);
    expect(agentsTab).toHaveAttribute("data-state", "active");
    expect(screen.getByText("Agents content")).toBeInTheDocument();
    expect(overviewTab).toHaveAttribute("data-slot", "tabs-trigger");

    const listClassName = tabsListVariants({ variant: "line" });
    expect(listClassName).toContain("bg-transparent");
  });
});
