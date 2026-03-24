import { normalizePlanningInput } from "./task-planning";

describe("normalizePlanningInput", () => {
  it("returns unscheduled when both dates are empty", () => {
    expect(normalizePlanningInput({ startDate: "", endDate: "" })).toEqual({
      kind: "unscheduled",
    });
  });

  it("treats one provided date as a single-day schedule", () => {
    expect(
      normalizePlanningInput({ startDate: "2026-03-30", endDate: "" })
    ).toEqual({
      kind: "scheduled",
      plannedStartAt: "2026-03-30T09:00:00.000Z",
      plannedEndAt: "2026-03-30T18:00:00.000Z",
    });
  });

  it("keeps explicit date ranges intact", () => {
    expect(
      normalizePlanningInput({
        startDate: "2026-03-30",
        endDate: "2026-04-01",
      })
    ).toEqual({
      kind: "scheduled",
      plannedStartAt: "2026-03-30T09:00:00.000Z",
      plannedEndAt: "2026-04-01T18:00:00.000Z",
    });
  });

  it("rejects end-before-start", () => {
    expect(
      normalizePlanningInput({
        startDate: "2026-04-02",
        endDate: "2026-04-01",
      })
    ).toEqual({ kind: "invalid", reason: "end_before_start" });
  });
});
