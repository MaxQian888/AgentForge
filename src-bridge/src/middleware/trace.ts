import type { MiddlewareHandler } from "hono";
import { randomBytes } from "node:crypto";

const CROCKFORD = "0123456789abcdefghjkmnpqrstvwxyz";

/**
 * Generate a 27-character trace id matching the Go side:
 *   "tr_" + 24 crockford-base32 chars from 15 random bytes (120 bits).
 * The encoding is deterministic, compatible with a lowercase crockford alphabet.
 */
export function newTraceId(): string {
  const bytes = randomBytes(15);
  let out = "";
  let bits = 0;
  let buf = 0;
  for (let i = 0; i < bytes.length; i++) {
    buf = (buf << 8) | bytes[i]!;
    bits += 8;
    while (bits >= 5) {
      bits -= 5;
      out += CROCKFORD[(buf >> bits) & 0x1f];
    }
  }
  if (bits > 0) out += CROCKFORD[(buf << (5 - bits)) & 0x1f];
  return "tr_" + out.slice(0, 24);
}

declare module "hono" {
  interface ContextVariableMap {
    traceId: string;
  }
}

export interface TraceMiddlewareOptions {
  /** Called once per request when trace_id was generated (no inbound header). */
  onMidchain?: (id: string) => void;
}

/**
 * traceMiddleware resolves a correlation id per request:
 *   X-Trace-ID → X-Request-ID → freshly generated.
 * Attaches the id to c.var.traceId and echoes it on the response.
 */
export function traceMiddleware(opts: TraceMiddlewareOptions = {}): MiddlewareHandler {
  return async (c, next) => {
    const inbound = c.req.header("X-Trace-ID") ?? c.req.header("X-Request-ID") ?? "";
    const id = inbound || newTraceId();
    if (!inbound && opts.onMidchain) {
      opts.onMidchain(id);
    }
    c.set("traceId", id);
    c.header("X-Trace-ID", id);
    await next();
  };
}
