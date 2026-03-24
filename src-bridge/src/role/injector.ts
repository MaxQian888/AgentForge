import type { RoleConfig } from "../types.js";

export function buildSystemPrompt(basePrompt: string, roleConfig?: RoleConfig): string {
  if (!roleConfig) return basePrompt;

  return `# Role: ${roleConfig.role}
## Goal
${roleConfig.goal}
## Backstory
${roleConfig.backstory}

${roleConfig.system_prompt}

---
${basePrompt}`;
}

export function filterTools(tools: string[], roleConfig?: RoleConfig): string[] {
  if (!roleConfig?.allowed_tools?.length) return tools;
  return tools.filter((t) => roleConfig.allowed_tools.includes(t));
}
