export type ReviewTransition =
  | "approve"
  | "reject"
  | "block"
  | "request_changes";

/**
 * Status transition rules for reviews.
 *
 * Today the backend only allows human decisions (approve, reject,
 * request changes) while a review is in `pending_human`. "Block" is
 * surfaced as a distinct UI intent but still maps onto the `reject`
 * endpoint - we validate client-side that the source status is one
 * the backend will honour.
 */
const humanDecisionStatuses = new Set<string>(["pending_human"]);

export function canTransition(
  status: string,
  transition: ReviewTransition,
): boolean {
  switch (transition) {
    case "approve":
    case "reject":
    case "block":
    case "request_changes":
      return humanDecisionStatuses.has(status);
    default:
      return false;
  }
}

export function invalidTransitionMessageKey(
  transition: ReviewTransition,
): string {
  switch (transition) {
    case "approve":
      return "transitionInvalidApprove";
    case "reject":
      return "transitionInvalidReject";
    case "block":
      return "transitionInvalidBlock";
    case "request_changes":
      return "transitionInvalidRequestChanges";
    default:
      return "transitionInvalidApprove";
  }
}
