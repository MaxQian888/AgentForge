export { ACP_ADAPTERS, type AdapterId, type AcpAdapterConfig } from "./registry.js";
export {
  AcpProtocolError,
  AcpProcessCrash,
  type AcpProcessCrashDetails,
  AcpCancelTimeout,
  AcpTransportClosed,
  AcpConcurrentPrompt,
  AcpCapabilityUnsupported,
  AcpAuthMissing,
  AcpCommandNotFound,
} from "./errors.js";
export { ChildProcessHost, RingBuffer, type ChildProcessHostOptions, type Logger } from "./process-host.js";
export {
  AcpConnectionPool,
  type AcpConnectionPoolOptions,
  type PooledEntry,
  type PooledEntryFactory,
  type AcquireContext,
} from "./connection-pool.js";
export { createPooledEntryFactory, type PooledEntryFactoryOpts } from "./connection-pool-factory.js";
export { MultiplexedClient, type PerSessionContext } from "./multiplexed-client.js";
export { AcpSession, type AcpSessionOptions } from "./session.js";
export {
  createAcpRuntimeAdapter,
  type AcpRuntimeAdapter,
  type AcpRuntimeAdapterFactory,
  type AcpDeps,
  type AcpTaskInput,
} from "./adapter-factory.js";
export { FsSandbox } from "./fs-sandbox.js";
export { TerminalManager, type TerminalManagerOpts, type TerminalExitInfo } from "./terminal-manager.js";
export {
  liveControlsFor,
  gateUnstable,
  type LiveControlsFlags,
  type LiveControlsInput,
} from "./capabilities.js";
export { mapSessionUpdate, type MappedEvent } from "./events/session-update.js";
