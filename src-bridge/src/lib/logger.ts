import pino from "pino";
import type { Logger } from "pino";

export interface CreateLoggerOpts {
  level?: "debug" | "info" | "warn" | "error";
  write?: (chunk: string) => void;
}

export function createLogger(opts: CreateLoggerOpts = {}): Logger {
  const level = opts.level ?? (process.env.LOG_LEVEL as CreateLoggerOpts["level"]) ?? "info";
  const base = { service: "ts-bridge" };
  if (opts.write) {
    const sink = { write: opts.write };
    return pino({ level, base }, sink as unknown as pino.DestinationStream);
  }
  return pino({ level, base });
}

export function withTrace(logger: Logger, traceId: string): Logger {
  return logger.child({ trace_id: traceId });
}
