type RouteParamValue = string | number | boolean | null | undefined;

function appendQueryParams(
  pathname: string,
  params: Record<string, RouteParamValue>,
) {
  const searchParams = new URLSearchParams();

  for (const [key, value] of Object.entries(params)) {
    if (value === undefined || value === null || value === "") {
      continue;
    }
    searchParams.set(key, typeof value === "boolean" ? (value ? "1" : "0") : String(value));
  }

  const query = searchParams.toString();
  return query ? `${pathname}?${query}` : pathname;
}

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

export function buildProjectScopedHref(
  pathname: string,
  options: {
    projectId?: string | null;
    projectParam?: "project" | "id";
    params?: Record<string, RouteParamValue>;
  },
) {
  return appendQueryParams(pathname, {
    [options.projectParam ?? "project"]: options.projectId ?? undefined,
    ...(options.params ?? {}),
  });
}

export function buildProjectTaskWorkspaceHref(options: {
  projectId?: string | null;
  sprintId?: string | null;
}) {
  return buildProjectScopedHref("/project", {
    projectId: options.projectId,
    projectParam: "id",
    params: {
      sprint: options.sprintId ?? undefined,
    },
  });
}
