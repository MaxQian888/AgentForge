"use client";

import { useSearchParams } from "next/navigation";
import { useTranslations } from "next-intl";
import { Card, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { PublicFormPageClient } from "./[slug]/page-client";

export function PublicFormsPageContent() {
  const t = useTranslations("forms");
  const searchParams = useSearchParams();
  const slug = searchParams.get("slug") ?? "";

  if (!slug) {
    return (
      <Card className="mx-auto max-w-2xl">
        <CardHeader>
          <CardTitle>{t("formNotFound")}</CardTitle>
          <CardDescription>{t("formNotFoundDesc")}</CardDescription>
        </CardHeader>
      </Card>
    );
  }

  return <PublicFormPageClient slug={slug} />;
}
