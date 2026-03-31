export const MOBILE_BREAKPOINT = 768;
export const DESKTOP_BREAKPOINT = 1280;

export const BREAKPOINT_QUERIES = {
  mobile: `(max-width: ${MOBILE_BREAKPOINT - 1}px)`,
  tablet: `(min-width: ${MOBILE_BREAKPOINT}px) and (max-width: ${
    DESKTOP_BREAKPOINT - 1
  }px)`,
  desktop: `(min-width: ${DESKTOP_BREAKPOINT}px)`,
} as const;

export type Breakpoint = keyof typeof BREAKPOINT_QUERIES;

export function getBreakpoint(width: number): Breakpoint {
  if (width >= DESKTOP_BREAKPOINT) {
    return "desktop";
  }

  if (width >= MOBILE_BREAKPOINT) {
    return "tablet";
  }

  return "mobile";
}
