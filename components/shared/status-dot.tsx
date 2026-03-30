"use client";

import { cn } from "@/lib/utils";
import { getStatusColor } from "@/lib/constants/status-colors";

interface StatusDotProps {
  status: string;
  size?: "sm" | "md";
  pulse?: boolean;
  className?: string;
}

export function StatusDot({
  status,
  size = "sm",
  pulse,
  className,
}: StatusDotProps) {
  const colors = getStatusColor(status);
  const shouldPulse =
    pulse ??
    ["active", "running", "executing"].includes(status.toLowerCase());

  return (
    <span
      className={cn(
        "inline-block shrink-0 rounded-full",
        size === "sm" ? "size-2" : "size-2.5",
        colors.dot,
        shouldPulse && "animate-pulse-dot",
        className
      )}
      role="status"
      aria-label={status}
    />
  );
}
