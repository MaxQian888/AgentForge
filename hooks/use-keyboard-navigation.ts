"use client";

import { useEffect, useRef } from "react";
import { useRouter } from "next/navigation";

const GO_TO_SHORTCUTS: Record<string, string> = {
  d: "/",
  p: "/projects",
  a: "/agents",
  t: "/teams",
  c: "/cost",
  r: "/roles",
  s: "/settings",
  m: "/memory",
  i: "/im",
};

export function useKeyboardNavigation() {
  const router = useRouter();
  const waitingForSecondKey = useRef(false);
  const timeoutRef = useRef<ReturnType<typeof setTimeout>>(undefined);

  useEffect(() => {
    function handleKeyDown(e: KeyboardEvent) {
      if (
        e.target instanceof HTMLInputElement ||
        e.target instanceof HTMLTextAreaElement ||
        e.target instanceof HTMLSelectElement ||
        (e.target instanceof HTMLElement && e.target.isContentEditable)
      ) {
        return;
      }

      if (e.metaKey || e.ctrlKey || e.altKey) return;

      if (waitingForSecondKey.current) {
        waitingForSecondKey.current = false;
        if (timeoutRef.current) clearTimeout(timeoutRef.current);

        const route = GO_TO_SHORTCUTS[e.key];
        if (route) {
          e.preventDefault();
          router.push(route);
        }
        return;
      }

      if (e.key === "g") {
        waitingForSecondKey.current = true;
        timeoutRef.current = setTimeout(() => {
          waitingForSecondKey.current = false;
        }, 1000);
      }
    }

    document.addEventListener("keydown", handleKeyDown);
    return () => {
      document.removeEventListener("keydown", handleKeyDown);
      if (timeoutRef.current) clearTimeout(timeoutRef.current);
    };
  }, [router]);
}
