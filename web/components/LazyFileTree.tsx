"use client";

import React, { useEffect, useCallback, useRef, useReducer } from "react";
import {
  Folder,
  FolderOpen,
  FileCode,
  FileText,
  FileJson,
  File,
  ChevronRight,
  ChevronDown,
  Loader2,
  RefreshCw,
} from "lucide-react";
import { api, TreeEntry } from "@/lib/api";

interface LazyFileTreeProps {
  repoId: number;
  currentPath?: string;
  currentRef?: string;
  onSelect: (path: string, isFolder: boolean) => void;
  className?: string;
  splitEnabled?: boolean;
}

interface TreeNode {
  name: string;
  path: string;
  type: "file" | "dir";
  language?: string;
  children?: TreeNode[];
  isLoading?: boolean;
  isLoaded?: boolean;
  isExpanded?: boolean;
}

// Get icon for file based on language/extension
export function getFileIcon(language?: string, name?: string) {
  const ext = name?.split(".").pop()?.toLowerCase();

  // Language-based icons
  if (language) {
    const langLower = language.toLowerCase();
    if (["javascript", "typescript", "jsx", "tsx"].includes(langLower)) {
      return <FileCode className="w-4 h-4 text-yellow-500" />;
    }
    if (["python"].includes(langLower)) {
      return <FileCode className="w-4 h-4 text-blue-500" />;
    }
    if (["go"].includes(langLower)) {
      return <FileCode className="w-4 h-4 text-cyan-500" />;
    }
    if (["java", "kotlin"].includes(langLower)) {
      return <FileCode className="w-4 h-4 text-orange-500" />;
    }
    if (["rust"].includes(langLower)) {
      return <FileCode className="w-4 h-4 text-orange-700" />;
    }
    if (["json"].includes(langLower)) {
      return <FileJson className="w-4 h-4 text-yellow-600" />;
    }
    if (["markdown", "md"].includes(langLower)) {
      return <FileText className="w-4 h-4 text-gray-500" />;
    }
  }

  // Extension-based fallback
  if (ext) {
    if (["js", "jsx", "ts", "tsx", "mjs", "cjs"].includes(ext)) {
      return <FileCode className="w-4 h-4 text-yellow-500" />;
    }
    if (["py"].includes(ext)) {
      return <FileCode className="w-4 h-4 text-blue-500" />;
    }
    if (["go"].includes(ext)) {
      return <FileCode className="w-4 h-4 text-cyan-500" />;
    }
    if (["java", "kt", "kts"].includes(ext)) {
      return <FileCode className="w-4 h-4 text-orange-500" />;
    }
    if (["rs"].includes(ext)) {
      return <FileCode className="w-4 h-4 text-orange-700" />;
    }
    if (["json"].includes(ext)) {
      return <FileJson className="w-4 h-4 text-yellow-600" />;
    }
    if (["md", "mdx"].includes(ext)) {
      return <FileText className="w-4 h-4 text-gray-500" />;
    }
    if (["yaml", "yml"].includes(ext)) {
      return <FileText className="w-4 h-4 text-red-400" />;
    }
    if (["html", "htm"].includes(ext)) {
      return <FileCode className="w-4 h-4 text-orange-500" />;
    }
    if (["css", "scss", "sass", "less"].includes(ext)) {
      return <FileCode className="w-4 h-4 text-blue-400" />;
    }
    if (["sql"].includes(ext)) {
      return <FileCode className="w-4 h-4 text-blue-600" />;
    }
    if (["sh", "bash", "zsh"].includes(ext)) {
      return <FileCode className="w-4 h-4 text-green-600" />;
    }
  }

  return <File className="w-4 h-4 text-gray-400" />;
}

// Convert API entries to tree nodes
function entriesToNodes(entries: TreeEntry[]): TreeNode[] {
  return entries
    .map((entry) => ({
      name: entry.name,
      path: entry.path,
      type: entry.type,
      language: entry.language,
      children: entry.type === "dir" ? [] : undefined,
      isLoaded: false,
      isExpanded: false,
    }))
    .sort((a, b) => {
      // Folders first, then alphabetically
      if (a.type === "dir" && b.type !== "dir") return -1;
      if (a.type !== "dir" && b.type === "dir") return 1;
      return a.name.localeCompare(b.name);
    });
}

// TreeNodeComponent
function TreeNodeComponent({
  node,
  depth,
  currentPath,
  onToggle,
  onSelect,
  splitEnabled,
}: {
  node: TreeNode;
  depth: number;
  currentPath?: string;
  onToggle: (path: string, isExpanded: boolean, isLoaded: boolean) => void;
  onSelect: (path: string, isFolder: boolean) => void;
  splitEnabled?: boolean;
}) {
  const isSelected = currentPath === node.path;
  const isFolder = node.type === "dir";
  const paddingLeft = 8 + depth * 16;

  const handleChevronClick = (e: React.MouseEvent) => {
    e.stopPropagation();
    if (isFolder) {
      onToggle(node.path, !!node.isExpanded, !!node.isLoaded);
    }
  };

  const handleRowClick = () => {
    if (isFolder) {
      if (splitEnabled) {
        // In split mode, just expand/collapse (VS Code style)
        onToggle(node.path, !!node.isExpanded, !!node.isLoaded);
      } else {
        // Without split, navigate to folder AND expand it
        onToggle(node.path, !!node.isExpanded, !!node.isLoaded);
        onSelect(node.path, true);
      }
    } else {
      // For files, select them
      onSelect(node.path, false);
    }
  };

  return (
    <>
      <button
        onClick={handleRowClick}
        onKeyDown={(e) => {
          if (e.key === 'Enter' || e.key === ' ') {
            handleRowClick();
          } else if (e.key === 'ArrowRight' && isFolder && !node.isExpanded) {
            onToggle(node.path, false, !!node.isLoaded);
          } else if (e.key === 'ArrowLeft' && isFolder && node.isExpanded) {
            onToggle(node.path, true, !!node.isLoaded);
          }
        }}
        className={`w-full flex items-center gap-1.5 py-1 pr-2 text-left hover:bg-gray-100 dark:hover:bg-gray-700/50 transition-colors cursor-pointer outline-none focus-visible:ring-2 focus-visible:ring-blue-500 z-0 ${isSelected ? "bg-blue-100 dark:bg-blue-900/30" : ""
          }`}
        style={{ paddingLeft }}
      >
        {/* Expand/collapse icon - separate interactive area */}
        {isFolder ? (
          <span
            role="button"
            tabIndex={-1}
            onClick={handleChevronClick}
            onKeyDown={(e) => {
              if (e.key === 'Enter' || e.key === ' ') {
                handleChevronClick(e as unknown as React.MouseEvent);
              }
            }}
            className="w-4 h-4 flex items-center justify-center flex-shrink-0 hover:bg-gray-200 dark:hover:bg-gray-600 rounded cursor-pointer outline-none focus-visible:ring-1 focus-visible:ring-blue-500"
            aria-label={node.isExpanded ? "Collapse folder" : "Expand folder"}
          >
            {node.isLoading ? (
              <Loader2 className="w-3 h-3 text-gray-400 animate-spin" />
            ) : node.isExpanded ? (
              <ChevronDown className="w-3 h-3 text-gray-400" />
            ) : (
              <ChevronRight className="w-3 h-3 text-gray-400" />
            )}
          </span>
        ) : (
          <div className="w-4 h-4 flex-shrink-0" />
        )}

        {/* Icon */}
        <span className="flex-shrink-0">
          {isFolder ? (
            node.isExpanded ? (
              <FolderOpen className="w-4 h-4 text-blue-500" />
            ) : (
              <Folder className="w-4 h-4 text-blue-500" />
            )
          ) : (
            getFileIcon(node.language, node.name)
          )}
        </span>

        {/* Name */}
        <span
          className={`truncate text-sm ${isSelected
            ? "text-blue-700 dark:text-blue-300 font-medium"
            : "text-gray-700 dark:text-gray-300"
            }`}
        >
          {node.name}
        </span>
      </button>

      {/* Children */}
      {isFolder && node.isExpanded && node.children && (
        <div>
          {node.children.map((child) => (
            <TreeNodeComponent
              key={child.path}
              node={child}
              depth={depth + 1}
              currentPath={currentPath}
              onToggle={onToggle}
              onSelect={onSelect}
              splitEnabled={splitEnabled}
            />
          ))}
        </div>
      )}
    </>
  );
}

export function LazyFileTree({
  repoId,
  currentPath,
  currentRef,
  onSelect,
  className = "",
  splitEnabled = false,
}: LazyFileTreeProps) {
  type TreeState = { rootNodes: TreeNode[]; loading: boolean; error: string | null };
  const [treeState, updateTree] = useReducer(
    (s: TreeState, u: Partial<TreeState> | ((prev: TreeState) => TreeState)) =>
      typeof u === 'function' ? u(s) : { ...s, ...u },
    { rootNodes: [] as TreeNode[], loading: true, error: null as string | null }
  );

  const { rootNodes, loading, error } = treeState;

  // Track repo to reset tree when it changes
  const lastRepoId = useRef(repoId);

  // Load root directory
  useEffect(() => {
    let cancelled = false;

    // Reset tree when repo changes and set loading
    const resetNeeded = lastRepoId.current !== repoId;
    if (resetNeeded) lastRepoId.current = repoId;
    updateTree(prev => ({
      ...prev,
      ...(resetNeeded ? { rootNodes: [] } : {}),
      loading: true,
      error: null,
    }));

    const loadRoot = async () => {
      try {
        const tree = await api.getTree(repoId, undefined, currentRef);
        if (!cancelled) {
          const nodes = entriesToNodes(tree.entries);
          updateTree(prev => ({ ...prev, rootNodes: nodes, loading: false }));
        }
      } catch (err) {
        if (!cancelled) {
          updateTree(prev => ({
            ...prev,
            error: err instanceof Error ? err.message : "Failed to load files",
            loading: false,
          }));
        }
      }
    };
    loadRoot();

    return () => { cancelled = true; };
  }, [repoId, currentRef]);

  // Check if a path is already expanded in the tree
  const isPathExpanded = useCallback((path: string, nodes: TreeNode[]): boolean => {
    for (const node of nodes) {
      if (node.path === path) {
        return !!node.isExpanded;
      }
      if (node.children) {
        const found = isPathExpanded(path, node.children);
        if (found) return true;
      }
    }
    return false;
  }, []);

  // Auto-expand to current path when it changes
  useEffect(() => {
    // Skip if no path, no nodes, or still loading
    if (!currentPath || rootNodes.length === 0 || loading) {
      return;
    }

    // Check if we need to expand - only expand paths that aren't already expanded
    const pathParts = currentPath.split("/");
    const pathsToExpand: string[] = [];
    let accPath = "";

    for (let i = 0; i < pathParts.length - 1; i++) {
      accPath = accPath ? `${accPath}/${pathParts[i]}` : pathParts[i];
      pathsToExpand.push(accPath);
    }

    // Find which paths need expanding (not already loaded/expanded)
    const needsExpanding = pathsToExpand.filter(p => {
      const findNode = (path: string, nodes: TreeNode[]): TreeNode | null => {
        for (const node of nodes) {
          if (node.path === path) return node;
          if (node.children) {
            const found = findNode(path, node.children);
            if (found) return found;
          }
        }
        return null;
      };
      const node = findNode(p, rootNodes);
      return !node?.isLoaded || !node?.isExpanded;
    });

    // If all paths are already expanded, nothing to do
    if (needsExpanding.length === 0) {
      return;
    }

    // Expand path asynchronously, preserving existing expanded folders
    const expandToPath = async () => {
      for (const pathToExpand of needsExpanding) {
        try {
          const tree = await api.getTree(repoId, pathToExpand, currentRef);
          const children = entriesToNodes(tree.entries);

          updateTree((prev) => {
            const updateInNodes = (nodes: TreeNode[]): TreeNode[] => {
              return nodes.map((node) => {
                if (node.path === pathToExpand) {
                  // Only update if not already loaded, preserve existing expanded state
                  if (!node.isLoaded) {
                    return { ...node, children, isLoaded: true, isExpanded: true };
                  }
                  // If loaded but not expanded, just expand it
                  return { ...node, isExpanded: true };
                }
                if (node.children) {
                  return { ...node, children: updateInNodes(node.children) };
                }
                return node;
              });
            };
            return { ...prev, rootNodes: updateInNodes(prev.rootNodes) };
          });
        } catch (err) {
          console.error("Failed to expand path:", pathToExpand, err);
          break;
        }
      }
    };

    expandToPath();
    // eslint-disable-next-line react-hooks/exhaustive-deps -- rootNodes excluded to prevent infinite loop
  }, [currentPath, rootNodes.length, loading, repoId, currentRef]);

  // Update a node in the tree
  const updateNode = useCallback(
    (
      path: string,
      updater: (node: TreeNode) => TreeNode,
      nodes: TreeNode[]
    ): TreeNode[] => {
      return nodes.map((node) => {
        if (node.path === path) {
          return updater(node);
        }
        if (node.children) {
          return {
            ...node,
            children: updateNode(path, updater, node.children),
          };
        }
        return node;
      });
    },
    []
  );

  // Load and expand a folder - called from TreeNodeItem click
  const loadAndExpandFolder = useCallback(
    async (path: string, isLoaded: boolean) => {
      // If already loaded, just expand
      if (isLoaded) {
        updateTree((prev) => ({
          ...prev,
          rootNodes: updateNode(path, (n) => ({ ...n, isExpanded: true }), prev.rootNodes)
        }));
        return;
      }

      // Set loading state
      updateTree((prev) => ({
        ...prev,
        rootNodes: updateNode(path, (n) => ({ ...n, isLoading: true }), prev.rootNodes)
      }));

      try {
        const tree = await api.getTree(repoId, path, currentRef);
        const children = entriesToNodes(tree.entries);

        updateTree((prev) => ({
          ...prev,
          rootNodes: updateNode(
            path,
            (n) => ({
              ...n,
              children,
              isLoaded: true,
              isLoading: false,
              isExpanded: true,
            }),
            prev.rootNodes
          )
        }));
      } catch (err) {
        console.error("Failed to load folder:", err);
        updateTree((prev) => ({
          ...prev,
          rootNodes: updateNode(path, (n) => ({ ...n, isLoading: false }), prev.rootNodes)
        }));
      }
    },
    [repoId, currentRef, updateNode]
  );

  // Toggle folder - called from TreeNodeItem
  const handleToggle = useCallback(
    (path: string, isExpanded: boolean, isLoaded: boolean) => {
      if (isExpanded) {
        // Collapse
        updateTree((prev) => ({
          ...prev,
          rootNodes: updateNode(path, (n) => ({ ...n, isExpanded: false }), prev.rootNodes)
        }));
      } else {
        // Expand (and load if needed)
        loadAndExpandFolder(path, isLoaded);
      }
    },
    [updateNode, loadAndExpandFolder]
  );

  // Refresh tree
  const handleRefresh = useCallback(async () => {
    updateTree(prev => ({ ...prev, loading: true, error: null }));

    try {
      const tree = await api.getTree(repoId, undefined, currentRef);
      const nodes = entriesToNodes(tree.entries);
      updateTree(prev => ({ ...prev, rootNodes: nodes, loading: false }));
    } catch (err) {
      updateTree(prev => ({ 
        ...prev, 
        error: err instanceof Error ? err.message : "Failed to load files",
        loading: false 
      }));
    }
  }, [repoId, currentRef]);

  if (loading && rootNodes.length === 0) {
    return (
      <div className={`flex items-center justify-center py-8 ${className}`}>
        <Loader2 className="w-5 h-5 animate-spin text-gray-400" />
      </div>
    );
  }

  if (error) {
    return (
      <div className={`p-4 ${className}`}>
        <div className="text-sm text-red-600 dark:text-red-400 mb-2">{error}</div>
        <button
          onClick={handleRefresh}
          className="flex items-center gap-1.5 text-xs text-blue-600 dark:text-blue-400 hover:underline"
        >
          <RefreshCw className="w-3 h-3" />
          Retry
        </button>
      </div>
    );
  }

  if (rootNodes.length === 0) {
    return (
      <div className={`p-4 text-sm text-gray-500 dark:text-gray-400 ${className}`}>
        Empty repository
      </div>
    );
  }

  return (
    <div className={`overflow-y-auto ${className}`}>
      {rootNodes.map((node) => (
        <TreeNodeComponent
          key={node.path}
          node={node}
          depth={0}
          currentPath={currentPath}
          onToggle={handleToggle}
          onSelect={onSelect}
          splitEnabled={splitEnabled}
        />
      ))}
    </div>
  );
}

export default LazyFileTree;
