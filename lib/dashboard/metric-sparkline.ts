export interface SparklinePoint {
  label: string;
  value: number;
}

interface SparklineOptions {
  days?: number;
  now?: string | Date;
}

interface TimestampedValue {
  timestamp: string | null | undefined;
}

interface SummedValue extends TimestampedValue {
  amount: number;
}

const DEFAULT_DAYS = 7;

function normalizeNow(now?: string | Date) {
  return now instanceof Date ? now : new Date(now ?? Date.now());
}

function startOfUtcDay(date: Date) {
  return new Date(Date.UTC(date.getUTCFullYear(), date.getUTCMonth(), date.getUTCDate()));
}

function toDateKey(date: Date) {
  return date.toISOString().slice(0, 10);
}

function buildDateKeys(now?: string | Date, days = DEFAULT_DAYS) {
  const anchor = startOfUtcDay(normalizeNow(now));

  return Array.from({ length: days }, (_, offset) => {
    const point = new Date(anchor);
    point.setUTCDate(anchor.getUTCDate() - (days - offset - 1));
    return toDateKey(point);
  });
}

function createSparklineMap(now?: string | Date, days = DEFAULT_DAYS) {
  const labels = buildDateKeys(now, days);
  const values = new Map(labels.map((label) => [label, 0]));

  return { labels, values };
}

function isTrackedDate(dateKey: string, values: Map<string, number>) {
  return values.has(dateKey);
}

export function buildRecentCountSparkline(
  items: TimestampedValue[],
  options?: SparklineOptions,
): SparklinePoint[] {
  const { labels, values } = createSparklineMap(options?.now, options?.days);

  for (const item of items) {
    if (!item.timestamp) {
      continue;
    }

    const label = toDateKey(new Date(item.timestamp));
    if (!isTrackedDate(label, values)) {
      continue;
    }

    values.set(label, (values.get(label) ?? 0) + 1);
  }

  return labels.map((label) => ({ label, value: values.get(label) ?? 0 }));
}

export function buildRecentSumSparkline(
  items: SummedValue[],
  options?: SparklineOptions,
): SparklinePoint[] {
  const { labels, values } = createSparklineMap(options?.now, options?.days);

  for (const item of items) {
    if (!item.timestamp) {
      continue;
    }

    const label = toDateKey(new Date(item.timestamp));
    if (!isTrackedDate(label, values)) {
      continue;
    }

    values.set(
      label,
      Number(((values.get(label) ?? 0) + item.amount).toFixed(2)),
    );
  }

  return labels.map((label) => ({ label, value: values.get(label) ?? 0 }));
}

export function buildSparklineTrend(points: SparklinePoint[]) {
  if (points.length < 2) {
    return undefined;
  }

  const first = points[0]?.value ?? 0;
  const last = points.at(-1)?.value ?? 0;
  const delta = last - first;

  return {
    direction:
      delta === 0 ? ("flat" as const) : delta > 0 ? ("up" as const) : ("down" as const),
    value: Math.round((Math.abs(delta) / Math.max(Math.abs(first), 1)) * 100),
  };
}
