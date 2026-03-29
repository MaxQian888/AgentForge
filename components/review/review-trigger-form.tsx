"use client";

import { useMemo, useState } from "react";
import { useTranslations } from "next-intl";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";

interface ReviewTriggerFormProps {
  open: boolean;
  loading?: boolean;
  onOpenChange: (open: boolean) => void;
  onSubmit: (prUrl: string) => void | Promise<void>;
}

function isValidPullRequestUrl(value: string): boolean {
  try {
    const url = new URL(value);
    return (url.protocol === "http:" || url.protocol === "https:") && url.hostname.length > 0;
  } catch {
    return false;
  }
}

export function ReviewTriggerForm({
  open,
  loading = false,
  onOpenChange,
  onSubmit,
}: ReviewTriggerFormProps) {
  const t = useTranslations("reviews");
  const [prUrl, setPrUrl] = useState("");
  const [submitted, setSubmitted] = useState(false);

  const normalizedValue = prUrl.trim();
  const isValid = useMemo(
    () => normalizedValue.length > 0 && isValidPullRequestUrl(normalizedValue),
    [normalizedValue],
  );

  const handleSubmit = async () => {
    setSubmitted(true);
    if (!isValid) {
      return;
    }

    await onSubmit(normalizedValue);
    setPrUrl("");
    setSubmitted(false);
  };

  if (!open) {
    return null;
  }

  return (
    <div className="flex flex-col gap-2 rounded-md border p-3">
      <Label className="text-xs">{t("prUrlLabel")}</Label>
      <Input
        value={prUrl}
        onChange={(event) => setPrUrl(event.target.value)}
        placeholder="https://github.com/org/repo/pull/123"
        className="h-8 text-sm"
      />
      {submitted && !isValid ? (
        <p className="text-xs text-red-600 dark:text-red-400">
          {t("invalidPrUrl")}
        </p>
      ) : null}
      <div className="flex items-center gap-2">
        <Button
          size="sm"
          onClick={() => {
            void handleSubmit();
          }}
          disabled={loading}
        >
          {t("submitTrigger")}
        </Button>
        <Button
          size="sm"
          variant="outline"
          onClick={() => {
            setPrUrl("");
            setSubmitted(false);
            onOpenChange(false);
          }}
        >
          {t("cancelTrigger")}
        </Button>
      </div>
    </div>
  );
}
