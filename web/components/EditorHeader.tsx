"use client";

import React from "react";
import Link from "next/link";
import {
  PanelLeftClose,
  PanelLeftOpen,
  ChevronLeft,
  ChevronRight,
  Search,
  Keyboard,
  ListTree,
  PanelRightClose,
  ArrowLeft,
  Tag,
  GitBranch,
  Home,
  Copy,
  Check,
  ExternalLink,
} from "lucide-react";
import { SearchDropdown } from "./SearchDropdown";
import { Repository, RefsResponse } from "@/lib/api";

interface EditorHeaderProps {
  repoId: number;
  repo: Repository | null;
  displayRepo: Repository | null;
  refs: RefsResponse | null;
  displayRefs: RefsResponse | null;
  currentRef: string;
  currentDisplayRef: string;
  showFileTree: boolean;
  showSymbols: boolean;
  showShortcutsHelp: boolean;
  isFile: boolean;
  currentPath: string;
  pathParts: string[];
  copied: boolean;
  splitPaneEnabled: boolean;
  navHistory: { canGoBack: boolean; canGoForward: boolean };
  onToggleFileTree: () => void;
  onToggleSymbols: () => void;
  onToggleShortcutsHelp: () => void;
  onShowQuickPicker: () => void;
  onHistoryBack: () => void;
  onHistoryForward: () => void;
  onRefChange: (ref: string) => void;
  onNavigate: (path: string) => void;
  onCopyPath: () => void;
}

export function EditorHeader({
  repoId,
  repo,
  displayRepo,
  refs,
  displayRefs,
  currentRef,
  currentDisplayRef,
  showFileTree,
  showSymbols,
  showShortcutsHelp,
  isFile,
  currentPath,
  pathParts,
  copied,
  splitPaneEnabled,
  navHistory,
  onToggleFileTree,
  onToggleSymbols,
  onToggleShortcutsHelp,
  onShowQuickPicker,
  onHistoryBack,
  onHistoryForward,
  onRefChange,
  onNavigate,
  onCopyPath,
}: EditorHeaderProps) {
  return (
    <>
      {/* Top Bar with Search */}
      <div className="flex-shrink-0 border-b border-gray-200 bg-white px-3 py-1.5 dark:border-gray-700 dark:bg-gray-800">
        <div className="flex items-center gap-3">
          {/* File tree toggle */}
          <button
            type="button"
            onClick={onToggleFileTree}
            className={`rounded-lg p-1.5 transition-colors ${
              showFileTree
                ? "bg-blue-100 text-blue-600 dark:bg-blue-900/30 dark:text-blue-400"
                : "text-gray-500 hover:bg-gray-100 hover:text-gray-700 dark:text-gray-400 dark:hover:bg-gray-700 dark:hover:text-gray-200"
            }`}
            title={showFileTree ? "Hide file tree" : "Show file tree"}
          >
            {showFileTree ? (
              <PanelLeftClose className="h-4 w-4" />
            ) : (
              <PanelLeftOpen className="h-4 w-4" />
            )}
          </button>

          {/* Search dropdown */}
          <SearchDropdown
            repoId={repoId}
            repoName={repo?.name}
            className="max-w-xl flex-1"
          />

          {/* Back/Forward navigation */}
          <div className="flex items-center gap-1 border-l border-gray-200 pl-3 dark:border-gray-700">
            <button
              type="button"
              onClick={onHistoryBack}
              disabled={!navHistory.canGoBack}
              className={`rounded p-1.5 transition-colors ${
                navHistory.canGoBack
                  ? "text-gray-500 hover:bg-gray-100 hover:text-gray-700 dark:text-gray-400 dark:hover:bg-gray-700 dark:hover:text-gray-200"
                  : "cursor-not-allowed text-gray-300 dark:text-gray-600"
              }`}
              title="Go back (⌘[)"
            >
              <ChevronLeft className="h-4 w-4" />
            </button>
            <button
              type="button"
              onClick={onHistoryForward}
              disabled={!navHistory.canGoForward}
              className={`rounded p-1.5 transition-colors ${
                navHistory.canGoForward
                  ? "text-gray-500 hover:bg-gray-100 hover:text-gray-700 dark:text-gray-400 dark:hover:bg-gray-700 dark:hover:text-gray-200"
                  : "cursor-not-allowed text-gray-300 dark:text-gray-600"
              }`}
              title="Go forward (⌘])"
            >
              <ChevronRight className="h-4 w-4" />
            </button>
          </div>

          {/* Actions */}
          <div className="flex items-center gap-1 border-l border-gray-200 pl-3 dark:border-gray-700">
            <button
              type="button"
              onClick={onShowQuickPicker}
              className="rounded p-1.5 text-gray-500 transition-colors hover:bg-gray-100 hover:text-gray-700 dark:text-gray-400 dark:hover:bg-gray-700 dark:hover:text-gray-200"
              title="Quick open file (⌘P)"
            >
              <Search className="h-4 w-4" />
            </button>
            <button
              type="button"
              onClick={onToggleShortcutsHelp}
              className={`rounded p-1.5 transition-colors ${
                showShortcutsHelp
                  ? "bg-blue-100 text-blue-600 dark:bg-blue-900/30 dark:text-blue-400"
                  : "text-gray-500 hover:bg-gray-100 hover:text-gray-700 dark:text-gray-400 dark:hover:bg-gray-700 dark:hover:text-gray-200"
              }`}
              title="Keyboard shortcuts"
            >
              <Keyboard className="h-4 w-4" />
            </button>
            {isFile && (
              <button
                type="button"
                onClick={onToggleSymbols}
                className={`rounded p-1.5 transition-colors ${
                  showSymbols
                    ? "bg-blue-100 text-blue-600 dark:bg-blue-900/30 dark:text-blue-400"
                    : "text-gray-500 hover:bg-gray-100 hover:text-gray-700 dark:text-gray-400 dark:hover:bg-gray-700 dark:hover:text-gray-200"
                }`}
                title={showSymbols ? "Hide symbols" : "Show symbols"}
              >
                {showSymbols ? (
                  <PanelRightClose className="h-4 w-4" />
                ) : (
                  <ListTree className="h-4 w-4" />
                )}
              </button>
            )}
          </div>
        </div>
      </div>

      {/* Secondary toolbar with repo info and breadcrumbs */}
      <div className="flex-shrink-0 border-b border-gray-200 bg-gray-50 px-3 py-1 dark:border-gray-700 dark:bg-gray-800/80">
        <div className="flex flex-wrap items-center gap-3">
          {/* Back to repos */}
          <Link
            href="/repos"
            className="flex items-center gap-1 text-gray-500 transition-colors hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200"
          >
            <ArrowLeft className="h-4 w-4" />
            <span className="hidden text-sm sm:inline">Repos</span>
          </Link>
          <span className="text-gray-300 dark:text-gray-600">/</span>

          {/* Repo name - shows active pane's repo in split mode */}
          <span className="max-w-[150px] truncate text-sm font-medium text-gray-700 dark:text-gray-300">
            {displayRepo?.name || repo?.name || "Loading..."}
          </span>

          {/* Ref selector - shows active pane's refs in split mode */}
          <div className="relative">
            <select
              value={currentDisplayRef}
              onChange={(e) => onRefChange(e.target.value)}
              className="cursor-pointer appearance-none rounded border border-gray-200 bg-white py-0.5 pl-6 pr-4 text-xs focus:border-blue-500 focus:outline-none dark:border-gray-700 dark:bg-gray-800"
            >
              {(displayRefs || refs)?.branches?.length ? (
                <optgroup label="Branches">
                  {(displayRefs || refs)?.branches?.map((branch) => (
                    <option key={`branch-${branch}`} value={branch}>
                      {branch}
                    </option>
                  ))}
                </optgroup>
              ) : null}
              {(displayRefs || refs)?.tags?.length ? (
                <optgroup label="Tags">
                  {(displayRefs || refs)?.tags?.map((tag) => (
                    <option key={`tag-${tag}`} value={tag}>
                      {tag}
                    </option>
                  ))}
                </optgroup>
              ) : null}
              {!(displayRefs || refs)?.branches?.length &&
                !(displayRefs || refs)?.tags?.length && (
                  <option value={currentDisplayRef}>{currentDisplayRef}</option>
                )}
            </select>
            {(displayRefs || refs)?.tags?.includes(currentDisplayRef) ? (
              <Tag className="pointer-events-none absolute left-1.5 top-1/2 h-3 w-3 -translate-y-1/2 text-gray-400" />
            ) : (
              <GitBranch className="pointer-events-none absolute left-1.5 top-1/2 h-3 w-3 -translate-y-1/2 text-gray-400" />
            )}
          </div>

          {/* Breadcrumbs */}
          <nav className="flex flex-1 items-center gap-1 overflow-x-auto text-sm">
            <button
              onClick={() => !splitPaneEnabled && onNavigate("")}
              className={`flex items-center gap-1 rounded px-1.5 py-0.5 transition-colors ${
                splitPaneEnabled
                  ? "cursor-default"
                  : "hover:bg-gray-200 dark:hover:bg-gray-700"
              }`}
            >
              <Home className="h-3.5 w-3.5 text-gray-400" />
              <span className="text-xs font-medium">
                {displayRepo?.name?.split("/").pop() ||
                  repo?.name?.split("/").pop() ||
                  "root"}
              </span>
            </button>

            {pathParts.map((part, index) => {
              const pathUpTo = pathParts.slice(0, index + 1).join("/");
              const isLast = index === pathParts.length - 1;

              return (
                <span key={pathUpTo} className="flex items-center">
                  <ChevronRight className="h-3.5 w-3.5 text-gray-400" />
                  <button
                    onClick={() => !splitPaneEnabled && onNavigate(pathUpTo)}
                    className={`rounded px-1.5 py-0.5 text-xs transition-colors ${isLast ? "font-medium text-blue-600 dark:text-blue-400" : ""} ${
                      splitPaneEnabled
                        ? "cursor-default"
                        : "hover:bg-gray-200 dark:hover:bg-gray-700"
                    }`}
                  >
                    {part}
                  </button>
                </span>
              );
            })}
          </nav>

          {/* File actions */}
          {isFile && currentPath && (
            <div className="ml-auto flex items-center gap-1">
              <button
                onClick={onCopyPath}
                className="p-1 text-gray-500 transition-colors hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200"
                title="Copy path"
              >
                {copied ? (
                  <Check className="h-4 w-4 text-green-500" />
                ) : (
                  <Copy className="h-4 w-4" />
                )}
              </button>
              {repo && (
                <a
                  href={(() => {
                    const baseUrl = repo.clone_url.replace(".git", "");
                    // GitHub uses /blob/, GitLab uses /-/blob/, Bitbucket uses /src/
                    if (baseUrl.includes("github.com")) {
                      return `${baseUrl}/blob/${currentRef}/${currentPath}`;
                    } else if (baseUrl.includes("bitbucket")) {
                      return `${baseUrl}/src/${currentRef}/${currentPath}`;
                    } else {
                      // GitLab and others use /-/blob/
                      return `${baseUrl}/-/blob/${currentRef}/${currentPath}`;
                    }
                  })()}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="p-1 text-gray-500 transition-colors hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200"
                  title="View on code host"
                >
                  <ExternalLink className="h-4 w-4" />
                </a>
              )}
            </div>
          )}
        </div>
      </div>
    </>
  );
}
