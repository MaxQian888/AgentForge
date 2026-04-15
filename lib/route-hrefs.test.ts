import {
  buildDocsHref,
  buildFormHref,
  buildProjectTaskWorkspaceHref,
  buildProjectScopedHref,
} from "./route-hrefs";

describe("route href builders", () => {
  it("builds a docs href with the required page id", () => {
    expect(buildDocsHref("page-1")).toBe("/docs?pageId=page-1");
  });

  it("adds optional docs params when provided", () => {
    expect(
      buildDocsHref("page 1", {
        readonly: true,
        version: "v1.2.3",
      }),
    ).toBe("/docs?pageId=page+1&version=v1.2.3&readonly=1");
  });

  it("builds a form href from the slug", () => {
    expect(buildFormHref("release-checklist")).toBe(
      "/forms?slug=release-checklist",
    );
  });

  it("builds a project-scoped href with the default project query key", () => {
    expect(
      buildProjectScopedHref("/workflow", {
        projectId: "project-7",
        params: { tab: "templates" },
      }),
    ).toBe("/workflow?project=project-7&tab=templates");
  });

  it("supports routes that still use id as the project query key", () => {
    expect(
      buildProjectScopedHref("/project", {
        projectId: "project-9",
        projectParam: "id",
        params: { action: "create-task" },
      }),
    ).toBe("/project?id=project-9&action=create-task");
  });

  it("builds a project task workspace href with optional sprint scope", () => {
    expect(
      buildProjectTaskWorkspaceHref({
        projectId: "project-9",
        sprintId: "sprint-2",
      }),
    ).toBe("/project?id=project-9&sprint=sprint-2");
  });
});
