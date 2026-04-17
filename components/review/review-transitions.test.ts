import {
  canTransition,
  invalidTransitionMessageKey,
} from "./review-transitions";

describe("review transitions", () => {
  describe("canTransition", () => {
    it.each(["approve", "reject", "block", "request_changes"] as const)(
      "allows %s from pending_human",
      (transition) => {
        expect(canTransition("pending_human", transition)).toBe(true);
      },
    );

    it.each([
      "pending",
      "in_progress",
      "completed",
      "failed",
      "unknown",
    ])("blocks transitions from %s", (status) => {
      expect(canTransition(status, "approve")).toBe(false);
      expect(canTransition(status, "reject")).toBe(false);
      expect(canTransition(status, "block")).toBe(false);
      expect(canTransition(status, "request_changes")).toBe(false);
    });
  });

  describe("invalidTransitionMessageKey", () => {
    it("returns distinct keys per transition", () => {
      expect(invalidTransitionMessageKey("approve")).toBe(
        "transitionInvalidApprove",
      );
      expect(invalidTransitionMessageKey("reject")).toBe(
        "transitionInvalidReject",
      );
      expect(invalidTransitionMessageKey("block")).toBe(
        "transitionInvalidBlock",
      );
      expect(invalidTransitionMessageKey("request_changes")).toBe(
        "transitionInvalidRequestChanges",
      );
    });
  });
});
