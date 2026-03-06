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
          className="fixed inset-0 bg-black/50 z-40 w-full h-full border-0 cursor-default"
          onClick={onClose}
          aria-label="Close file tree"
        />
      )}
      <div
        ref={fileTreeRef}
        className={`${isMobile
          ? "fixed left-0 top-0 bottom-0 z-50 w-72"
          : "relative flex-shrink-0 border-r border-gray-200 dark:border-gray-700"
          } bg-white dark:bg-gray-900 flex flex-col overflow-hidden`}
        style={!isMobile ? { width: fileTreeWidth } : {}}
      >
        <div className="flex items-center justify-between px-3 py-2 border-b border-gray-100 dark:border-gray-800 bg-gray-50/50 dark:bg-gray-800/50 flex-shrink-0 h-10">
          <div className="flex items-center gap-2 overflow-hidden">
            <ListTree className="w-4 h-4 text-gray-400 flex-shrink-0" />
            <span className="text-xs font-semibold text-gray-500 dark:text-gray-400 uppercase tracking-wider truncate">
              Files
            </span>
          </div>
          {isMobile && (
            <button
              onClick={onClose}
              className="p-1 hover:bg-gray-200 dark:hover:bg-gray-700 rounded transition-colors"
            >
              <X className="w-4 h-4 text-gray-500" />
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
            className="absolute top-0 right-0 w-1 h-full cursor-col-resize hover:bg-blue-500/30 transition-colors z-10"
            onMouseDown={startResizing}
          />
        )}
      </div>
    </>
  );
}
