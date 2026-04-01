"use client";

import { useEffect } from "react";
import { AlertTriangle, RotateCcw } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardFooter, CardHeader, CardTitle } from "@/components/ui/card";

export default function DashboardError({
  error,
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  useEffect(() => {
    console.error("[DashboardError]", error);
  }, [error]);

  return (
    <div className="flex flex-1 items-center justify-center p-6">
      <Card className="w-full max-w-md">
        <CardHeader className="text-center">
          <div className="mx-auto mb-2 flex h-12 w-12 items-center justify-center rounded-full bg-destructive/10">
            <AlertTriangle className="h-6 w-6 text-destructive" />
          </div>
          <CardTitle>Page Error</CardTitle>
        </CardHeader>
        <CardContent className="text-center text-sm text-muted-foreground">
          <p>This page encountered an error and could not be rendered.</p>
          {error.message && (
            <p className="mt-2 rounded bg-muted px-3 py-2 text-left font-mono text-xs">
              {error.message}
            </p>
          )}
          {error.digest && (
            <p className="mt-2 font-mono text-xs text-muted-foreground/60">
              Error ID: {error.digest}
            </p>
          )}
        </CardContent>
        <CardFooter className="justify-center gap-2">
          <Button variant="outline" onClick={() => (window.location.href = "/")}>
            Back to Dashboard
          </Button>
          <Button onClick={reset}>
            <RotateCcw className="mr-1.5 h-4 w-4" />
            Retry
          </Button>
        </CardFooter>
      </Card>
    </div>
  );
}
