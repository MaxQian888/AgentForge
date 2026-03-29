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

  if (roleConfig.loaded_skills?.length) {
    parts.push([
      "## Loaded Skills",
      ...roleConfig.loaded_skills.map((skill) => {
        const blocks = [`### ${skill.label} (${skill.path})`];
        if (skill.description) {
          blocks.push(skill.description);
        }
        if (skill.instructions) {
          blocks.push(skill.instructions);
        }
        return blocks.join("\n\n");
      }),
    ].join("\n\n"));
  }

  if (roleConfig.available_skills?.length) {
    parts.push([
      "## Available On-Demand Skills",
      ...roleConfig.available_skills.map((skill) =>
        skill.description
          ? `- ${skill.label} (${skill.path}): ${skill.description}`
          : `- ${skill.label} (${skill.path})`,
      ),
    ].join("\n"));
  }

  parts.push("---", basePrompt);
  return parts.join("\n\n");
}

export function filterTools(tools: string[], roleConfig?: RoleConfig): string[] {
  if (!roleConfig?.allowed_tools?.length) return tools;
  return tools.filter((t) => roleConfig.allowed_tools.includes(t));
}
