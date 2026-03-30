import { buildDocsHref, buildFormHref } from "./route-hrefs";

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
});
