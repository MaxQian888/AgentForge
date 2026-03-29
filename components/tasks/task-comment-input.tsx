"use client";

import { useMemo, useState } from "react";
import { useTranslations } from "next-intl";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";

export function TaskCommentInput({
  onSubmit,
  suggestions = [],
}: {
  onSubmit: (body: string) => void | Promise<void>;
  suggestions?: string[];
}) {
  const t = useTranslations("tasks");
  const [value, setValue] = useState("");
  const suggestionItems = useMemo(() => {
    const match = value.match(/@([\w-]*)$/);
    if (!match) return [];
    return suggestions.filter((item) =>
      item.toLowerCase().includes(match[1].toLowerCase()),
    );
  }, [suggestions, value]);

  return (
    <div className="flex flex-col gap-2">
      <Input
        placeholder={t("comments.placeholder")}
        value={value}
        onChange={(event) => setValue(event.target.value)}
      />
      {suggestionItems.length > 0 ? (
        <div className="flex flex-wrap gap-2 text-xs text-muted-foreground">
          {suggestionItems.map((item) => (
            <button
              key={item}
              type="button"
              className="rounded-full border px-2 py-1 hover:bg-accent"
              onClick={() =>
                setValue((current) => current.replace(/@([\w-]*)$/, `@${item}`))
              }
            >
              @{item}
            </button>
          ))}
        </div>
      ) : null}
      <Button
        type="button"
        size="sm"
        onClick={async () => {
          if (!value.trim()) return;
          await onSubmit(value);
          setValue("");
        }}
      >
        {t("comments.submit")}
      </Button>
    </div>
  );
}
