"use client";

import { useEffect, useState } from "react";
import { getDefaultBackendUrl, resolveBackendUrl } from "@/lib/backend-url";

/**
 * Returns the base URL for the Go backend server.
 *
 * - In Tauri desktop mode: calls `get_backend_url` Tauri command to get
 *   the dynamically assigned localhost URL from the Rust layer.
 * - In web/separated mode: reads NEXT_PUBLIC_API_URL env var
 *   (falls back to http://localhost:7777).
 */
export function useBackendUrl(): string {
  const [url, setUrl] = useState<string>(getDefaultBackendUrl());

  useEffect(() => {
    resolveBackendUrl()
      .then(setUrl)
      .catch((err) => {
        console.warn("Failed to resolve backend URL:", err);
      });
  }, []);

  return url;
}
