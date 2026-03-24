/** Filter applied to agent output text before emission. */
export interface OutputFilter {
  name: string;
  apply(text: string): string;
}

/**
 * Redacts common credential patterns: API keys, AWS keys, Bearer tokens,
 * passwords in URLs, and generic secret-like strings.
 */
export const NO_CREDENTIALS_FILTER: OutputFilter = {
  name: "no_credentials",
  apply(text: string): string {
    return text
      // OpenAI / Anthropic style API keys
      .replace(/sk-[a-zA-Z0-9]{20,}/g, "[REDACTED_API_KEY]")
      // AWS access keys
      .replace(/AKIA[A-Z0-9]{16}/g, "[REDACTED_AWS_KEY]")
      // Bearer tokens
      .replace(/Bearer\s+[a-zA-Z0-9._\-/+=]{16,}/g, "Bearer [REDACTED]")
      // Passwords in URLs (scheme://user:pass@host)
      .replace(/:\/\/([^:]+):([^@]+)@/g, "://$1:[REDACTED]@")
      // Generic secret assignments (SECRET=..., TOKEN=..., PASSWORD=...)
      .replace(/((?:SECRET|TOKEN|PASSWORD|API_KEY|PRIVATE_KEY)\s*[=:]\s*)[^\s"',;]+/gi, "$1[REDACTED]");
  },
};

/**
 * Redacts basic PII patterns: email addresses and phone numbers.
 */
export const NO_PII_FILTER: OutputFilter = {
  name: "no_pii",
  apply(text: string): string {
    return text
      // Email addresses
      .replace(/[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}/g, "[REDACTED_EMAIL]")
      // US phone numbers (various formats)
      .replace(/\b\d{3}[-.]?\d{3}[-.]?\d{4}\b/g, "[REDACTED_PHONE]");
  },
};

const FILTER_REGISTRY: Record<string, OutputFilter> = {
  no_credentials: NO_CREDENTIALS_FILTER,
  no_pii: NO_PII_FILTER,
};

/** Build a filter pipeline from an array of filter names. Unknown names are skipped. */
export function buildFilterPipeline(filterNames: string[]): OutputFilter[] {
  return filterNames.map((n) => FILTER_REGISTRY[n]).filter(Boolean);
}

/** Apply all filters in sequence to the input text. */
export function applyFilters(text: string, filters: OutputFilter[]): string {
  return filters.reduce((t, f) => f.apply(t), text);
}
