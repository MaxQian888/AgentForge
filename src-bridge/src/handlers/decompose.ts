import { DecomposeTaskResponseSchema } from "../schemas.js";
import type {
  DecomposeTaskRequest,
  DecomposeTaskResponse,
} from "../types.js";
import {
  runTextGeneration,
  type TextGenerationRunner,
} from "../providers/text-generation.js";
import type { ResolvedProviderSelection } from "../providers/registry.js";

export type DecomposeTaskExecutor = (
  params: {
    prompt: string;
    req: DecomposeTaskRequest;
    provider: ResolvedProviderSelection;
  },
) => Promise<DecomposeTaskResponse>;

export function buildDecompositionPrompt(req: DecomposeTaskRequest): string {
  const promptSections = [
    "You are decomposing an engineering task into implementation-ready subtasks.",
    "Return valid JSON only with this shape: {\"summary\": string, \"subtasks\": [{\"title\": string, \"description\": string, \"priority\": \"critical\"|\"high\"|\"medium\"|\"low\", \"executionMode\": \"human\"|\"agent\"}]}",
    "Return a concise summary and a small ordered list of subtasks.",
    "Each subtask must have a title, description, one priority from critical/high/medium/low, and one executionMode of human or agent.",
    "Use executionMode=agent for repetitive, well-bounded implementation work. Use executionMode=human for tasks that need architecture judgment, product coordination, security review, or stakeholder decisions.",
    `Task ID: ${req.task_id}`,
    `Title: ${req.title}`,
    `Priority: ${req.priority}`,
    `Description:\n${req.description}`,
  ];

  if (req.context !== undefined) {
    promptSections.push("Additional Context");
    promptSections.push(JSON.stringify(req.context));
  }

  return promptSections.join("\n\n");
}

export function createTextGenerationDecomposeExecutor(
  runner?: TextGenerationRunner,
): DecomposeTaskExecutor {
  return async function generateDecomposition(
    params: {
      prompt: string;
      req: DecomposeTaskRequest;
      provider: ResolvedProviderSelection;
    },
  ): Promise<DecomposeTaskResponse> {
    const result = await runTextGeneration(params.provider, params.prompt, runner);
    return result.output;
  };
}

const defaultDecomposeExecutor = createTextGenerationDecomposeExecutor();

async function generateDecomposition(
  params: {
    prompt: string;
    req: DecomposeTaskRequest;
    provider: ResolvedProviderSelection;
  },
): Promise<DecomposeTaskResponse> {
  return defaultDecomposeExecutor(params);
}

export async function handleDecompose(
  req: DecomposeTaskRequest,
  provider: ResolvedProviderSelection,
  executor: DecomposeTaskExecutor = generateDecomposition,
): Promise<DecomposeTaskResponse> {
  const prompt = buildDecompositionPrompt(req);
  const result = await executor({ prompt, req, provider });
  const parsed = DecomposeTaskResponseSchema.safeParse(result);
  if (!parsed.success) {
    throw new Error("Invalid decomposition output");
  }
  return parsed.data;
}
