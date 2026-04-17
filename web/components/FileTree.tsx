"use client";

import { useMemo, useCallback } from "react";
import { Tree, NodeRendererProps, NodeApi } from "react-arborist";
import {
  Folder,
  FolderOpen,
  FileCode,
  FileText,
  FileJson,
  File,
  ChevronRight,
  ChevronDown,
} from "lucide-react";

export interface TreeEntry {
  name: string;
  type: "file" | "dir";
  path: string;
  size?: number;
  language?: string;
}

interface FileTreeNode {
  id: string;
  name: string;
  isFolder: boolean;
  path: string;
  language?: string;
  children?: FileTreeNode[];
}

interface FileTreeProps {
  entries: TreeEntry[];
  currentPath?: string;
  onSelect: (path: string, isFolder: boolean) => void;
  className?: string;
  height?: number;
}

// Convert flat entries to tree structure
function buildTreeFromEntries(entries: TreeEntry[]): FileTreeNode[] {
  return entries.map((entry) => ({
    id: entry.path || entry.name,
    name: entry.name,
    isFolder: entry.type === "dir",
    path: entry.path,
    language: entry.language,
    // Children are loaded dynamically when folder is expanded
    children: entry.type === "dir" ? [] : undefined,
  }));
}

// Get icon for file based on language/extension
function getFileIcon(language?: string, name?: string) {
  const ext = name?.split(".").pop()?.toLowerCase();

  // Language-based icons
  if (language) {
    const langLower = language.toLowerCase();
    if (["javascript", "typescript", "jsx", "tsx"].includes(langLower)) {
      return <FileCode className="h-4 w-4 text-yellow-500" />;
    }
    if (["python"].includes(langLower)) {
      return <FileCode className="h-4 w-4 text-blue-500" />;
    }
    if (["go"].includes(langLower)) {
      return <FileCode className="h-4 w-4 text-cyan-500" />;
    }
    if (["java", "kotlin"].includes(langLower)) {
      return <FileCode className="h-4 w-4 text-orange-500" />;
    }
    if (["rust"].includes(langLower)) {
      return <FileCode className="h-4 w-4 text-orange-700" />;
    }
    if (["json"].includes(langLower)) {
      return <FileJson className="h-4 w-4 text-yellow-600" />;
    }
    if (["markdown", "md"].includes(langLower)) {
      return <FileText className="h-4 w-4 text-gray-500" />;
    }
  }

  // Extension-based fallback
  if (ext) {
    if (["js", "jsx", "ts", "tsx", "mjs", "cjs"].includes(ext)) {
      return <FileCode className="h-4 w-4 text-yellow-500" />;
    }
    if (["py"].includes(ext)) {
      return <FileCode className="h-4 w-4 text-blue-500" />;
    }
    if (["go"].includes(ext)) {
      return <FileCode className="h-4 w-4 text-cyan-500" />;
    }
    if (["java", "kt", "kts"].includes(ext)) {
      return <FileCode className="h-4 w-4 text-orange-500" />;
    }
    if (["rs"].includes(ext)) {
      return <FileCode className="h-4 w-4 text-orange-700" />;
    }
    if (["json"].includes(ext)) {
      return <FileJson className="h-4 w-4 text-yellow-600" />;
    }
    if (["md", "mdx"].includes(ext)) {
      return <FileText className="h-4 w-4 text-gray-500" />;
    }
    if (["yaml", "yml"].includes(ext)) {
      return <FileText className="h-4 w-4 text-red-400" />;
    }
    if (["html", "htm"].includes(ext)) {
      return <FileCode className="h-4 w-4 text-orange-500" />;
    }
    if (["css", "scss", "sass", "less"].includes(ext)) {
      return <FileCode className="h-4 w-4 text-blue-400" />;
    }
    if (["sql"].includes(ext)) {
      return <FileCode className="h-4 w-4 text-blue-600" />;
    }
    if (["sh", "bash", "zsh"].includes(ext)) {
      return <FileCode className="h-4 w-4 text-green-600" />;
    }
  }

  return <File className="h-4 w-4 text-gray-400" />;
}

// Node renderer component
function Node({ node, style, dragHandle }: NodeRendererProps<FileTreeNode>) {
  const data = node.data;
  const isSelected = node.isSelected;
  const isFolder = data.isFolder;

  return (
    <div
      ref={dragHandle}
      style={style}
      role="button"
      tabIndex={0}
      className={`flex cursor-pointer select-none items-center gap-1 rounded px-2 py-1 outline-none hover:bg-gray-100 focus-visible:ring-1 focus-visible:ring-blue-500 dark:hover:bg-gray-700/50 ${isSelected ? "bg-blue-100 dark:bg-blue-900/30" : ""} `}
      onClick={() => {
        if (isFolder) {
          node.toggle();
        } else {
          node.select();
        }
      }}
      onKeyDown={(e) => {
        if (e.key === "Enter" || e.key === " ") {
          if (isFolder) node.toggle();
          else node.select();
        }
      }}
    >
      {/* Expand/collapse arrow for folders */}
      <span className="flex h-4 w-4 flex-shrink-0 items-center justify-center">
        {isFolder &&
          (node.isOpen ? (
            <ChevronDown className="h-3 w-3 text-gray-400" />
          ) : (
            <ChevronRight className="h-3 w-3 text-gray-400" />
          ))}
      </span>

      {/* Icon */}
      <span className="flex-shrink-0">
        {isFolder ? (
          node.isOpen ? (
            <FolderOpen className="h-4 w-4 text-blue-500" />
          ) : (
            <Folder className="h-4 w-4 text-blue-500" />
          )
        ) : (
          getFileIcon(data.language, data.name)
        )}
      </span>

      {/* Name */}
      <span className="truncate text-sm text-gray-700 dark:text-gray-300">
        {data.name}
      </span>
    </div>
  );
}

function FileTree({
  entries,
  currentPath,
  onSelect,
  className = "",
  height = 400,
}: FileTreeProps) {
  // Build tree data from entries
  const treeData = useMemo(() => buildTreeFromEntries(entries), [entries]);

  // Handle selection
  const handleSelect = useCallback(
    (nodes: NodeApi<FileTreeNode>[]) => {
      if (nodes.length > 0) {
        const node = nodes[0];
        onSelect(node.data.path, node.data.isFolder);
      }
    },
    [onSelect]
  );

  // Find initially selected node based on currentPath
  const selection = useMemo(() => {
    if (!currentPath) return undefined;
    return currentPath;
  }, [currentPath]);

  if (entries.length === 0) {
    return (
      <div
        className={`flex h-32 items-center justify-center text-gray-400 dark:text-gray-500 ${className}`}
      >
        Empty directory
      </div>
    );
  }

  return (
    <div className={`file-tree ${className}`}>
      <Tree<FileTreeNode>
        data={treeData}
        width="100%"
        height={height}
        indent={16}
        rowHeight={28}
        selection={selection}
        onSelect={handleSelect}
        disableDrag
        disableDrop
      >
        {Node}
      </Tree>
    </div>
  );
}

export default FileTree;
