"use client";

import { useMemo, useState } from "react";
import { useTranslations } from "next-intl";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";

export function CommentInput({
  onSubmit,
  suggestions = [],
  placeholder,
}: {
  onSubmit: (body: string) => void | Promise<void>;
  suggestions?: string[];
  placeholder?: string;
}) {
  const t = useTranslations("docs");
  const [value, setValue] = useState("");
  const suggestionItems = useMemo(() => {
    const match = value.match(/@([\w-]*)$/);
    if (!match) return [];
    return suggestions.filter((item) =>
      item.toLowerCase().includes(match[1].toLowerCase())
    );
  }, [suggestions, value]);

  return (
    <div className="flex flex-col gap-2">
      <Input
        value={value}
        placeholder={placeholder ?? t("comments.writeComment")}
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
                setValue((current) => current.replace(/@([\w-]*)$/, `@${item} `))
              }
            >
              @{item}
            </button>
          ))}
        </div>
      ) : null}
      <Button
        size="sm"
        onClick={async () => {
          if (!value.trim()) return;
          await onSubmit(value);
          setValue("");
        }}
      >
        {t("comments.comment")}
      </Button>
    </div>
  );
}
