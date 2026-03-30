import { formatRelativeTime } from "./format-relative-time";

const NOW = new Date("2026-03-30T12:00:00.000Z").getTime();

describe("formatRelativeTime", () => {
  beforeEach(() => {
    jest.spyOn(Date, "now").mockReturnValue(NOW);
  });

  afterEach(() => {
    jest.restoreAllMocks();
  });

  it("returns N/A when no value is provided", () => {
    expect(formatRelativeTime(undefined)).toBe("N/A");
  });

  it("returns the original value for invalid dates", () => {
    expect(formatRelativeTime("not-a-date")).toBe("not-a-date");
  });

  it("formats future and past dates with relative labels", () => {
    expect(formatRelativeTime(new Date(NOW + 2 * 60 * 60 * 1000))).toBe(
      "in 2 hours",
    );
    expect(formatRelativeTime(new Date(NOW - 2 * 24 * 60 * 60 * 1000))).toBe(
      "2 days ago",
    );
  });

  it("uses a just-now label for sub-second differences", () => {
    expect(formatRelativeTime(new Date(NOW - 500))).toBe("just now");
  });
});
