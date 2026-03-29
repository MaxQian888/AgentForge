import { Suspense } from "react";
import { Card, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { PublicFormsPageContent } from "./page-content";

function PublicFormsPageFallback() {
  return (
    <Card className="mx-auto max-w-2xl">
      <CardHeader>
        <CardTitle>Loading form</CardTitle>
        <CardDescription>Checking the requested form link...</CardDescription>
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
