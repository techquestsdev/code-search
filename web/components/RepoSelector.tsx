import React from "react";
import {
  Search,
  X,
  Regex,
  Loader2,
  RefreshCw,
  Plus,
  Minus,
  Check,
  GitFork,
  FolderKanban,
} from "lucide-react";
import { Repository } from "@/lib/api";
import { Context, CONTEXT_COLORS } from "@/lib/contexts";

interface RepoSelectorProps {
  selectedContext: Context | null;
  searchQuery: string;
  setSearchQuery: (q: string) => void;
  isRegexSearch: boolean;
  setIsRegexSearch: (val: boolean) => void;
  allRepos: Repository[];
  allReposLoaded: boolean;
  loadingRepos: boolean;
  filteredRepos: Repository[];
  matchInfo: { valid: boolean; matchCount: number; totalCount: number } | null;
  refreshRepos: () => void;
  isRepoInContext: (id: number) => boolean;
  handleToggleRepo: (repo: Repository) => void;
  handleAddAllFiltered: () => void;
  handleRemoveAllFiltered: () => void;
  filteredInContextCount: number;
  filteredNotInContextCount: number;
  editingName: boolean;
  tempName: string;
  setTempName: (name: string) => void;
  setEditingName: (val: boolean) => void;
  handleSaveName: (name: string) => void;
  updateContextColor: (color: string) => void;
  handleDeleteContext: () => void;
  onUseContext: () => void;
  onClose: () => void;
}

export function RepoSelector({
  selectedContext,
  searchQuery,
  setSearchQuery,
  isRegexSearch,
  setIsRegexSearch,
  allRepos,
  allReposLoaded: _allReposLoaded,
  loadingRepos,
  filteredRepos,
  matchInfo,
  refreshRepos,
  isRepoInContext,
  handleToggleRepo,
  handleAddAllFiltered,
  handleRemoveAllFiltered,
  filteredInContextCount,
  filteredNotInContextCount,
  editingName,
  tempName,
  setTempName,
  setEditingName,
  handleSaveName,
  updateContextColor,
  handleDeleteContext,
  onUseContext,
  onClose,
}: RepoSelectorProps) {
  if (!selectedContext) {
    return (
      <div className="flex min-w-0 flex-1 flex-col">
        <div className="flex h-[49px] items-center justify-end border-b border-gray-200 px-4 dark:border-gray-700">
          <button
            onClick={onClose}
            className="rounded-lg p-1.5 text-gray-400 hover:text-gray-600 dark:hover:text-gray-300"
          >
            <X className="h-4 w-4" />
          </button>
        </div>
        <div className="flex flex-1 items-center justify-center text-gray-400">
          <div className="text-center">
            <FolderKanban className="mx-auto mb-3 h-12 w-12 opacity-50" />
            <p>Select or create a context to manage repos</p>
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className="flex min-w-0 flex-1 flex-col">
      {/* Header */}
      <div className="flex h-[49px] items-center justify-between border-b border-gray-200 px-4 dark:border-gray-700">
        <div className="flex min-w-0 flex-1 items-center gap-3">
          <button
            type="button"
            className="h-3 w-3 flex-shrink-0 cursor-pointer rounded-full outline-none ring-offset-2 transition-shadow hover:ring-2 focus-visible:ring-2 dark:ring-offset-gray-800"
            style={{ backgroundColor: selectedContext.color }}
            title="Click to change color"
            aria-label="Change context color"
            onClick={() => {
              const colors = CONTEXT_COLORS;
              const currentIndex = colors.indexOf(selectedContext.color);
              const nextColor = colors[(currentIndex + 1) % colors.length];
              updateContextColor(nextColor);
            }}
            onKeyDown={(e) => {
              if (e.key === "Enter" || e.key === " ") {
                const colors = CONTEXT_COLORS;
                const currentIndex = colors.indexOf(selectedContext.color);
                const nextColor = colors[(currentIndex + 1) % colors.length];
                updateContextColor(nextColor);
              }
            }}
          />
          {editingName ? (
            <input
              type="text"
              value={tempName}
              onChange={(e) => setTempName(e.target.value)}
              onBlur={(e) => {
                const val = e.target.value;
                setTimeout(() => handleSaveName(val), 100);
              }}
              onKeyDown={(e) => {
                if (e.key === "Enter") handleSaveName(tempName);
                else if (e.key === "Escape") setEditingName(false);
              }}
              className="min-w-0 flex-1 border-b-2 border-blue-500 bg-white px-1 text-base font-semibold outline-none dark:bg-gray-800"
            />
          ) : (
            <button
              type="button"
              className="rounded px-1 text-left text-base font-semibold text-gray-900 outline-none transition-colors hover:text-blue-600 focus-visible:ring-1 focus-visible:ring-blue-500 dark:text-white"
              onClick={() => {
                setTempName(selectedContext.name);
                setEditingName(true);
              }}
            >
              {selectedContext.name}
            </button>
          )}
          <span className="text-sm text-gray-400">
            {selectedContext.repos.length} repos
          </span>
        </div>
        <button
          onClick={onClose}
          className="rounded-lg p-1.5 text-gray-400 hover:text-gray-600 dark:hover:text-gray-300"
        >
          <X className="h-4 w-4" />
        </button>
      </div>

      {/* Search */}
      <div className="border-b border-gray-100 px-4 py-3 dark:border-gray-700">
        <div className="flex items-center gap-2">
          <div className="relative flex-1">
            <div className="absolute left-3 top-1/2 -translate-y-1/2">
              <Search className="h-4 w-4 text-gray-400" />
            </div>
            <input
              type="text"
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              placeholder={
                isRegexSearch ? "Regex pattern..." : "Search repositories..."
              }
              aria-label="Search repositories"
              className="w-full rounded-lg border border-gray-200 bg-gray-50 py-2 pl-9 pr-9 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 dark:border-gray-600 dark:bg-gray-700/50"
            />
            {searchQuery && (
              <button
                onClick={() => setSearchQuery("")}
                className="absolute right-3 top-1/2 -translate-y-1/2 text-gray-400 outline-none hover:text-gray-600 focus-visible:text-gray-600 dark:hover:text-gray-300"
                aria-label="Clear search"
              >
                <X className="h-4 w-4" />
              </button>
            )}
          </div>
          <button
            onClick={() => setIsRegexSearch(!isRegexSearch)}
            className={`rounded-lg border p-2 transition-colors ${isRegexSearch ? "border-blue-300 bg-blue-50 text-blue-600 dark:border-blue-700 dark:bg-blue-900/30 dark:text-blue-400" : "border-gray-200 text-gray-400 dark:border-gray-600"}`}
            title="Toggle regex"
          >
            <Regex className="h-4 w-4" />
          </button>
        </div>
        <div className="mt-1.5 flex items-center justify-between text-xs text-gray-500">
          <div>
            {matchInfo && !matchInfo.valid ? (
              <span className="text-red-500">Invalid regex</span>
            ) : searchQuery ? (
              <span>
                Matching{" "}
                <span className="font-medium text-blue-600 dark:text-blue-400">
                  {matchInfo?.matchCount}
                </span>{" "}
                of {matchInfo?.totalCount}
              </span>
            ) : (
              <span>{allRepos.length} repos available</span>
            )}
          </div>
          <button
            onClick={refreshRepos}
            className="flex items-center gap-1 hover:text-gray-700 dark:hover:text-gray-300"
          >
            <RefreshCw className="h-3 w-3" /> Refresh
          </button>
        </div>
      </div>

      {/* List */}
      <div className="flex-1 overflow-y-auto p-3">
        {loadingRepos ? (
          <div className="flex items-center justify-center py-12">
            <Loader2 className="animate-spin text-gray-400" />
          </div>
        ) : (
          <div className="space-y-2">
            <div className="mb-3 flex items-center justify-between">
              <span className="text-xs text-gray-500">
                {filteredRepos.length} repos
              </span>
              <div className="flex gap-2">
                {filteredNotInContextCount > 0 && (
                  <button
                    onClick={handleAddAllFiltered}
                    className="flex items-center gap-1 text-xs text-green-600 dark:text-green-400"
                  >
                    <Plus className="h-3 w-3" /> Add all (
                    {filteredNotInContextCount})
                  </button>
                )}
                {filteredInContextCount > 0 && (
                  <button
                    onClick={handleRemoveAllFiltered}
                    className="flex items-center gap-1 text-xs text-red-500 dark:text-red-400"
                  >
                    <Minus className="h-3 w-3" /> Remove all (
                    {filteredInContextCount})
                  </button>
                )}
              </div>
            </div>
            {filteredRepos.map((repo) => (
              <button
                key={repo.id}
                onClick={() => handleToggleRepo(repo)}
                className={`flex w-full items-center gap-3 rounded-lg border px-3 py-2 transition-all ${isRepoInContext(repo.id) ? "border-blue-200 bg-blue-50 dark:border-blue-800 dark:bg-blue-900/20" : "border-gray-200 dark:border-gray-700"}`}
              >
                <div
                  className={`flex h-5 w-5 items-center justify-center rounded ${isRepoInContext(repo.id) ? "bg-blue-600 text-white" : "border-2 border-gray-300 dark:border-gray-600"}`}
                >
                  {isRepoInContext(repo.id) && <Check className="h-3 w-3" />}
                </div>
                <GitFork className="h-4 w-4 text-gray-400" />
                <span className="flex-1 truncate text-left text-sm text-gray-700 dark:text-gray-300">
                  {repo.name}
                </span>
              </button>
            ))}
          </div>
        )}
      </div>

      {/* Footer */}
      <div className="flex justify-between border-t border-gray-200 px-6 py-3 dark:border-gray-700">
        <button
          onClick={handleDeleteContext}
          className="text-sm text-red-600 dark:text-red-400"
        >
          Delete Context
        </button>
        <button
          onClick={onUseContext}
          className="rounded-lg bg-blue-600 px-4 py-2 text-sm font-medium text-white"
        >
          Use This Context
        </button>
      </div>
    </div>
  );
}
