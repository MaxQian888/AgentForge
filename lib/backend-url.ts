const DEFAULT_BACKEND_URL =
  process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:7777";

function isTauriRuntime(): boolean {
  return typeof window !== "undefined" && "__TAURI_INTERNALS__" in window;
}

export function getDefaultBackendUrl(): string {
  return DEFAULT_BACKEND_URL;
}

export async function resolveBackendUrl(): Promise<string> {
  if (!isTauriRuntime()) {
    return DEFAULT_BACKEND_URL;
  }

  try {
    const { invoke } = await import("@tauri-apps/api/core");
    return await invoke<string>("get_backend_url");
  } catch {
    return DEFAULT_BACKEND_URL;
  }
}
