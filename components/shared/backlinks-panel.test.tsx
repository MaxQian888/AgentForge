import { render, screen } from "@testing-library/react";
import { BacklinksPanel } from "./backlinks-panel";

describe("BacklinksPanel", () => {
  it("renders backlink entries", () => {
    render(
      <BacklinksPanel
        items={[
          {
            linkId: "link-1",
            entityId: "page-2",
            entityType: "wiki_page",
            title: "Architecture Notes",
          },
        ]}
      />,
    );

    expect(screen.getByText("Backlinks")).toBeInTheDocument();
    expect(screen.getByText("Architecture Notes")).toBeInTheDocument();
  });
});
