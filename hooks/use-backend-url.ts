"use client";

import { useEffect, useState } from "react";

const DEFAULT_URL =
  process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:7777";

function isTauri(): boolean {
  return (
    typeof window !== "undefined" && "__TAURI_INTERNALS__" in window
  );
}

/**
 * Returns the base URL for the Go backend server.
 *
 * - In Tauri desktop mode: calls `get_backend_url` Tauri command to get
 *   the dynamically assigned localhost URL from the Rust layer.
 * - In web/separated mode: reads NEXT_PUBLIC_API_URL env var
 *   (falls back to http://localhost:7777).
 */
export function useBackendUrl(): string {
  const [url, setUrl] = useState<string>(DEFAULT_URL);

  useEffect(() => {
    if (!isTauri()) return;

    // Dynamic import to avoid SSR issues and missing module errors in web mode
    import("@tauri-apps/api/core")
      .then(({ invoke }) => invoke<string>("get_backend_url"))
      .then(setUrl)
      .catch((err) => {
        console.warn("Failed to get backend URL from Tauri:", err);
      });
  }, []);

  return url;
}
