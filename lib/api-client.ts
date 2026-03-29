import { DEFAULT_LOCALE, getPreferredLocale } from "@/lib/stores/locale-store";

function getLocale(): string {
  try {
    return getPreferredLocale();
  } catch {
    return DEFAULT_LOCALE;
  }
}

type RequestOptions = Omit<RequestInit, "method" | "body">;

type ApiResponse<T> = {
  data: T;
  status: number;
};

async function request<T>(
  baseUrl: string,
  path: string,
  init: RequestInit
): Promise<ApiResponse<T>> {
  const url = `${baseUrl.replace(/\/$/, "")}${path}`;
  const res = await fetch(url, {
    ...init,
    headers: {
      "Content-Type": "application/json",
      "Accept-Language": getLocale(),
      ...init.headers,
    },
  });

  const data = await res.json().catch(() => null);

  if (!res.ok) {
    const message =
      (data as { message?: string })?.message ?? `HTTP ${res.status}`;
    throw new ApiError(message, res.status, data);
  }

  return { data: data as T, status: res.status };
}

export class ApiError extends Error {
  constructor(
    message: string,
    public readonly status: number,
    public readonly body: unknown = null
  ) {
    super(message);
    this.name = "ApiError";
  }
}

/**
 * Creates a typed API client bound to a given base URL.
 *
 * Usage:
 * ```ts
 * const backendUrl = useBackendUrl();
 * const api = createApiClient(backendUrl);
 * const { data } = await api.post<AuthResponse>("/api/v1/auth/login", { email, password });
 * ```
 */
export function createApiClient(baseUrl: string) {
  return {
    get<T>(path: string, opts?: RequestOptions & { token?: string }) {
      const { token, ...rest } = opts ?? {};
      return request<T>(baseUrl, path, {
        ...rest,
        method: "GET",
        headers: token
          ? { Authorization: `Bearer ${token}` }
          : undefined,
      });
    },

    post<T>(
      path: string,
      body: unknown,
      opts?: RequestOptions & { token?: string }
    ) {
      const { token, ...rest } = opts ?? {};
      return request<T>(baseUrl, path, {
        ...rest,
        method: "POST",
        body: JSON.stringify(body),
        headers: token
          ? { Authorization: `Bearer ${token}` }
          : undefined,
      });
    },

    put<T>(
      path: string,
      body: unknown,
      opts?: RequestOptions & { token?: string }
    ) {
      const { token, ...rest } = opts ?? {};
      return request<T>(baseUrl, path, {
        ...rest,
        method: "PUT",
        body: JSON.stringify(body),
        headers: token
          ? { Authorization: `Bearer ${token}` }
          : undefined,
      });
    },

    patch<T>(
      path: string,
      body: unknown,
      opts?: RequestOptions & { token?: string }
    ) {
      const { token, ...rest } = opts ?? {};
      return request<T>(baseUrl, path, {
        ...rest,
        method: "PATCH",
        body: JSON.stringify(body),
        headers: token
          ? { Authorization: `Bearer ${token}` }
          : undefined,
      });
    },

    delete<T>(path: string, opts?: RequestOptions & { token?: string }) {
      const { token, ...rest } = opts ?? {};
      return request<T>(baseUrl, path, {
        ...rest,
        method: "DELETE",
        headers: token
          ? { Authorization: `Bearer ${token}` }
          : undefined,
      });
    },

    /** Create a WebSocket URL from the base URL (http → ws, https → wss). */
    wsUrl(path: string, token?: string): string {
      const ws = baseUrl.replace(/^http/, "ws").replace(/\/$/, "");
      return token ? `${ws}${path}?token=${token}` : `${ws}${path}`;
    },
  };
}
