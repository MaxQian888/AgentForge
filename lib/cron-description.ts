const COMMON_PATTERNS: Record<string, string> = {
  "* * * * *": "Every minute",
  "*/1 * * * *": "Every minute",
  "*/5 * * * *": "Every 5 minutes",
  "*/10 * * * *": "Every 10 minutes",
  "*/15 * * * *": "Every 15 minutes",
  "*/30 * * * *": "Every 30 minutes",
  "0 * * * *": "Every hour",
  "0 */2 * * *": "Every 2 hours",
  "0 */3 * * *": "Every 3 hours",
  "0 */4 * * *": "Every 4 hours",
  "0 */6 * * *": "Every 6 hours",
  "0 */12 * * *": "Every 12 hours",
  "0 0 * * *": "Daily at midnight",
  "0 3 * * *": "Daily at 3:00 AM",
  "0 0 * * 0": "Weekly on Sunday",
  "0 0 * * 1": "Weekly on Monday",
  "0 0 1 * *": "Monthly on the 1st",
  "0 0 * * 1-5": "Weekdays at midnight",
};

export function describeCron(expression: string): string {
  if (!expression) {
    return "";
  }

  const trimmed = expression.trim();
  if (COMMON_PATTERNS[trimmed]) {
    return COMMON_PATTERNS[trimmed];
  }

  const parts = trimmed.split(/\s+/);
  if (parts.length !== 5) {
    return trimmed;
  }

  const [minute, hour, dom, month, dow] = parts;

  if (minute.startsWith("*/") && hour === "*" && dom === "*" && month === "*" && dow === "*") {
    const interval = minute.slice(2);
    return `Every ${interval} minutes`;
  }

  if (minute === "0" && hour.startsWith("*/") && dom === "*" && month === "*" && dow === "*") {
    const interval = hour.slice(2);
    return `Every ${interval} hours`;
  }

  if (minute !== "*" && hour !== "*" && dom === "*" && month === "*" && dow === "*") {
    return `Daily at ${hour.padStart(2, "0")}:${minute.padStart(2, "0")}`;
  }

  return trimmed;
}
