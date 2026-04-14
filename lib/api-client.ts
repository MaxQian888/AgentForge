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

// ---------------------------------------------------------------------------
// Token-refresh registration (avoids circular import with auth-store)
// ---------------------------------------------------------------------------

type TokenRefreshFn = () => Promise<string>;

let _onTokenRefresh: TokenRefreshFn | null = null;
let _refreshPromise: Promise<string> | null = null;

/**
 * Register the callback the API client will invoke when a 401 is received on
 * an authenticated request. The callback should refresh the session and return
 * the new access token (or throw).
 *
 * Called once by the auth store at module-init time.
 */
export function registerTokenRefresh(fn: TokenRefreshFn) {
  _onTokenRefresh = fn;
}

/**
 * Attempt to refresh the access token, coalescing concurrent callers into a
 * single in-flight request. Returns the new access token or throws.
 */
async function refreshAccessToken(): Promise<string> {
  if (!_onTokenRefresh) throw new Error("No token refresh handler registered");

  // Coalesce: reuse an existing in-flight refresh if present
  if (_refreshPromise) return _refreshPromise;

  _refreshPromise = _onTokenRefresh().finally(() => {
    _refreshPromise = null;
  });

  return _refreshPromise;
}

// ---------------------------------------------------------------------------

async function request<T>(
  baseUrl: string,
  path: string,
  init: RequestInit,
  _isRetry = false
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
    // 401 on an authenticated request → try refresh + retry (once)
    const hadToken = !!(init.headers as Record<string, string>)?.Authorization;
    if (res.status === 401 && hadToken && !_isRetry && _onTokenRefresh) {
      try {
        const newToken = await refreshAccessToken();
        const retryInit: RequestInit = {
          ...init,
          headers: {
            ...((init.headers as Record<string, string>) ?? {}),
            Authorization: `Bearer ${newToken}`,
          },
        };
        return request<T>(baseUrl, path, retryInit, true);
      } catch {
        // Refresh failed — fall through to throw the original 401 error
      }
    }

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
