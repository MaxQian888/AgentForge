"use client";

import dynamic from "next/dynamic";
import { EditorLoadingSkeleton } from "./editor-loading-skeleton";

const DynamicBlockEditor = dynamic(
  () => import("./block-editor-client").then((mod) => mod.BlockEditorClient),
  {
    ssr: false,
    loading: () => <EditorLoadingSkeleton />,
  }
);

export function BlockEditor(props: {
  value: string;
  editable?: boolean;
  commentedBlockIds?: string[];
  taskCountsByBlock?: Record<string, number>;
  onCreateTasksFromSelection?: (blockIds: string[]) => void;
  onChange?: (content: string, contentText: string) => void;
}) {
  return <DynamicBlockEditor {...props} />;
}
