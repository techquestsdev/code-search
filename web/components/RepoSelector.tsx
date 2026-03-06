import React from "react";
import { Search, X, Regex, Loader2, RefreshCw, Plus, Minus, Check, GitFork, FolderKanban } from "lucide-react";
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
      <div className="flex-1 flex flex-col min-w-0">
        <div className="h-[49px] flex items-center justify-end px-4 border-b border-gray-200 dark:border-gray-700">
          <button onClick={onClose} className="p-1.5 text-gray-400 hover:text-gray-600 dark:hover:text-gray-300 rounded-lg">
            <X className="w-4 h-4" />
          </button>
        </div>
        <div className="flex-1 flex items-center justify-center text-gray-400">
          <div className="text-center">
            <FolderKanban className="w-12 h-12 mx-auto mb-3 opacity-50" />
            <p>Select or create a context to manage repos</p>
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className="flex-1 flex flex-col min-w-0">
      {/* Header */}
      <div className="h-[49px] flex items-center justify-between px-4 border-b border-gray-200 dark:border-gray-700">
        <div className="flex items-center gap-3 flex-1 min-w-0">
          <button
            type="button"
            className="w-3 h-3 rounded-full cursor-pointer hover:ring-2 ring-offset-2 dark:ring-offset-gray-800 transition-shadow flex-shrink-0 outline-none focus-visible:ring-2"
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
              if (e.key === 'Enter' || e.key === ' ') {
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
              className="flex-1 min-w-0 text-base font-semibold bg-white dark:bg-gray-800 border-b-2 border-blue-500 outline-none px-1"
            />
          ) : (
            <button
              type="button"
              className="text-base font-semibold text-gray-900 dark:text-white hover:text-blue-600 transition-colors text-left outline-none focus-visible:ring-1 focus-visible:ring-blue-500 rounded px-1"
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
        <button onClick={onClose} className="p-1.5 text-gray-400 hover:text-gray-600 dark:hover:text-gray-300 rounded-lg">
          <X className="w-4 h-4" />
        </button>
      </div>

      {/* Search */}
      <div className="px-4 py-3 border-b border-gray-100 dark:border-gray-700">
        <div className="flex items-center gap-2">
          <div className="relative flex-1">
            <div className="absolute left-3 top-1/2 -translate-y-1/2">
              <Search className="w-4 h-4 text-gray-400" />
            </div>
            <input
              type="text"
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              placeholder={isRegexSearch ? "Regex pattern..." : "Search repositories..."}
              aria-label="Search repositories"
              className="w-full pl-9 pr-9 py-2 text-sm bg-gray-50 dark:bg-gray-700/50 border border-gray-200 dark:border-gray-600 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500"
            />
            {searchQuery && (
              <button
                onClick={() => setSearchQuery("")}
                className="absolute right-3 top-1/2 -translate-y-1/2 text-gray-400 hover:text-gray-600 dark:hover:text-gray-300 outline-none focus-visible:text-gray-600"
                aria-label="Clear search"
              >
                <X className="w-4 h-4" />
              </button>
            )}
          </div>
          <button
            onClick={() => setIsRegexSearch(!isRegexSearch)}
            className={`p-2 rounded-lg border transition-colors ${isRegexSearch ? "bg-blue-50 dark:bg-blue-900/30 text-blue-600 dark:text-blue-400 border-blue-300 dark:border-blue-700" : "text-gray-400 border-gray-200 dark:border-gray-600"}`}
            title="Toggle regex"
          >
            <Regex className="w-4 h-4" />
          </button>
        </div>
        <div className="mt-1.5 flex items-center justify-between text-xs text-gray-500">
          <div>
            {matchInfo && !matchInfo.valid ? (
              <span className="text-red-500">Invalid regex</span>
            ) : searchQuery ? (
              <span>Matching <span className="text-blue-600 dark:text-blue-400 font-medium">{matchInfo?.matchCount}</span> of {matchInfo?.totalCount}</span>
            ) : (
              <span>{allRepos.length} repos available</span>
            )}
          </div>
          <button onClick={refreshRepos} className="flex items-center gap-1 hover:text-gray-700 dark:hover:text-gray-300">
            <RefreshCw className="w-3 h-3" /> Refresh
          </button>
        </div>
      </div>

      {/* List */}
      <div className="flex-1 overflow-y-auto p-3">
        {loadingRepos ? (
          <div className="flex items-center justify-center py-12"><Loader2 className="animate-spin text-gray-400" /></div>
        ) : (
          <div className="space-y-2">
            <div className="flex items-center justify-between mb-3">
              <span className="text-xs text-gray-500">{filteredRepos.length} repos</span>
              <div className="flex gap-2">
                {filteredNotInContextCount > 0 && (
                  <button onClick={handleAddAllFiltered} className="text-xs text-green-600 dark:text-green-400 flex items-center gap-1"><Plus className="w-3 h-3" /> Add all ({filteredNotInContextCount})</button>
                )}
                {filteredInContextCount > 0 && (
                  <button onClick={handleRemoveAllFiltered} className="text-xs text-red-500 dark:text-red-400 flex items-center gap-1"><Minus className="w-3 h-3" /> Remove all ({filteredInContextCount})</button>
                )}
              </div>
            </div>
            {filteredRepos.map(repo => (
              <button
                key={repo.id}
                onClick={() => handleToggleRepo(repo)}
                className={`w-full flex items-center gap-3 px-3 py-2 rounded-lg border transition-all ${isRepoInContext(repo.id) ? "bg-blue-50 dark:bg-blue-900/20 border-blue-200 dark:border-blue-800" : "border-gray-200 dark:border-gray-700"}`}
              >
                <div className={`w-5 h-5 rounded flex items-center justify-center ${isRepoInContext(repo.id) ? "bg-blue-600 text-white" : "border-2 border-gray-300 dark:border-gray-600"}`}>
                  {isRepoInContext(repo.id) && <Check className="w-3 h-3" />}
                </div>
                <GitFork className="w-4 h-4 text-gray-400" />
                <span className="flex-1 text-left text-sm truncate text-gray-700 dark:text-gray-300">{repo.name}</span>
              </button>
            ))}
          </div>
        )}
      </div>

      {/* Footer */}
      <div className="px-6 py-3 border-t border-gray-200 dark:border-gray-700 flex justify-between">
        <button onClick={handleDeleteContext} className="text-sm text-red-600 dark:text-red-400">Delete Context</button>
        <button onClick={onUseContext} className="px-4 py-2 text-sm font-medium text-white bg-blue-600 rounded-lg">Use This Context</button>
      </div>
    </div>
  );
}
