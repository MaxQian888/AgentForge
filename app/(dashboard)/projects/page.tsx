"use client";

import { useEffect, useMemo, useState } from "react";
import { useTranslations } from "next-intl";
import { Plus, FolderKanban, Search, ListChecks } from "lucide-react";
import { Card, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Skeleton } from "@/components/ui/skeleton";
import { PageHeader } from "@/components/shared/page-header";
import { EmptyState } from "@/components/shared/empty-state";
import { MetricCard } from "@/components/shared/metric-card";
import { FilterBar } from "@/components/shared/filter-bar";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import { ProjectCard } from "@/components/project/project-card";
import { EditProjectDialog } from "@/components/project/edit-project-dialog";
import {
  useProjectStore,
  type Project,
  type ProjectUpdateInput,
} from "@/lib/stores/project-store";
import { useBreadcrumbs } from "@/hooks/use-breadcrumbs";

function CreateProjectDialog() {
  const t = useTranslations("projects");
  const createProject = useProjectStore((s) => s.createProject);
  const [open, setOpen] = useState(false);
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    await createProject({ name, description });
    setName("");
    setDescription("");
    setOpen(false);
  };

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger asChild>
        <Button size="sm">
          <Plus className="mr-1 size-4" />
          {t("newProject")}
        </Button>
      </DialogTrigger>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t("createProject.title")}</DialogTitle>
          <DialogDescription>
            {t("createProject.description")}
          </DialogDescription>
        </DialogHeader>
        <form onSubmit={handleSubmit} className="flex flex-col gap-4">
          <div className="flex flex-col gap-2">
            <Label htmlFor="proj-name">{t("createProject.nameLabel")}</Label>
            <Input
              id="proj-name"
              value={name}
              onChange={(e) => setName(e.target.value)}
              required
            />
          </div>
          <div className="flex flex-col gap-2">
            <Label htmlFor="proj-desc">
              {t("createProject.descriptionLabel")}
            </Label>
            <Input
              id="proj-desc"
              value={description}
              onChange={(e) => setDescription(e.target.value)}
            />
          </div>
          <Button type="submit">{t("createProject.submit")}</Button>
        </form>
      </DialogContent>
    </Dialog>
  );
}

function ProjectCardSkeleton() {
  return (
    <Card>
      <CardContent className="flex flex-col gap-3 py-4">
        <div className="flex items-center justify-between">
          <Skeleton className="h-5 w-32" />
          <Skeleton className="h-5 w-16 rounded-full" />
        </div>
        <Skeleton className="h-4 w-full" />
        <Skeleton className="h-4 w-40" />
        <div className="flex gap-2">
          <Skeleton className="h-5 w-16 rounded-full" />
          <Skeleton className="h-5 w-16 rounded-full" />
        </div>
      </CardContent>
    </Card>
  );
}

export default function ProjectsPage() {
  useBreadcrumbs([{ label: "Workspace", href: "/" }, { label: "Projects" }]);
  const t = useTranslations("projects");
  const { projects, fetchProjects, updateProject, deleteProject, loading } =
    useProjectStore();
  const [searchQuery, setSearchQuery] = useState("");
  const [editingProject, setEditingProject] = useState<Project | null>(null);

  useEffect(() => {
    fetchProjects();
  }, [fetchProjects]);

  const filteredProjects = useMemo(
    () =>
      projects.filter((p) =>
        p.name.toLowerCase().includes(searchQuery.toLowerCase())
      ),
    [projects, searchQuery]
  );

  const activeCount = useMemo(
    () => projects.filter((p) => p.status === "active").length,
    [projects]
  );

  const totalTasks = useMemo(
    () => projects.reduce((sum, p) => sum + p.taskCount, 0),
    [projects]
  );

  const handleSaveEdit = async (id: string, input: ProjectUpdateInput) => {
    await updateProject(id, input);
  };

  return (
    <div className="flex flex-col gap-6">
      {/* Header */}
      <PageHeader
        title={t("title")}
        actions={<CreateProjectDialog />}
      />

      {/* Search */}
      <FilterBar
        searchValue={searchQuery}
        searchPlaceholder={t("search.placeholder")}
        onSearch={setSearchQuery}
        onReset={() => setSearchQuery("")}
      />

      {/* Stats */}
      {!loading && projects.length > 0 && (
        <div className="grid gap-4 sm:grid-cols-3">
          <MetricCard
            label={t("stats.total")}
            value={projects.length}
            icon={FolderKanban}
          />
          <MetricCard
            label={t("stats.active")}
            value={activeCount}
          />
          <MetricCard
            label={t("stats.totalTasks")}
            value={totalTasks}
            icon={ListChecks}
          />
        </div>
      )}

      {/* Content */}
      {loading ? (
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {Array.from({ length: 3 }).map((_, i) => (
            <ProjectCardSkeleton key={i} />
          ))}
        </div>
      ) : projects.length === 0 ? (
        <EmptyState
          icon={FolderKanban}
          title={t("empty.icon")}
        />
      ) : filteredProjects.length === 0 ? (
        <EmptyState
          icon={Search}
          title={t("empty.noSearchResults")}
        />
      ) : (
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {filteredProjects.map((p) => (
            <ProjectCard
              key={p.id}
              project={p}
              onEdit={setEditingProject}
              onDelete={deleteProject}
            />
          ))}
        </div>
      )}

      {/* Edit dialog */}
      {editingProject && (
        <EditProjectDialog
          open={!!editingProject}
          project={editingProject}
          onSave={handleSaveEdit}
          onClose={() => setEditingProject(null)}
        />
      )}
    </div>
  );
}
