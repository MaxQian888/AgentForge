import { getStatusColor, STATUS_COLORS } from "./status-colors";

const FALLBACK_STATUS_COLOR = {
  dot: "bg-zinc-300 dark:bg-zinc-600",
  bg: "bg-zinc-500/10",
  text: "text-zinc-600 dark:text-zinc-400",
};

describe("status-colors", () => {
  it("returns the configured palette for known statuses", () => {
    expect(getStatusColor("active")).toEqual(STATUS_COLORS.active);
    expect(getStatusColor("completed")).toEqual(STATUS_COLORS.completed);
  });

  it("normalizes status casing before lookup", () => {
    expect(getStatusColor("SUCCESS")).toEqual(STATUS_COLORS.success);
    expect(getStatusColor("ReViEwInG")).toEqual(STATUS_COLORS.reviewing);
  });

  it("falls back to the neutral palette for unknown statuses", () => {
    expect(getStatusColor("mystery")).toEqual(FALLBACK_STATUS_COLOR);
  });
});
