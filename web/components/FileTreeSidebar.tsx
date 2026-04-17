"use client";

import React from "react";
import { ListTree, X } from "lucide-react";
import { LazyFileTree } from "./LazyFileTree";
import { Repository } from "@/lib/api";

interface FileTreeSidebarProps {
  repo: Repository | null;
  currentPath: string;
  refName: string;
  onSelect: (path: string, isFolder: boolean) => void;
  isMobile: boolean;
  fileTreeWidth: number;
  onClose: () => void;
  startResizing: () => void;
  fileTreeRef: React.RefObject<HTMLDivElement | null>;
}

export function FileTreeSidebar({
  repo,
  currentPath,
  refName,
  onSelect,
  isMobile,
  fileTreeWidth,
  onClose,
  startResizing,
  fileTreeRef,
}: FileTreeSidebarProps) {
  return (
    <>
      {/* Overlay for mobile */}
      {isMobile && (
        <button
          className="fixed inset-0 z-40 h-full w-full cursor-default border-0 bg-black/50"
          onClick={onClose}
          aria-label="Close file tree"
        />
      )}
      <div
        ref={fileTreeRef}
        className={`${
          isMobile
            ? "fixed bottom-0 left-0 top-0 z-50 w-72"
            : "relative flex-shrink-0 border-r border-gray-200 dark:border-gray-700"
        } flex flex-col overflow-hidden bg-white dark:bg-gray-900`}
        style={!isMobile ? { width: fileTreeWidth } : {}}
      >
        <div className="flex h-10 flex-shrink-0 items-center justify-between border-b border-gray-100 bg-gray-50/50 px-3 py-2 dark:border-gray-800 dark:bg-gray-800/50">
          <div className="flex items-center gap-2 overflow-hidden">
            <ListTree className="h-4 w-4 flex-shrink-0 text-gray-400" />
            <span className="truncate text-xs font-semibold uppercase tracking-wider text-gray-500 dark:text-gray-400">
              Files
            </span>
          </div>
          {isMobile && (
            <button
              onClick={onClose}
              className="rounded p-1 transition-colors hover:bg-gray-200 dark:hover:bg-gray-700"
            >
              <X className="h-4 w-4 text-gray-500" />
            </button>
          )}
        </div>

        <div className="flex-1 overflow-auto">
          {repo && (
            <LazyFileTree
              repoId={repo.id}
              currentRef={refName}
              currentPath={currentPath}
              onSelect={onSelect}
            />
          )}
        </div>

        {/* Resize handle */}
        {!isMobile && (
          <div
            className="absolute right-0 top-0 z-10 h-full w-1 cursor-col-resize transition-colors hover:bg-blue-500/30"
            onMouseDown={startResizing}
          />
        )}
      </div>
    </>
  );
}
