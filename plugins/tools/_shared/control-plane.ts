export type JsonRecord = Record<string, unknown>;

export function getRequiredEnv(name: string): string {
  const value = process.env[name];
  if (!value?.trim()) {
    throw new Error(`Missing required environment variable ${name}`);
  }
  return value;
}

export async function requestControlPlaneJson<T>(path: string, init: RequestInit): Promise<T> {
  const baseUrl = getRequiredEnv("AGENTFORGE_API_BASE_URL").replace(/\/$/, "");
  const token = getRequiredEnv("AGENTFORGE_API_TOKEN");
  const response = await fetch(`${baseUrl}${path}`, {
    ...init,
    headers: {
      Accept: "application/json",
      Authorization: `Bearer ${token}`,
      ...(init.headers ?? {}),
    },
  });

  if (!response.ok) {
    const message = await response.text();
    throw new Error(message || `HTTP ${response.status}`);
  }

  return (await response.json()) as T;
}
