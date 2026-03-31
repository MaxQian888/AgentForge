"use client";

import * as React from "react";
import { BREAKPOINT_QUERIES, getBreakpoint, type Breakpoint } from "@/lib/responsive";

export interface BreakpointState {
  breakpoint: Breakpoint;
  isMobile: boolean;
  isTablet: boolean;
  isDesktop: boolean;
}

function buildBreakpointState(width: number): BreakpointState {
  const breakpoint = getBreakpoint(width);

  return {
    breakpoint,
    isMobile: breakpoint === "mobile",
    isTablet: breakpoint === "tablet",
    isDesktop: breakpoint === "desktop",
  };
}

function readViewportWidth() {
  if (typeof window === "undefined") {
    return 0;
  }

  return window.innerWidth;
}

function subscribe(queryList: MediaQueryList, listener: () => void) {
  if (typeof queryList.addEventListener === "function") {
    queryList.addEventListener("change", listener);
    return () => queryList.removeEventListener("change", listener);
  }

  queryList.addListener(listener);
  return () => queryList.removeListener(listener);
}

export function useBreakpoint(): BreakpointState {
  const [state, setState] = React.useState<BreakpointState>(() =>
    buildBreakpointState(readViewportWidth()),
  );

  React.useEffect(() => {
    if (typeof window === "undefined" || typeof window.matchMedia !== "function") {
      return undefined;
    }

    const mediaQueries = Object.values(BREAKPOINT_QUERIES).map((query) =>
      window.matchMedia(query),
    );
    const update = () => {
      setState(buildBreakpointState(readViewportWidth()));
    };
    const unsubscribe = mediaQueries.map((query) => subscribe(query, update));

    update();

    return () => {
      for (const cleanup of unsubscribe) {
        cleanup();
      }
    };
  }, []);

  return state;
}
