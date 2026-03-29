export function normalizeMessageNamespace(
  value: Record<string, unknown>,
): Record<string, unknown> {
  const output: Record<string, unknown> = {};

  for (const [key, entry] of Object.entries(value)) {
    assignNestedValue(output, key, normalizeNestedValue(entry));
  }

  return output;
}

function normalizeNestedValue(value: unknown): unknown {
  if (!value || typeof value !== "object" || Array.isArray(value)) {
    return value;
  }

  return normalizeMessageNamespace(value as Record<string, unknown>);
}

function assignNestedValue(
  target: Record<string, unknown>,
  dottedKey: string,
  value: unknown,
) {
  if (!dottedKey.includes(".")) {
    target[dottedKey] = value;
    return;
  }

  const segments = dottedKey.split(".");
  let current: Record<string, unknown> = target;

  for (let index = 0; index < segments.length - 1; index += 1) {
    const segment = segments[index];
    const existing = current[segment];
    if (!existing || typeof existing !== "object" || Array.isArray(existing)) {
      current[segment] = {};
    }
    current = current[segment] as Record<string, unknown>;
  }

  current[segments[segments.length - 1]] = value;
}

export function normalizeMessageBundle(
  bundle: Record<string, Record<string, unknown>>,
): Record<string, Record<string, unknown>> {
  return Object.fromEntries(
    Object.entries(bundle).map(([namespace, messages]) => [
      namespace,
      normalizeMessageNamespace(messages),
    ]),
  );
}
