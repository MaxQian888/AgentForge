import { describeCron } from "./cron-description";

describe("describeCron", () => {
  it("returns an empty string for empty expressions", () => {
    expect(describeCron("")).toBe("");
  });

  it("maps trimmed common cron patterns to friendly labels", () => {
    expect(describeCron("  0 0 * * 1  ")).toBe("Weekly on Monday");
    expect(describeCron("*/5 * * * *")).toBe("Every 5 minutes");
  });

  it("describes simple interval schedules", () => {
    expect(describeCron("*/20 * * * *")).toBe("Every 20 minutes");
    expect(describeCron("0 */8 * * *")).toBe("Every 8 hours");
  });

  it("describes simple daily schedules with zero-padded time", () => {
    expect(describeCron("5 3 * * *")).toBe("Daily at 03:05");
  });

  it("returns the trimmed expression for unsupported or malformed schedules", () => {
    expect(describeCron("0 0 * *")).toBe("0 0 * *");
    expect(describeCron("15 8 1 * 1")).toBe("15 8 1 * 1");
  });
});
