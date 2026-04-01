"use client";

function escapeHighlightPattern(value: string): string {
  return value.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}

export function HighlightedText({
  text,
  query,
}: {
  text: string;
  query?: string;
}) {
  const normalizedQuery = query?.trim();

  if (!text || !normalizedQuery) {
    return <>{text}</>;
  }

  const matcher = new RegExp(`(${escapeHighlightPattern(normalizedQuery)})`, "ig");
  const parts = text.split(matcher);

  return (
    <>
      {parts
        .filter((part) => part.length > 0)
        .map((part, index) =>
          part.toLowerCase() === normalizedQuery.toLowerCase() ? (
            <mark
              key={`${part}-${index}`}
              className="rounded bg-amber-300/40 px-0.5 text-current dark:bg-amber-500/20"
            >
              {part}
            </mark>
          ) : (
            <span key={`${part}-${index}`}>{part}</span>
          )
        )}
    </>
  );
}
