import { z } from "zod";
import { anthropic } from "@ai-sdk/anthropic";
import { google } from "@ai-sdk/google";
import { openai } from "@ai-sdk/openai";
import { generateText, Output } from "ai";
import {
  assertProviderCredentials,
  type ResolvedProviderSelection,
} from "../providers/registry.js";

export const ClassifyIntentRequestSchema = z.object({
  text: z.string().min(1),
  user_id: z.string().default(""),
  project_id: z.string().default(""),
  candidates: z.array(z.string().min(1)).default([]),
  context: z.unknown().optional(),
});

export const ClassifyIntentResponseSchema = z.object({
  intent: z.string(),
  command: z.string(),
  args: z.string(),
  confidence: z.number().min(0).max(1),
  reply: z.string().optional(),
});

export type ClassifyIntentRequest = z.input<typeof ClassifyIntentRequestSchema>;
export type ClassifyIntentResponse = z.infer<
  typeof ClassifyIntentResponseSchema
>;

export type IntentClassifierExecutor = (params: {
  prompt: string;
  provider: ResolvedProviderSelection;
}) => Promise<ClassifyIntentResponse>;

function buildClassifyIntentPrompt(req: ClassifyIntentRequest): string {
  const candidates = req.candidates ?? [];
  const sections = [
    "You are an intent classifier for AgentForge, a development management platform.",
    "Given a user's natural language message, classify it into one of the following intents and extract the corresponding command and arguments.",
    "",
    "Available commands:",
    "  /task create <title>       — Create a new task",
    "  /task list [status]        — List tasks",
    "  /task status <id>          — Check task status",
    "  /task assign <id> <person> — Assign a task",
    "  /task decompose <id>       — AI decompose a task",
    "  /agent spawn <task-id>     — Start an agent for a task",
    "  /agent run <prompt>        — Run an agent with a prompt",
    "  /agent list                — List agent pool status",
    "  /review <pr-url>           — Trigger code review",
    "  /sprint status             — Current sprint overview",
    "  /cost                      — Show cost statistics",
    "  /help                      — Show help",
    "",
    "If the message maps to a command, set intent to a descriptive name (e.g. 'create_task'), command to the slash command, and args to extracted arguments.",
    "If the message is greeting or casual conversation, set intent to 'chat' and provide a friendly reply.",
    "If you can't determine the intent, set intent to 'unknown' and provide a helpful reply.",
    "",
  ];
  if (candidates.length > 0) {
    sections.push(`Allowed intents: ${candidates.join(", ")}`);
  }
  if (req.context !== undefined) {
    sections.push(`Conversation context: ${JSON.stringify(req.context)}`);
  }
  sections.push("", `User message: ${req.text}`);
  return sections.join("\n");
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

function createDefaultExecutor(): IntentClassifierExecutor {
  return async function classifyIntent(params: {
    prompt: string;
    provider: ResolvedProviderSelection;
  }): Promise<ClassifyIntentResponse> {
    assertProviderCredentials(params.provider);
    const model = createModel(params.provider);
    const result = await generateText({
      model,
      prompt: params.prompt,
      output: Output.object({
        schema: ClassifyIntentResponseSchema,
      }),
    });
    return result.output;
  };
}

const defaultExecutor = createDefaultExecutor();

export async function handleClassifyIntent(
  req: ClassifyIntentRequest,
  provider: ResolvedProviderSelection,
  executor: IntentClassifierExecutor = defaultExecutor,
): Promise<ClassifyIntentResponse> {
  const prompt = buildClassifyIntentPrompt(req);
  const result = await executor({ prompt, provider });
  const parsed = ClassifyIntentResponseSchema.safeParse(result);
  if (!parsed.success) {
    return {
      intent: "unknown",
      command: "",
      args: "",
      confidence: 0,
      reply: "无法理解您的请求。请使用 /help 查看可用命令。",
    };
  }
  return parsed.data;
}
