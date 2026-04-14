"use client";

import { useEffect, useState } from "react";
import { useTranslations } from "next-intl";
import { formatDistanceToNow } from "date-fns";
import {
  FileText,
  FileType2,
  Sheet,
  Presentation,
  Trash2,
  Loader2,
  FileStack,
} from "lucide-react";
import { toast } from "sonner";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { ErrorBanner } from "@/components/shared/error-banner";
import { EmptyState } from "@/components/shared/empty-state";
import { DocumentUploadZone } from "./document-upload-zone";
import { useDocumentStore, type ProjectDocument } from "@/lib/stores/document-store";

interface DocumentPanelProps {
  projectId: string;
}

function formatBytes(bytes: number) {
  if (!bytes) return "0 B";
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

const fileTypeIcons: Record<string, typeof FileText> = {
  pdf: FileType2,
  docx: FileText,
  xlsx: Sheet,
  pptx: Presentation,
};

function getFileTypeIcon(fileType: string) {
  return fileTypeIcons[fileType.toLowerCase()] ?? FileText;
}

function StatusBadge({
  status,
  t,
}: {
  status: ProjectDocument["status"];
  t: (key: string) => string;
}) {
  switch (status) {
    case "pending":
      return <Badge variant="outline">{t("statusPending")}</Badge>;
    case "processing":
      return (
        <Badge variant="secondary" className="gap-1">
          <Loader2 className="size-3 animate-spin" />
          {t("statusProcessing")}
        </Badge>
      );
    case "ready":
      return (
        <Badge className="bg-green-500/15 text-green-700 hover:bg-green-500/25 dark:text-green-300">
          {t("statusReady")}
        </Badge>
      );
    case "failed":
      return <Badge variant="destructive">{t("statusFailed")}</Badge>;
    default:
      return <Badge variant="outline">{status}</Badge>;
  }
}

export function DocumentPanel({ projectId }: DocumentPanelProps) {
  const t = useTranslations("documents");
  const {
    documents,
    loading,
    uploading,
    error,
    loadDocuments,
    uploadDocument,
    deleteDocument,
    clearError,
  } = useDocumentStore();

  const [deleteTarget, setDeleteTarget] = useState<ProjectDocument | null>(
    null,
  );

  useEffect(() => {
    loadDocuments(projectId);
  }, [projectId, loadDocuments]);

  const handleUpload = async (file: File) => {
    try {
      await uploadDocument(projectId, file);
      toast.success(t("uploadSuccess"));
    } catch {
      toast.error(t("uploadError"));
    }
  };

  const handleDelete = async () => {
    if (!deleteTarget) return;
    try {
      await deleteDocument(projectId, deleteTarget.id);
      toast.success(t("deleteSuccess"));
    } catch {
      toast.error(t("deleteError"));
    } finally {
      setDeleteTarget(null);
    }
  };

  return (
    <div className="flex flex-col gap-6">
      <DocumentUploadZone onUpload={handleUpload} uploading={uploading} />

      {error && (
        <ErrorBanner
          message={error}
          onRetry={() => {
            clearError();
            loadDocuments(projectId);
          }}
        />
      )}

      <Card>
        <CardHeader>
          <CardTitle>{t("listTitle")}</CardTitle>
          {documents.length > 0 && (
            <CardDescription>
              {t("listDescription", { count: documents.length })}
            </CardDescription>
          )}
        </CardHeader>
        <CardContent>
          {loading ? (
            <div className="flex items-center justify-center py-10 text-sm text-muted-foreground">
              <Loader2 className="mr-2 size-4 animate-spin" />
              {t("loading")}
            </div>
          ) : documents.length === 0 ? (
            <EmptyState
              icon={FileStack}
              title={t("noDocuments")}
              description={t("noDocumentsHint")}
            />
          ) : (
            <div className="overflow-x-auto">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>{t("fileName")}</TableHead>
                    <TableHead>{t("fileType")}</TableHead>
                    <TableHead>{t("fileSize")}</TableHead>
                    <TableHead>{t("status")}</TableHead>
                    <TableHead>{t("uploadedAt")}</TableHead>
                    <TableHead className="w-[60px]">{t("actions")}</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {documents.map((doc) => {
                    const Icon = getFileTypeIcon(doc.fileType);
                    return (
                      <TableRow key={doc.id}>
                        <TableCell>
                          <div className="flex items-center gap-2">
                            <Icon className="size-4 shrink-0 text-muted-foreground" />
                            <span className="truncate font-medium">
                              {doc.fileName}
                            </span>
                          </div>
                        </TableCell>
                        <TableCell>
                          <Badge variant="outline" className="uppercase">
                            {doc.fileType}
                          </Badge>
                        </TableCell>
                        <TableCell className="text-muted-foreground">
                          {formatBytes(doc.fileSize)}
                        </TableCell>
                        <TableCell>
                          <StatusBadge status={doc.status} t={t} />
                        </TableCell>
                        <TableCell className="text-muted-foreground">
                          {formatDistanceToNow(new Date(doc.uploadedAt), {
                            addSuffix: true,
                          })}
                        </TableCell>
                        <TableCell>
                          <Button
                            variant="ghost"
                            size="icon"
                            className="size-8 text-muted-foreground hover:text-destructive"
                            onClick={() => setDeleteTarget(doc)}
                          >
                            <Trash2 className="size-4" />
                          </Button>
                        </TableCell>
                      </TableRow>
                    );
                  })}
                </TableBody>
              </Table>
            </div>
          )}
        </CardContent>
      </Card>

      <AlertDialog
        open={!!deleteTarget}
        onOpenChange={(open) => !open && setDeleteTarget(null)}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t("deleteConfirmTitle")}</AlertDialogTitle>
            <AlertDialogDescription>
              {t("deleteConfirmDescription", {
                name: deleteTarget?.fileName ?? "",
              })}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t("cancelButton")}</AlertDialogCancel>
            <AlertDialogAction
              onClick={handleDelete}
              className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
            >
              {t("deleteButton")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}
