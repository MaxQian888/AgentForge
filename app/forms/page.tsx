import { Suspense } from "react";
import { useTranslations } from "next-intl";
import { Card, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { PublicFormsPageContent } from "./page-content";

function PublicFormsPageFallback() {
  const t = useTranslations("forms");
  return (
    <Card className="mx-auto max-w-2xl">
      <CardHeader>
        <CardTitle>{t("loadingForm")}</CardTitle>
        <CardDescription>{t("checkingForm")}</CardDescription>
      </CardHeader>
    </Card>
  );
}

export default function PublicFormsPage() {
  return (
    <Suspense fallback={<PublicFormsPageFallback />}>
      <PublicFormsPageContent />
    </Suspense>
  );
}
