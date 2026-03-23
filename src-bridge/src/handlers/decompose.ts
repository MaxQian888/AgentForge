import { DecomposeTaskResponseSchema } from "../schemas.js";
import type {
  DecomposeTaskRequest,
  DecomposeTaskResponse,
} from "../types.js";

export type DecomposeTaskExecutor = (
  prompt: string,
  req: DecomposeTaskRequest,
) => Promise<unknown>;

const priorityOrder: Array<DecomposeTaskRequest["priority"]> = [
  "critical",
  "high",
  "medium",
  "low",
];

function downgradePriority(
  priority: DecomposeTaskRequest["priority"],
  steps: number,
): DecomposeTaskRequest["priority"] {
  const index = priorityOrder.indexOf(priority);
  return priorityOrder[Math.min(index + steps, priorityOrder.length - 1)];
}

export function buildDecompositionPrompt(req: DecomposeTaskRequest): string {
  return [
    "You are decomposing an engineering task into implementation-ready subtasks.",
    "Return a concise summary and a small ordered list of subtasks.",
    "Each subtask must have a title, description, and one priority from critical/high/medium/low.",
    `Task ID: ${req.task_id}`,
    `Title: ${req.title}`,
    `Priority: ${req.priority}`,
    `Description:\n${req.description}`,
  ].join("\n\n");
}

async function simulateDecomposition(
  prompt: string,
  req: DecomposeTaskRequest,
): Promise<DecomposeTaskResponse> {
  const trimmedDescription = req.description.trim();
  return {
    summary: `Decomposed "${req.title}" into focused delivery steps based on the requested scope.`,
    subtasks: [
      {
        title: `Clarify ${req.title} contract`,
        description: `Review the current interfaces and constraints for ${req.title}. Context: ${trimmedDescription}`,
        priority: req.priority,
      },
      {
        title: `Implement ${req.title} backend flow`,
        description:
          "Add the core application logic and persistence updates required by the decomposition request.",
        priority: downgradePriority(req.priority, 1),
      },
      {
        title: `Verify ${req.title} integration`,
        description: `Validate the end-to-end behavior and edge cases covered by this request.\n\nPrompt basis:\n${prompt}`,
        priority: downgradePriority(req.priority, 2),
      },
    ],
  };
}

export async function handleDecompose(
  req: DecomposeTaskRequest,
  executor: DecomposeTaskExecutor = simulateDecomposition,
): Promise<DecomposeTaskResponse> {
  const prompt = buildDecompositionPrompt(req);
  const result = await executor(prompt, req);
  const parsed = DecomposeTaskResponseSchema.safeParse(result);
  if (!parsed.success) {
    throw new Error("Invalid decomposition output");
  }
  return parsed.data;
}
