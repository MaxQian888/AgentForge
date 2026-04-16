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

export interface AcpProcessCrashDetails {
  stderrTail: string;
  exitCode: number;
  signal?: string;
}

/**
 * The agent child process exited (cleanly or otherwise) while the
 * client still had pending work. Carries the last 2 KB of stderr and
 * the exit code/signal per Spec §4.4.
 */
export class AcpProcessCrash extends Error {
  readonly stderrTail: string;
  readonly exitCode: number;
  readonly signal?: string;
  constructor(message: string, details: AcpProcessCrashDetails) {
    super(message);
    this.name = "AcpProcessCrash";
    this.stderrTail = details.stderrTail;
    this.exitCode = details.exitCode;
    this.signal = details.signal;
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

/**
 * A method was invoked on an `AcpSession` that the negotiated agent
 * capabilities do not advertise. Callers map this to structured
 * `{support_state:"unsupported", reason_code}` per runtime-capability
 * contract.
 */
export class AcpCapabilityUnsupported extends Error {
  readonly method: string;
  readonly reason: string;
  constructor(method: string, reason: string) {
    super(`ACP capability unsupported: ${method} (${reason})`);
    this.name = "AcpCapabilityUnsupported";
    this.method = method;
    this.reason = reason;
  }
}

/**
 * Adapter cannot be spawned because required env variables are absent.
 * Raised inside `AcpConnectionPool.acquire` before any process is forked.
 */
export class AcpAuthMissing extends Error {
  readonly adapterId: string;
  readonly missingEnv: string[];
  constructor(adapterId: string, missingEnv: string[]) {
    super(`ACP auth missing for ${adapterId}: ${missingEnv.join(", ")}`);
    this.name = "AcpAuthMissing";
    this.adapterId = adapterId;
    this.missingEnv = missingEnv;
  }
}

/**
 * `ChildProcessHost.start()` failed because the binary resolved from
 * `ACP_ADAPTERS[adapterId].command` is not on PATH. User must install it.
 */
export class AcpCommandNotFound extends Error {
  readonly adapterId: string;
  readonly command: string;
  constructor(adapterId: string, command: string) {
    super(`ACP command not found for ${adapterId}: ${command}`);
    this.name = "AcpCommandNotFound";
    this.adapterId = adapterId;
    this.command = command;
  }
}
