"use client";

import { useSearchParams } from "next/navigation";
import { Card, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { PublicFormPageClient } from "./[slug]/page-client";

export function PublicFormsPageContent() {
  const searchParams = useSearchParams();
  const slug = searchParams.get("slug") ?? "";

  if (!slug) {
    return (
      <Card className="mx-auto max-w-2xl">
        <CardHeader>
          <CardTitle>Form not found</CardTitle>
          <CardDescription>Provide a form slug in the query string to load a public form.</CardDescription>
        </CardHeader>
      </Card>
    );
  }

  return <PublicFormPageClient slug={slug} />;
}
