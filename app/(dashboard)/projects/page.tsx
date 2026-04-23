"use client";

import { useEffect, useMemo, useState } from "react";
import { useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import { Plus, FolderKanban, Search, ListChecks, Archive } from "lucide-react";
import { Card, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Skeleton } from "@/components/ui/skeleton";
import { EmptyState } from "@/components/shared/empty-state";
import { MetricCard } from "@/components/shared/metric-card";
import { FilterBar } from "@/components/shared/filter-bar";
import { ListLayout } from "@/components/layout/templates/list-layout";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import {
  Tabs,
  TabsList,
  TabsTrigger,
} from "@/components/ui/tabs";
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
  const router = useRouter();
  const createProject = useProjectStore((s) => s.createProject);
  const [open, setOpen] = useState(false);
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    const createdProject = await createProject({ name, description });
    setName("");
    setDescription("");
    setOpen(false);
    if (createdProject?.id) {
      router.replace(`/?project=${createdProject.id}`);
    }
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
  const tc = useTranslations("common");
  useBreadcrumbs([{ label: tc("nav.group.workspace"), href: "/" }, { label: tc("nav.projects") }]);
  const t = useTranslations("projects");
  const {
    projects,
    fetchProjects,
    updateProject,
    deleteProject,
    archiveProject,
    unarchiveProject,
    loading,
  } = useProjectStore();
  const [searchQuery, setSearchQuery] = useState("");
  const [editingProject, setEditingProject] = useState<Project | null>(null);
  const [viewMode, setViewMode] = useState<"active" | "archived">("active");

  useEffect(() => {
    fetchProjects({ includeArchived: viewMode === "archived" });
  }, [fetchProjects, viewMode]);

  const viewFilteredProjects = useMemo(() => {
    if (viewMode === "archived") {
      return projects.filter((p) => p.status === "archived");
    }
    return projects.filter((p) => p.status !== "archived");
  }, [projects, viewMode]);

  const filteredProjects = useMemo(
    () =>
      viewFilteredProjects.filter((p) =>
        p.name.toLowerCase().includes(searchQuery.toLowerCase())
      ),
    [viewFilteredProjects, searchQuery]
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

  const handleArchive = async (id: string) => {
    await archiveProject(id);
  };

  const handleUnarchive = async (id: string) => {
    await unarchiveProject(id);
  };

  const toolbar = (
    <div className="flex flex-col gap-[var(--space-stack-sm)]">
      <Tabs
        value={viewMode}
        onValueChange={(v) => setViewMode(v === "archived" ? "archived" : "active")}
      >
        <TabsList>
          <TabsTrigger value="active">{t("tabs.active")}</TabsTrigger>
          <TabsTrigger value="archived">
            <Archive className="mr-1 size-3" />
            {t("tabs.archived")}
          </TabsTrigger>
        </TabsList>
      </Tabs>
      <FilterBar
        searchValue={searchQuery}
        searchPlaceholder={t("search.placeholder")}
        onSearch={setSearchQuery}
        onReset={() => setSearchQuery("")}
      />
    </div>
  );

  return (
    <ListLayout
      title={t("title")}
      actions={<CreateProjectDialog />}
      toolbar={toolbar}
    >
      <div className="flex flex-col gap-[var(--space-section-gap)]">
        {/* Stats */}
        {!loading && projects.length > 0 && (
          <div className="grid grid-cols-1 gap-[var(--space-grid-gap)] sm:grid-cols-2 md:grid-cols-3 lg:grid-cols-3 xl:grid-cols-3">
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
          <div className="grid grid-cols-1 gap-[var(--space-grid-gap)] sm:grid-cols-2 md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-3">
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
          <div className="grid grid-cols-1 gap-[var(--space-grid-gap)] sm:grid-cols-2 md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-3">
            {filteredProjects.map((p) => (
              <ProjectCard
                key={p.id}
                project={p}
                onEdit={setEditingProject}
                onDelete={deleteProject}
                onArchive={handleArchive}
                onUnarchive={handleUnarchive}
              />
            ))}
          </div>
        )}
      </div>

      {/* Edit dialog */}
      {editingProject && (
        <EditProjectDialog
          open={!!editingProject}
          project={editingProject}
          onSave={handleSaveEdit}
          onClose={() => setEditingProject(null)}
        />
      )}
    </ListLayout>
  );
}
