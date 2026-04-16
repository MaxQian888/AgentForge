// T4d placeholder: real implementation lands in T4d (capability-gated passthrough).
import type {
  CreateElicitationRequest,
  CreateElicitationResponse,
} from "@agentclientprotocol/sdk";
import type { PerSessionContext } from "../multiplexed-client.js";

/* eslint-disable @typescript-eslint/no-unused-vars */
export async function createElicitation(
  _ctx: PerSessionContext,
  _params: CreateElicitationRequest,
): Promise<CreateElicitationResponse> {
  throw new Error("elicitation.createElicitation not yet implemented (T4d)");
}
