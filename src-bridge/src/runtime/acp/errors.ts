// Error vocabulary for the ACP client. Subsequent tasks (T2–T5) will
// throw these and add typed fields (e.g. stderr capture for crashes,
// wire payload for protocol errors). For T1 they are bare classes that
// only set `name` so the rest of the module can `import { ... }` and
// `instanceof` them without forward-reference cycles.

/**
 * The peer (agent) sent a message that violates the ACP / JSON-RPC 2.0
 * contract — malformed JSON, missing `jsonrpc: "2.0"`, response id with
 * no pending request, etc.
 */
export class AcpProtocolError extends Error {
  constructor(message: string) {
    super(message);
    this.name = "AcpProtocolError";
  }
}

/**
 * The agent child process exited (cleanly or otherwise) while the
 * client still had pending work. Later tasks will attach the last 2 KB
 * of stderr and the exit code/signal here.
 */
export class AcpProcessCrash extends Error {
  constructor(message: string) {
    super(message);
    this.name = "AcpProcessCrash";
  }
}

/**
 * `AcpClient.cancel(sessionId)` did not observe the in-flight prompt
 * resolve with `stopReason === "cancelled"` within the cancel deadline
 * (T3 default: 2 s). Caller decides whether to escalate to dispose.
 */
export class AcpCancelTimeout extends Error {
  constructor(message: string) {
    super(message);
    this.name = "AcpCancelTimeout";
  }
}

/**
 * The underlying NDJSON transport was closed (locally via `dispose()`
 * or remotely via stdout EOF) while a JSON-RPC request was pending.
 * All such pending promises reject with this error.
 */
export class AcpTransportClosed extends Error {
  constructor(message: string) {
    super(message);
    this.name = "AcpTransportClosed";
  }
}

/**
 * A second `prompt()` was issued against a session that already has an
 * in-flight prompt. ACP serializes prompts per session — the second
 * call rejects immediately rather than queuing.
 */
export class AcpConcurrentPrompt extends Error {
  constructor(message: string) {
    super(message);
    this.name = "AcpConcurrentPrompt";
  }
}
