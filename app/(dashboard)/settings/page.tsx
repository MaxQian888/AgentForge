"use client";

import { useEffect, useState } from "react";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Separator } from "@/components/ui/separator";
import { useProjectStore } from "@/lib/stores/project-store";
import { useDashboardStore } from "@/lib/stores/dashboard-store";

export default function SettingsPage() {
  const { selectedProjectId } = useDashboardStore();
  const { projects, fetchProjects, updateProject } = useProjectStore();

  const project = projects.find((p) => p.id === selectedProjectId);

  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [repoUrl, setRepoUrl] = useState("");
  const [defaultBranch, setDefaultBranch] = useState("");
  const [saved, setSaved] = useState(false);

  useEffect(() => {
    void fetchProjects();
  }, [fetchProjects]);

  useEffect(() => {
    if (project) {
      setName(project.name);
      setDescription(project.description ?? "");
      setRepoUrl(project.repoUrl ?? "");
      setDefaultBranch(project.defaultBranch ?? "main");
    }
  }, [project]);

  const handleSave = async () => {
    if (!project) return;
    await updateProject(project.id, {
      name,
      description,
      repoUrl,
      defaultBranch,
    });
    setSaved(true);
    setTimeout(() => setSaved(false), 2000);
  };

  if (!selectedProjectId) {
    return (
      <div className="flex flex-col gap-6">
        <h1 className="text-2xl font-bold">Settings</h1>
        <p className="text-sm text-muted-foreground">
          Select a project from the Dashboard to configure settings.
        </p>
      </div>
    );
  }

  if (!project) {
    return (
      <div className="flex flex-col gap-6">
        <h1 className="text-2xl font-bold">Settings</h1>
        <p className="text-sm text-muted-foreground">Loading project...</p>
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-6">
      <h1 className="text-2xl font-bold">Project Settings</h1>

      <Card>
        <CardHeader>
          <CardTitle>General</CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="flex flex-col gap-2">
            <Label>Project Name</Label>
            <Input value={name} onChange={(e) => setName(e.target.value)} />
          </div>
          <div className="flex flex-col gap-2">
            <Label>Description</Label>
            <Input
              value={description}
              onChange={(e) => setDescription(e.target.value)}
            />
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Repository</CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="flex flex-col gap-2">
            <Label>Repository URL</Label>
            <Input
              value={repoUrl}
              placeholder="https://github.com/org/repo"
              onChange={(e) => setRepoUrl(e.target.value)}
            />
          </div>
          <div className="flex flex-col gap-2">
            <Label>Default Branch</Label>
            <Input
              value={defaultBranch}
              onChange={(e) => setDefaultBranch(e.target.value)}
            />
          </div>
        </CardContent>
      </Card>

      <Separator />

      <div className="flex items-center gap-3">
        <Button type="button" onClick={() => void handleSave()}>
          Save Settings
        </Button>
        {saved && (
          <span className="text-sm text-emerald-600 dark:text-emerald-400">
            Settings saved
          </span>
        )}
      </div>
    </div>
  );
}
