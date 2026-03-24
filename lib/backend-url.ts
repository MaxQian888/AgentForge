import { platformRuntime } from "@/lib/platform-runtime";

export function getDefaultBackendUrl(): string {
  return platformRuntime.defaultBackendUrl;
}

export async function resolveBackendUrl(): Promise<string> {
  return platformRuntime.resolveBackendUrl();
}
