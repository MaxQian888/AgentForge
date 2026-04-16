import type {
  CreateElicitationRequest,
  CreateElicitationResponse,
} from "@agentclientprotocol/sdk";
import type { PerSessionContext } from "../multiplexed-client.js";

/**
 * unstable_createElicitation handler: emits `elicitation_request` on
 * the per-task streamer (so downstream observers can surface it if
 * desired) and returns `{action:"cancel"}` as the default response.
 *
 * Frontend has no UI this phase (spec §6.4). When a real elicitation
 * UX spec lands, this handler will grow a router pattern similar to
 * `permission.ts`. For now the passthrough keeps the wire connected
 * so agents that probe elicitation support see a well-formed
 * structured decline rather than an RPC error.
 */
export async function createElicitation(
  ctx: PerSessionContext,
  params: CreateElicitationRequest,
): Promise<CreateElicitationResponse> {
  ctx.streamer.emit({
    type: "elicitation_request",
    session_id: (params as unknown as { sessionId: string }).sessionId,
    payload: params,
  });
  return { action: "cancel" };
}
