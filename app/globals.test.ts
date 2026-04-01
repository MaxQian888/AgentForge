import { readFileSync } from "node:fs";
import path from "node:path";

describe("globals.css responsive tokens", () => {
  it("defines spacing and typography tokens for the responsive layout system", () => {
    const css = readFileSync(
      path.join(process.cwd(), "app", "globals.css"),
      "utf8",
    );

    expect(css).toContain("--space-page-inline");
    expect(css).toContain("--space-section-gap");
    expect(css).toContain("--space-card-padding");
    expect(css).toContain("--font-size-page-title");
    expect(css).toContain("--font-size-metric-value");
    expect(css).toContain(".text-fluid-title");
    expect(css).toContain(".text-fluid-body");
    expect(css).toContain(".text-fluid-metric");
  });
});
