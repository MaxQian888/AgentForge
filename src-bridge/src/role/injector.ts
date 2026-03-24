import type { RoleConfig } from "../types.js";

export function buildSystemPrompt(basePrompt: string, roleConfig?: RoleConfig): string {
  if (!roleConfig) return basePrompt;

  const parts = [
    `# Role: ${roleConfig.role}`,
    `## Goal\n${roleConfig.goal}`,
    `## Backstory\n${roleConfig.backstory}`,
    roleConfig.system_prompt,
  ];

  if (roleConfig.knowledge_context) {
    parts.push(`## Knowledge Context\n${roleConfig.knowledge_context}`);
  }

  parts.push("---", basePrompt);
  return parts.join("\n\n");
}

export function filterTools(tools: string[], roleConfig?: RoleConfig): string[] {
  if (!roleConfig?.allowed_tools?.length) return tools;
  return tools.filter((t) => roleConfig.allowed_tools.includes(t));
}
