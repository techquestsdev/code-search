"use client";

import React from "react";
import { Loader2, X } from "lucide-react";
import dynamic from "next/dynamic";
import { BinaryFileViewer } from "./BinaryFileViewer";
import { BlobResponse, PaneTab, ActivePane } from "@/lib/api";
import { getFileIcon } from "./LazyFileTree";

const CodeViewer = dynamic(() => import("./CodeViewer"), {
  ssr: false,
  loading: () => (
    <div className="flex h-full flex-1 items-center justify-center">
      <Loader2 className="h-6 w-6 animate-spin text-gray-400" />
    </div>
  ),
});

interface FilePaneProps {
  pane: ActivePane;
  activeTabId: string | null;
  tabs: PaneTab[];
  blob: BlobResponse | null;
  loading: boolean;
  activePane: ActivePane;
  direction: "vertical" | "horizontal";
  ratio: number;
  scrollToLine?: { line: number; key: number } | null;
  onSelectTab: (pane: ActivePane, tabId: string) => void;
  onCloseTab: (pane: ActivePane, tabId: string) => void;
  onClosePane: (pane: ActivePane) => void;
  onSetActive: (pane: ActivePane) => void;
  onFindReferences: (
    word: string,
    line: number,
    col: number,
    repoId: number,
    path: string,
    lang?: string
  ) => void;
  onGoToDefinition: (
    word: string,
    line: number,
    col: number,
    repoId: number,
    path: string,
    lang?: string
  ) => void;
  onLineClick: (line: number, repoId: number, path: string) => void;
}

export function FilePane({
  pane,
  activeTabId,
  tabs,
  blob,
  loading,
  activePane,
  direction,
  ratio,
  scrollToLine,
  onSelectTab,
  onCloseTab,
  onClosePane,
  onSetActive,
  onFindReferences,
  onGoToDefinition,
  onLineClick,
}: FilePaneProps) {
  const isActive = activePane === pane;
  const activeTab = tabs.find((t) => t.id === activeTabId);

  return (
    <div
      role="button"
      aria-label={`${pane} editor pane`}
      tabIndex={0}
      onClick={() => onSetActive(pane)}
      onKeyDown={(e) => {
        if (e.key === "Enter" || e.key === " ") {
          onSetActive(pane);
        }
      }}
      className={`flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden text-left outline-none ${
        isActive ? "border-2 border-blue-400/40" : "border-2 border-transparent"
      }`}
      style={{
        [direction === "vertical" ? "width" : "height"]:
          pane === "primary" ? `${ratio * 100}%` : `${(1 - ratio) * 100}%`,
      }}
    >
      {/* Pane tabs */}
      {tabs.length > 0 && (
        <div
          className={`flex w-full items-center border-b border-gray-200 dark:border-gray-700 ${
            isActive
              ? "bg-blue-50/30 dark:bg-blue-900/5"
              : "bg-gray-50 dark:bg-gray-800/80"
          }`}
        >
          <div className="scrollbar-thin flex flex-1 items-center overflow-x-auto">
            {tabs.map((tab) => (
              <button
                key={tab.id}
                onClick={(e) => {
                  e.stopPropagation();
                  onSelectTab(pane, tab.id);
                }}
                onKeyDown={(e) => {
                  if (e.key === "Enter" || e.key === " ") {
                    e.stopPropagation();
                    onSelectTab(pane, tab.id);
                  }
                }}
                onMouseDown={(e) => {
                  if (e.button === 1) {
                    e.preventDefault();
                    onCloseTab(pane, tab.id);
                  }
                }}
                className={`group flex cursor-pointer items-center gap-1.5 border-r border-gray-200 px-3 py-1.5 text-xs outline-none focus-visible:bg-gray-100 dark:border-gray-700 dark:focus-visible:bg-gray-700/50 ${
                  tab.id === activeTabId
                    ? "bg-white text-gray-800 dark:bg-gray-800 dark:text-gray-200"
                    : "text-gray-500 hover:bg-gray-100 dark:text-gray-400 dark:hover:bg-gray-700/50"
                }`}
              >
                {getFileIcon(undefined, tab.file.path.split("/").pop())}
                <span className="max-w-[120px] truncate">
                  {tab.file.path.split("/").pop()}
                </span>
                <span
                  role="button"
                  tabIndex={-1}
                  onClick={(e) => {
                    e.stopPropagation();
                    onCloseTab(pane, tab.id);
                  }}
                  onKeyDown={(e) => {
                    if (e.key === "Enter" || e.key === " ") {
                      e.stopPropagation();
                      onCloseTab(pane, tab.id);
                    }
                  }}
                  className="cursor-pointer rounded p-0.5 opacity-0 transition-opacity hover:bg-gray-200 group-hover:opacity-100 dark:hover:bg-gray-600"
                  aria-label={`Close ${tab.file.path.split("/").pop()}`}
                >
                  <X className="h-3 w-3" />
                </span>
              </button>
            ))}
          </div>
          <button
            onClick={(e) => {
              e.stopPropagation();
              onClosePane(pane);
            }}
            className="flex-shrink-0 p-1.5 text-gray-400 transition-colors hover:text-gray-600 dark:hover:text-gray-300"
            title="Close this pane"
          >
            <X className="h-4 w-4" />
          </button>
        </div>
      )}

      {/* Code content */}
      <div className="min-h-0 w-full flex-1 overflow-auto">
        {loading ? (
          <div className="flex h-full flex-1 items-center justify-center">
            <Loader2 className="h-6 w-6 animate-spin text-gray-400" />
          </div>
        ) : blob?.binary ? (
          <div className="flex flex-1 flex-col items-center overflow-auto p-8">
            {activeTab && (
              <BinaryFileViewer
                repoId={activeTab.file.repoId}
                path={activeTab.file.path}
                refName={activeTab.file.ref}
              />
            )}
          </div>
        ) : blob && activeTab ? (
          <CodeViewer
            key={`${pane}-${scrollToLine?.key}`}
            content={blob.content}
            languageMode={blob.language_mode}
            language={blob.language}
            repoId={activeTab.file.repoId}
            filePath={activeTab.file.path}
            highlightLines={scrollToLine ? [scrollToLine.line] : []}
            scrollToLine={scrollToLine?.line}
            onWordClick={(word, line, col) =>
              onFindReferences(
                word,
                line,
                col,
                activeTab.file.repoId,
                activeTab.file.path,
                blob.language
              )
            }
            onGoToDefinition={(word, line, col) =>
              onGoToDefinition(
                word,
                line,
                col,
                activeTab.file.repoId,
                activeTab.file.path,
                blob.language
              )
            }
            onLineClick={(line) =>
              onLineClick(line, activeTab.file.repoId, activeTab.file.path)
            }
          />
        ) : (
          <div className="p-8 text-center text-gray-500 dark:text-gray-400">
            Select a file to view
          </div>
        )}
      </div>
    </div>
  );
}
