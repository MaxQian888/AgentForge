import {
  getMemberStatusLabel,
  isMemberAvailable,
  normalizeMemberStatus,
} from "./member-status";

describe("member-status", () => {
  it("preserves supported status values", () => {
    expect(normalizeMemberStatus("active", false)).toBe("active");
    expect(normalizeMemberStatus("inactive", true)).toBe("inactive");
    expect(normalizeMemberStatus("suspended", true)).toBe("suspended");
  });

  it("falls back to activity state for unknown statuses", () => {
    expect(normalizeMemberStatus("unknown", true)).toBe("active");
    expect(normalizeMemberStatus(null, false)).toBe("inactive");
  });

  it("treats only active members as available", () => {
    expect(isMemberAvailable("active")).toBe(true);
    expect(isMemberAvailable(undefined, true)).toBe(true);
    expect(isMemberAvailable("suspended", true)).toBe(false);
  });

  it("returns the display label for each normalized status", () => {
    expect(getMemberStatusLabel("active")).toBe("Active");
    expect(getMemberStatusLabel("inactive")).toBe("Inactive");
    expect(getMemberStatusLabel("suspended")).toBe("Suspended");
  });
});
