import { readFileSync } from "node:fs";
import path from "node:path";

describe("/i18n-fill command", () => {
  it("points to the nested i18n audit script", () => {
    const commandDoc = readFileSync(
      path.join(process.cwd(), ".claude/commands/i18n-fill.md"),
      "utf8",
    );

    expect(commandDoc).toContain("node scripts/i18n/i18n-audit.js --json");
    expect(commandDoc).not.toContain("node scripts/i18n-audit.js --json");
  });
});
