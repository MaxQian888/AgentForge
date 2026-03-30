import { useEffect } from "react";
import { useLayoutStore, type BreadcrumbItem } from "@/lib/stores/layout-store";

export function useBreadcrumbs(breadcrumbs: BreadcrumbItem[]) {
  const setBreadcrumbs = useLayoutStore((s) => s.setBreadcrumbs);

  useEffect(() => {
    setBreadcrumbs(breadcrumbs);
    return () => setBreadcrumbs([]);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [setBreadcrumbs, JSON.stringify(breadcrumbs)]);
}
