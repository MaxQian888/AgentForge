export function buildDocsHref(
  pageId: string,
  options?: {
    readonly?: boolean;
    version?: string | null;
  }
) {
  const params = new URLSearchParams({ pageId });
  if (options?.version) {
    params.set("version", options.version);
  }
  if (options?.readonly) {
    params.set("readonly", "1");
  }
  return `/docs?${params.toString()}`;
}

export function buildFormHref(slug: string) {
  const params = new URLSearchParams({ slug });
  return `/forms?${params.toString()}`;
}