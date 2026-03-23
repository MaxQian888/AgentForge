"use client";

import { Shield } from "lucide-react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";

interface PresetRole {
  name: string;
  description: string;
  tags: string[];
  model: string;
}

const presetRoles: PresetRole[] = [
  {
    name: "Senior Developer",
    description:
      "Writes production-ready code with tests. Follows best practices and project conventions.",
    tags: ["coding", "testing", "review"],
    model: "claude-sonnet-4-6",
  },
  {
    name: "Tech Lead",
    description:
      "Reviews architecture decisions, breaks down epics into tasks, and coordinates agent work.",
    tags: ["planning", "review", "architecture"],
    model: "claude-opus-4-6",
  },
  {
    name: "QA Engineer",
    description:
      "Writes and runs test suites, verifies bug fixes, performs regression testing.",
    tags: ["testing", "automation", "quality"],
    model: "claude-sonnet-4-6",
  },
  {
    name: "DevOps Engineer",
    description:
      "Manages CI/CD pipelines, Docker configs, and deployment scripts.",
    tags: ["infra", "ci-cd", "docker"],
    model: "claude-sonnet-4-6",
  },
  {
    name: "Technical Writer",
    description:
      "Writes documentation, API references, and user guides.",
    tags: ["docs", "api", "writing"],
    model: "claude-haiku-4-5",
  },
  {
    name: "Security Analyst",
    description:
      "Audits code for vulnerabilities, reviews dependencies, suggests fixes.",
    tags: ["security", "audit", "review"],
    model: "claude-sonnet-4-6",
  },
];

export default function RolesPage() {
  return (
    <div className="flex flex-col gap-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold">Role Configuration</h1>
      </div>

      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
        {presetRoles.map((role) => (
          <Card key={role.name}>
            <CardHeader className="flex flex-row items-center gap-3 pb-2">
              <Shield className="size-5 text-primary" />
              <CardTitle className="text-base">{role.name}</CardTitle>
            </CardHeader>
            <CardContent>
              <p className="mb-3 text-sm text-muted-foreground">
                {role.description}
              </p>
              <div className="mb-2 flex flex-wrap gap-1.5">
                {role.tags.map((tag) => (
                  <Badge key={tag} variant="secondary" className="text-xs">
                    {tag}
                  </Badge>
                ))}
              </div>
              <p className="text-xs text-muted-foreground">
                Model: {role.model}
              </p>
            </CardContent>
          </Card>
        ))}
      </div>
    </div>
  );
}
