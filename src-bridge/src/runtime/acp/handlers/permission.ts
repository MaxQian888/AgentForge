import type {
  RequestPermissionRequest,
  RequestPermissionResponse,
} from "@agentclientprotocol/sdk";
import type { PerSessionContext } from "../multiplexed-client.js";

/**
 * session/request_permission handler: delegates the decision to the
 * per-task `permissionRouter`, which owns the `request_id` allocation,
 * `permission_request` event emission via `EventStreamer`, and the
 * `/bridge/permission-response/:request_id` round trip via
 * `HookCallbackManager`.
 *
 * Timeout + cancel-mid-prompt handling lives inside the router so the
 * handler stays a thin dispatch.
 */
export async function handle(
  ctx: PerSessionContext,
  params: RequestPermissionRequest,
): Promise<RequestPermissionResponse> {
  const toolCall = (params as unknown as { toolCall: Parameters<PerSessionContext["permissionRouter"]["request"]>[1] }).toolCall;
  const options = (params as unknown as { options?: Parameters<PerSessionContext["permissionRouter"]["request"]>[2] }).options ?? [];
  const outcome = await ctx.permissionRouter.request(ctx.taskId, toolCall, options);
  return { outcome };
}
