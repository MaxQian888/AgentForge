export type MemberStatus = "active" | "inactive" | "suspended";

export function normalizeMemberStatus(
  status: string | null | undefined,
  isActive: boolean,
): MemberStatus {
  switch (status) {
    case "active":
    case "inactive":
    case "suspended":
      return status;
    default:
      return isActive ? "active" : "inactive";
  }
}

export function isMemberAvailable(
  status: string | null | undefined,
  isActive = false,
): boolean {
  return normalizeMemberStatus(status, isActive) === "active";
}

export function getMemberStatusLabel(status: MemberStatus): string {
  switch (status) {
    case "active":
      return "Active";
    case "inactive":
      return "Inactive";
    case "suspended":
      return "Suspended";
  }
}
