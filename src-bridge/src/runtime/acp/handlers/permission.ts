// T4c placeholder: real implementation lands in T4c (HookCallbackManager wiring).
import type {
  RequestPermissionRequest,
  RequestPermissionResponse,
} from "@agentclientprotocol/sdk";
import type { PerSessionContext } from "../multiplexed-client.js";

/* eslint-disable @typescript-eslint/no-unused-vars */
export async function handle(
  _ctx: PerSessionContext,
  _params: RequestPermissionRequest,
): Promise<RequestPermissionResponse> {
  throw new Error("permission.handle not yet implemented (T4c)");
}
