"use client";

import { cn } from "@/lib/utils";
import { useTheme } from "@/lib/theme/provider";

const THEME_OPTIONS = [
  { value: "light", label: "Light" },
  { value: "dark", label: "Dark" },
  { value: "system", label: "System" },
] as const;

export function ThemeToggle({ className }: { className?: string }) {
  const { theme, setTheme } = useTheme();

  return (
    <div className={cn("inline-flex rounded-md border", className)}>
      {THEME_OPTIONS.map(({ value, label }) => (
        <button
          key={value}
          type="button"
          onClick={() => setTheme(value)}
          className={cn(
            "px-3 py-1.5 text-sm font-medium transition-colors first:rounded-l-md last:rounded-r-md",
            theme === value
              ? "bg-primary text-primary-foreground"
              : "bg-transparent text-muted-foreground hover:bg-muted hover:text-foreground"
          )}
        >
          {label}
        </button>
      ))}
    </div>
  );
}
