import { formatDuration } from "./format-duration";

describe("formatDuration", () => {
  it("returns a placeholder for nullish or negative durations", () => {
    expect(formatDuration(null)).toBe("-");
    expect(formatDuration(undefined)).toBe("-");
    expect(formatDuration(-1)).toBe("-");
  });

  it("formats millisecond and second durations", () => {
    expect(formatDuration(999)).toBe("999ms");
    expect(formatDuration(1500)).toBe("1.5s");
  });

  it("formats minute durations with and without remaining seconds", () => {
    expect(formatDuration(61_000)).toBe("1m 1s");
    expect(formatDuration(120_000)).toBe("2m");
  });

  it("formats hour durations with and without remaining minutes", () => {
    expect(formatDuration(3_660_000)).toBe("1h 1m");
    expect(formatDuration(7_200_000)).toBe("2h");
  });
});
