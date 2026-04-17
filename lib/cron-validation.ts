// Lightweight cron expression validator for the standard 5-field syntax:
//   minute hour day-of-month month day-of-week
//
// Supports:
//   - wildcard '*'
//   - numeric values within the allowed range
//   - comma-separated lists (1,5,10)
//   - ranges (1-5)
//   - step values with /N (e.g. '*' slash 5, or '0-30' slash 10)
//   - day-of-week short names (SUN-SAT) and month short names (JAN-DEC)
//
// Returns an error message when invalid, or null when the expression parses.

interface FieldRange {
  name: string;
  min: number;
  max: number;
  aliases?: Record<string, number>;
}

const FIELD_RANGES: FieldRange[] = [
  { name: "minute", min: 0, max: 59 },
  { name: "hour", min: 0, max: 23 },
  { name: "day-of-month", min: 1, max: 31 },
  {
    name: "month",
    min: 1,
    max: 12,
    aliases: {
      JAN: 1,
      FEB: 2,
      MAR: 3,
      APR: 4,
      MAY: 5,
      JUN: 6,
      JUL: 7,
      AUG: 8,
      SEP: 9,
      OCT: 10,
      NOV: 11,
      DEC: 12,
    },
  },
  {
    name: "day-of-week",
    min: 0,
    max: 7,
    aliases: { SUN: 0, MON: 1, TUE: 2, WED: 3, THU: 4, FRI: 5, SAT: 6 },
  },
];

function resolveAliasedNumber(
  raw: string,
  aliases: Record<string, number> | undefined,
): number | null {
  const trimmed = raw.trim().toUpperCase();
  if (aliases && trimmed in aliases) {
    return aliases[trimmed];
  }
  if (!/^-?\d+$/.test(trimmed)) {
    return null;
  }
  return Number.parseInt(trimmed, 10);
}

function validateSegment(segment: string, field: FieldRange): string | null {
  const [valuePart, stepPart] = segment.split("/");
  if (valuePart === undefined || valuePart === "") {
    return `Invalid ${field.name} value`;
  }
  if (stepPart !== undefined) {
    if (!/^\d+$/.test(stepPart) || Number.parseInt(stepPart, 10) <= 0) {
      return `Invalid step in ${field.name}`;
    }
  }

  if (valuePart === "*") {
    return null;
  }

  if (valuePart.includes("-")) {
    const [startRaw, endRaw] = valuePart.split("-");
    const start = resolveAliasedNumber(startRaw ?? "", field.aliases);
    const end = resolveAliasedNumber(endRaw ?? "", field.aliases);
    if (start === null || end === null) {
      return `Invalid ${field.name} range`;
    }
    if (start < field.min || end > field.max || start > end) {
      return `${field.name} range out of bounds (${field.min}-${field.max})`;
    }
    return null;
  }

  const value = resolveAliasedNumber(valuePart, field.aliases);
  if (value === null) {
    return `Invalid ${field.name} value`;
  }
  if (value < field.min || value > field.max) {
    return `${field.name} must be between ${field.min} and ${field.max}`;
  }
  return null;
}

export function validateCronExpression(expression: string): string | null {
  if (typeof expression !== "string") {
    return "Cron expression is required";
  }
  const trimmed = expression.trim();
  if (!trimmed) {
    return "Cron expression is required";
  }

  const parts = trimmed.split(/\s+/);
  if (parts.length !== 5) {
    return "Expected 5 fields: minute hour day month weekday";
  }

  for (let i = 0; i < FIELD_RANGES.length; i += 1) {
    const part = parts[i]!;
    const field = FIELD_RANGES[i]!;
    const segments = part.split(",");
    for (const segment of segments) {
      const error = validateSegment(segment, field);
      if (error) {
        return error;
      }
    }
  }

  return null;
}

export function isValidCronExpression(expression: string): boolean {
  return validateCronExpression(expression) === null;
}
