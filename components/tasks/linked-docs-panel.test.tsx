import type { EntityLink } from "@/lib/stores/entity-link-store";

type EntityLinkStoreSlice = {
  linksByEntity: Record<string, EntityLink[]>;
  loading: boolean;
  error: string | null;
  fetchLinks: jest.Mock;
  createLink: jest.Mock;
  deleteLink: jest.Mock;
};

jest.mock("@/lib/stores/entity-link-store", () => ({
  useEntityLinkStore: Object.assign(
    (selector: (state: EntityLinkStoreSlice) => unknown) =>
      selector({
        linksByEntity: {
          "task:task-1": [
            {
              id: "link-1",
              projectId: "project-1",
              sourceType: "task",
              sourceId: "task-1",
              targetType: "wiki_page",
              targetId: "page-1",
              linkType: "requirement",
              createdBy: "user-1",
              createdAt: "2026-03-26T10:00:00.000Z",
            },
          ],
        },
        loading: false,
        error: null,
        fetchLinks: jest.fn(),
        createLink: jest.fn(),
        deleteLink: jest.fn(),
      } satisfies EntityLinkStoreSlice),
    {
      getState: () => ({
        linksByEntity: {
          "task:task-1": [
            {
              id: "link-1",
              projectId: "project-1",
              sourceType: "task",
              sourceId: "task-1",
              targetType: "wiki_page",
              targetId: "page-1",
              linkType: "requirement",
              createdBy: "user-1",
              createdAt: "2026-03-26T10:00:00.000Z",
            },
          ],
        },
      }),
    },
  ),
}));

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { LinkedDocsPanel } from "./linked-docs-panel";

describe("LinkedDocsPanel", () => {
  it("renders grouped related docs and allows add/remove actions", async () => {
    const user = userEvent.setup();
    const onAdd = jest.fn();
    const onRemove = jest.fn();

    render(
      <LinkedDocsPanel
        projectId="project-1"
        taskId="task-1"
        docs={[
          {
            id: "link-1",
            pageId: "page-1",
            title: "PRD",
            linkType: "requirement",
            updatedAt: "2026-03-26T10:00:00.000Z",
            preview: "First five blocks",
          },
        ]}
        onAddLink={onAdd}
        onRemoveLink={onRemove}
      />,
    );

    expect(screen.getByText("Related Docs")).toBeInTheDocument();
    expect(screen.getByText("PRD")).toBeInTheDocument();
    expect(screen.getByText("First five blocks")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Add Doc" }));
    expect(onAdd).toHaveBeenCalled();

    await user.click(screen.getByRole("button", { name: "Remove PRD" }));
    expect(onRemove).toHaveBeenCalledWith("link-1");
  });
});
