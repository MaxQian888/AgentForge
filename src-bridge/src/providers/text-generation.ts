import { anthropic } from "@ai-sdk/anthropic";
import { google } from "@ai-sdk/google";
import { openai } from "@ai-sdk/openai";
import { generateText, Output } from "ai";
import { DecomposeTaskResponseSchema } from "../schemas.js";
import type { DecomposeTaskResponse } from "../types.js";
import { assertProviderCredentials, type ResolvedProviderSelection } from "./registry.js";

export interface TextGenerationResult {
  output: DecomposeTaskResponse;
}

export type TextGenerationRunner = (params: {
  provider: ResolvedProviderSelection;
  prompt: string;
}) => Promise<TextGenerationResult>;

export async function runTextGeneration(
  provider: ResolvedProviderSelection,
  prompt: string,
  runner: TextGenerationRunner = defaultTextGenerationRunner,
): Promise<TextGenerationResult> {
  assertProviderCredentials(provider);
  return runner({ provider, prompt });
}

async function defaultTextGenerationRunner({
  provider,
  prompt,
}: {
  provider: ResolvedProviderSelection;
  prompt: string;
}): Promise<TextGenerationResult> {
  const model = createModel(provider);
  const result = await generateText({
    model,
    prompt,
    output: Output.object({
      schema: DecomposeTaskResponseSchema,
    }),
  });

  return { output: result.output };
}

function createModel(provider: ResolvedProviderSelection) {
  switch (provider.provider) {
    case "anthropic":
      return anthropic(provider.model);
    case "openai":
      return openai(provider.model);
    case "google":
      return google(provider.model);
  }
}
