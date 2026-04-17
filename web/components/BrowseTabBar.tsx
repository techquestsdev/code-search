"use client";

import React, { useRef } from "react";
import { X, Columns, Rows } from "lucide-react";
import { getFileIcon } from "./LazyFileTree";

export interface BrowseTab {
  id: string;
  repoId: number;
  repoName: string;
  filePath: string;
  ref?: string;
  language?: string;
}

interface BrowseTabBarProps {
  tabs: BrowseTab[];
  activeTabId: string | null;
  onTabSelect: (tab: BrowseTab) => void;
  onTabClose: (tabId: string) => void;
  onSplitVertical?: () => void;
  onSplitHorizontal?: () => void;
  showSplitButtons?: boolean;
}

export function BrowseTabBar({
  tabs,
  activeTabId,
  onTabSelect,
  onTabClose,
  onSplitVertical,
  onSplitHorizontal,
  showSplitButtons = false,
}: BrowseTabBarProps) {
  const tabsRef = useRef<HTMLDivElement>(null);

  if (tabs.length === 0) {
    return null;
  }

  const getFileName = (filePath: string) => {
    return filePath.split("/").pop() || filePath;
  };

  const handleMiddleClick = (e: React.MouseEvent, tabId: string) => {
    if (e.button === 1) {
      e.preventDefault();
      onTabClose(tabId);
    }
  };

  return (
    <div className="flex min-h-[36px] items-center border-b border-gray-200 bg-gray-100 dark:border-gray-700 dark:bg-gray-800">
      {/* Tab list */}
      <div
        ref={tabsRef}
        className="scrollbar-thin scrollbar-thumb-gray-300 dark:scrollbar-thumb-gray-600 flex flex-1 items-center overflow-x-auto"
      >
        {tabs.map((tab) => {
          const isActive = tab.id === activeTabId;
          const fileName = getFileName(tab.filePath);
          const fileIcon = getFileIcon(tab.language, fileName);

          return (
            <button
              key={tab.id}
              onClick={() => onTabSelect(tab)}
              onMouseDown={(e) => handleMiddleClick(e, tab.id)}
              onKeyDown={(e) => {
                if (e.key === "Enter" || e.key === " ") {
                  onTabSelect(tab);
                }
              }}
              className={`group flex min-w-0 max-w-[200px] cursor-pointer items-center gap-1.5 border-r border-gray-200 px-3 py-1.5 text-left transition-colors dark:border-gray-700 ${
                isActive
                  ? "bg-white text-gray-900 dark:bg-gray-900 dark:text-white"
                  : "text-gray-600 hover:bg-gray-50 dark:text-gray-400 dark:hover:bg-gray-700/50"
              } `}
              title={`${tab.repoName}/${tab.filePath}`}
            >
              <span className="flex-shrink-0">{fileIcon}</span>
              <span className="truncate text-xs">{fileName}</span>
              <span
                role="button"
                tabIndex={-1}
                onClick={(e) => {
                  e.stopPropagation();
                  onTabClose(tab.id);
                }}
                onKeyDown={(e) => {
                  if (e.key === "Enter" || e.key === " ") {
                    e.stopPropagation();
                    onTabClose(tab.id);
                  }
                }}
                aria-label={`Close ${fileName}`}
                className={`ml-1 flex-shrink-0 cursor-pointer rounded p-0.5 ${
                  isActive
                    ? "opacity-60 hover:bg-gray-200 hover:opacity-100 dark:hover:bg-gray-700"
                    : "opacity-0 hover:bg-gray-200 hover:opacity-100 group-hover:opacity-60 dark:hover:bg-gray-600"
                } `}
              >
                <X className="h-3 w-3" />
              </span>
            </button>
          );
        })}
      </div>

      {/* Split buttons */}
      {showSplitButtons && tabs.length > 0 && (
        <div className="flex items-center gap-0.5 border-l border-gray-200 px-2 dark:border-gray-700">
          {onSplitVertical && (
            <button
              onClick={onSplitVertical}
              className="rounded p-1 text-gray-400 hover:bg-gray-200 hover:text-gray-600 dark:hover:bg-gray-700 dark:hover:text-gray-300"
              title="Split Right (⌘\)"
            >
              <Columns className="h-4 w-4" />
            </button>
          )}
          {onSplitHorizontal && (
            <button
              onClick={onSplitHorizontal}
              className="rounded p-1 text-gray-400 hover:bg-gray-200 hover:text-gray-600 dark:hover:bg-gray-700 dark:hover:text-gray-300"
              title="Split Down (⌘⇧\)"
            >
              <Rows className="h-4 w-4" />
            </button>
          )}
        </div>
      )}
    </div>
  );
}
