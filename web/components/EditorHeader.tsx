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
  ExternalLink
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
      <div className="flex-shrink-0 border-b border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-800 px-3 py-1.5">
        <div className="flex items-center gap-3">
          {/* File tree toggle */}
          <button
            type="button"
            onClick={onToggleFileTree}
            className={`p-1.5 rounded-lg transition-colors ${showFileTree
              ? "bg-blue-100 dark:bg-blue-900/30 text-blue-600 dark:text-blue-400"
              : "text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200 hover:bg-gray-100 dark:hover:bg-gray-700"
              }`}
            title={showFileTree ? "Hide file tree" : "Show file tree"}
          >
            {showFileTree ? (
              <PanelLeftClose className="w-4 h-4" />
            ) : (
              <PanelLeftOpen className="w-4 h-4" />
            )}
          </button>

          {/* Search dropdown */}
          <SearchDropdown
            repoId={repoId}
            repoName={repo?.name}
            className="flex-1 max-w-xl"
          />

          {/* Back/Forward navigation */}
          <div className="flex items-center gap-1 border-l border-gray-200 dark:border-gray-700 pl-3">
            <button
              type="button"
              onClick={onHistoryBack}
              disabled={!navHistory.canGoBack}
              className={`p-1.5 rounded transition-colors ${navHistory.canGoBack
                ? "text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200 hover:bg-gray-100 dark:hover:bg-gray-700"
                : "text-gray-300 dark:text-gray-600 cursor-not-allowed"
                }`}
              title="Go back (⌘[)"
            >
              <ChevronLeft className="w-4 h-4" />
            </button>
            <button
              type="button"
              onClick={onHistoryForward}
              disabled={!navHistory.canGoForward}
              className={`p-1.5 rounded transition-colors ${navHistory.canGoForward
                ? "text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200 hover:bg-gray-100 dark:hover:bg-gray-700"
                : "text-gray-300 dark:text-gray-600 cursor-not-allowed"
                }`}
              title="Go forward (⌘])"
            >
              <ChevronRight className="w-4 h-4" />
            </button>
          </div>

          {/* Actions */}
          <div className="flex items-center gap-1 border-l border-gray-200 dark:border-gray-700 pl-3">
            <button
              type="button"
              onClick={onShowQuickPicker}
              className="p-1.5 text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200 hover:bg-gray-100 dark:hover:bg-gray-700 rounded transition-colors"
              title="Quick open file (⌘P)"
            >
              <Search className="w-4 h-4" />
            </button>
            <button
              type="button"
              onClick={onToggleShortcutsHelp}
              className={`p-1.5 rounded transition-colors ${showShortcutsHelp
                ? "bg-blue-100 dark:bg-blue-900/30 text-blue-600 dark:text-blue-400"
                : "text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200 hover:bg-gray-100 dark:hover:bg-gray-700"
                }`}
              title="Keyboard shortcuts"
            >
              <Keyboard className="w-4 h-4" />
            </button>
            {isFile && (
              <button
                type="button"
                onClick={onToggleSymbols}
                className={`p-1.5 rounded transition-colors ${showSymbols
                  ? "bg-blue-100 dark:bg-blue-900/30 text-blue-600 dark:text-blue-400"
                  : "text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200 hover:bg-gray-100 dark:hover:bg-gray-700"
                  }`}
                title={showSymbols ? "Hide symbols" : "Show symbols"}
              >
                {showSymbols ? (
                  <PanelRightClose className="w-4 h-4" />
                ) : (
                  <ListTree className="w-4 h-4" />
                )}
              </button>
            )}
          </div>
        </div>
      </div>

      {/* Secondary toolbar with repo info and breadcrumbs */}
      <div className="flex-shrink-0 border-b border-gray-200 dark:border-gray-700 bg-gray-50 dark:bg-gray-800/80 px-3 py-1">
        <div className="flex flex-wrap items-center gap-3">
          {/* Back to repos */}
          <Link
            href="/repos"
            className="flex items-center gap-1 text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200 transition-colors"
          >
            <ArrowLeft className="w-4 h-4" />
            <span className="text-sm hidden sm:inline">Repos</span>
          </Link>
          <span className="text-gray-300 dark:text-gray-600">/</span>

          {/* Repo name - shows active pane's repo in split mode */}
          <span className="text-sm font-medium text-gray-700 dark:text-gray-300 truncate max-w-[150px]">
            {displayRepo?.name || repo?.name || "Loading..."}
          </span>

          {/* Ref selector - shows active pane's refs in split mode */}
          <div className="relative">
            <select
              value={currentDisplayRef}
              onChange={(e) => onRefChange(e.target.value)}
              className="appearance-none pl-6 pr-4 py-0.5 text-xs border border-gray-200 dark:border-gray-700 rounded bg-white dark:bg-gray-800 focus:outline-none focus:border-blue-500 cursor-pointer"
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
              {!(displayRefs || refs)?.branches?.length && !(displayRefs || refs)?.tags?.length && (
                <option value={currentDisplayRef}>{currentDisplayRef}</option>
              )}
            </select>
            {(displayRefs || refs)?.tags?.includes(currentDisplayRef) ? (
              <Tag className="absolute left-1.5 top-1/2 -translate-y-1/2 w-3 h-3 text-gray-400 pointer-events-none" />
            ) : (
              <GitBranch className="absolute left-1.5 top-1/2 -translate-y-1/2 w-3 h-3 text-gray-400 pointer-events-none" />
            )}
          </div>

          {/* Breadcrumbs */}
          <nav className="flex items-center gap-1 text-sm overflow-x-auto flex-1">
            <button
              onClick={() => !splitPaneEnabled && onNavigate("")}
              className={`flex items-center gap-1 px-1.5 py-0.5 rounded transition-colors ${splitPaneEnabled ? "cursor-default" : "hover:bg-gray-200 dark:hover:bg-gray-700"
                }`}
            >
              <Home className="w-3.5 h-3.5 text-gray-400" />
              <span className="text-xs font-medium">{displayRepo?.name?.split("/").pop() || repo?.name?.split("/").pop() || "root"}</span>
            </button>

            {pathParts.map((part, index) => {
              const pathUpTo = pathParts.slice(0, index + 1).join("/");
              const isLast = index === pathParts.length - 1;

              return (
                <span key={pathUpTo} className="flex items-center">
                  <ChevronRight className="w-3.5 h-3.5 text-gray-400" />
                  <button
                    onClick={() => !splitPaneEnabled && onNavigate(pathUpTo)}
                    className={`px-1.5 py-0.5 rounded transition-colors text-xs ${isLast ? "font-medium text-blue-600 dark:text-blue-400" : ""} ${splitPaneEnabled ? "cursor-default" : "hover:bg-gray-200 dark:hover:bg-gray-700"
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
            <div className="flex items-center gap-1 ml-auto">
              <button
                onClick={onCopyPath}
                className="p-1 text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200 transition-colors"
                title="Copy path"
              >
                {copied ? <Check className="w-4 h-4 text-green-500" /> : <Copy className="w-4 h-4" />}
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
                  className="p-1 text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200 transition-colors"
                  title="View on code host"
                >
                  <ExternalLink className="w-4 h-4" />
                </a>
              )}
            </div>
          )}
        </div>
      </div>
    </>
  );
}
