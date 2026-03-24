export interface ClassifiedError {
  code: string;
  message: string;
  retryable: boolean;
  metadata?: Record<string, unknown>;
}

export function classifyError(err: unknown): ClassifiedError {
  const message = err instanceof Error ? err.message : String(err);

  if (message.includes("Cancelled") || message.includes("aborted")) {
    return { code: "CANCELLED", message, retryable: false };
  }

  if (message.includes("rate_limit") || message.includes("429")) {
    return { code: "RATE_LIMIT", message, retryable: true };
  }

  if (message.includes("overloaded") || message.includes("529")) {
    return { code: "OVERLOADED", message, retryable: true };
  }

  if (message.includes("budget") || message.includes("exceeded")) {
    return { code: "BUDGET_EXCEEDED", message, retryable: false };
  }

  if (message.includes("timeout") || message.includes("ETIMEDOUT")) {
    return { code: "TIMEOUT", message, retryable: true };
  }

  if (message.includes("session") && message.includes("expired")) {
    return { code: "SESSION_EXPIRED", message, retryable: false };
  }

  if (message.includes("authentication") || message.includes("401")) {
    return { code: "AUTH_FAILED", message, retryable: false };
  }

  return { code: "INTERNAL", message, retryable: false };
}
