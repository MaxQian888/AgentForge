import { z } from "zod";
import { anthropic } from "@ai-sdk/anthropic";
import { google } from "@ai-sdk/google";
import { openai } from "@ai-sdk/openai";
import { generateText } from "ai";
import {
  assertProviderCredentials,
  type ResolvedProviderSelection,
} from "../providers/registry.js";

export const GenerateRequestSchema = z.object({
  prompt: z.string().min(1),
  system_prompt: z.string().optional(),
  provider: z.string().min(1).optional(),
  model: z.string().min(1).optional(),
  max_tokens: z.number().int().positive().optional(),
  temperature: z.number().min(0).max(2).optional(),
});

export type GenerateRequest = z.infer<typeof GenerateRequestSchema>;

export interface GenerateResponse {
  text: string;
  usage: {
    input_tokens: number;
    output_tokens: number;
  };
}

export type GenerateExecutor = (params: {
  prompt: string;
  systemPrompt?: string;
  provider: ResolvedProviderSelection;
  maxTokens?: number;
  temperature?: number;
}) => Promise<GenerateResponse>;

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

function createDefaultExecutor(): GenerateExecutor {
  return async function generate(params): Promise<GenerateResponse> {
    assertProviderCredentials(params.provider);
    const model = createModel(params.provider);
    const opts: Record<string, unknown> = {
      model,
      prompt: params.prompt,
    };
    if (params.systemPrompt) opts.system = params.systemPrompt;
    if (params.maxTokens) opts.maxTokens = params.maxTokens;
    if (params.temperature !== undefined) opts.temperature = params.temperature;

    const result = await generateText(opts as Parameters<typeof generateText>[0]);
    const usage = result.usage as unknown as Record<string, number> | undefined;
    return {
      text: result.text,
      usage: {
        input_tokens: usage?.promptTokens ?? usage?.input_tokens ?? 0,
        output_tokens: usage?.completionTokens ?? usage?.output_tokens ?? 0,
      },
    };
  };
}

const defaultExecutor = createDefaultExecutor();

export async function handleGenerate(
  req: GenerateRequest,
  provider: ResolvedProviderSelection,
  executor: GenerateExecutor = defaultExecutor,
): Promise<GenerateResponse> {
  return executor({
    prompt: req.prompt,
    systemPrompt: req.system_prompt,
    provider,
    maxTokens: req.max_tokens,
    temperature: req.temperature,
  });
}
