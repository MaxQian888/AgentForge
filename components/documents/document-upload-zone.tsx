"use client";

import { useCallback, useRef, useState } from "react";
import { useTranslations } from "next-intl";
import { Upload } from "lucide-react";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";

const ALLOWED_EXTENSIONS = [".pdf", ".docx", ".xlsx", ".pptx"];
const MAX_FILE_SIZE = 50 * 1024 * 1024; // 50 MB

interface DocumentUploadZoneProps {
  onUpload: (file: File) => void;
  uploading: boolean;
  className?: string;
}

function validateFile(
  file: File,
  t: (key: string) => string,
): string | null {
  const ext = file.name.slice(file.name.lastIndexOf(".")).toLowerCase();
  if (!ALLOWED_EXTENSIONS.includes(ext)) {
    return t("invalidFileType");
  }
  if (file.size > MAX_FILE_SIZE) {
    return t("fileTooLarge");
  }
  return null;
}

export function DocumentUploadZone({
  onUpload,
  uploading,
  className,
}: DocumentUploadZoneProps) {
  const t = useTranslations("documents");
  const inputRef = useRef<HTMLInputElement>(null);
  const [dragOver, setDragOver] = useState(false);
  const [validationError, setValidationError] = useState<string | null>(null);

  const handleFile = useCallback(
    (file: File) => {
      setValidationError(null);
      const error = validateFile(file, t);
      if (error) {
        setValidationError(error);
        return;
      }
      onUpload(file);
    },
    [onUpload, t],
  );

  const handleDrop = useCallback(
    (e: React.DragEvent) => {
      e.preventDefault();
      setDragOver(false);
      const file = e.dataTransfer.files[0];
      if (file) handleFile(file);
    },
    [handleFile],
  );

  const handleDragOver = useCallback((e: React.DragEvent) => {
    e.preventDefault();
    setDragOver(true);
  }, []);

  const handleDragLeave = useCallback((e: React.DragEvent) => {
    e.preventDefault();
    setDragOver(false);
  }, []);

  const handleInputChange = useCallback(
    (e: React.ChangeEvent<HTMLInputElement>) => {
      const file = e.target.files?.[0];
      if (file) handleFile(file);
      // Reset input so the same file can be re-selected
      if (inputRef.current) inputRef.current.value = "";
    },
    [handleFile],
  );

  return (
    <div
      className={cn(
        "flex flex-col items-center gap-3 rounded-lg border-2 border-dashed px-6 py-10 text-center transition-colors",
        dragOver
          ? "border-primary bg-primary/5"
          : "border-muted-foreground/25",
        uploading && "pointer-events-none opacity-60",
        className,
      )}
      onDrop={handleDrop}
      onDragOver={handleDragOver}
      onDragLeave={handleDragLeave}
    >
      <div className="rounded-full bg-muted p-3">
        <Upload className="size-6 text-muted-foreground" />
      </div>
      <div className="space-y-1">
        <p className="text-sm font-medium">
          {uploading ? t("uploading") : t("uploadDescription")}
        </p>
        <p className="text-xs text-muted-foreground">{t("uploadHint")}</p>
      </div>
      {validationError && (
        <p className="text-xs text-destructive">{validationError}</p>
      )}
      <Button
        variant="outline"
        size="sm"
        disabled={uploading}
        onClick={() => inputRef.current?.click()}
      >
        {t("uploadButton")}
      </Button>
      <input
        ref={inputRef}
        type="file"
        className="hidden"
        accept=".pdf,.docx,.xlsx,.pptx"
        onChange={handleInputChange}
      />
    </div>
  );
}
