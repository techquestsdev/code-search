"use client";

import { useState, useEffect, useCallback, useRef } from "react";
import { api, Repository, Connection } from "@/lib/api";
import {
  GitBranch,
  Search,
  RefreshCw,
  Trash2,
  Loader2,
  CheckCircle2,
  Clock,
  AlertCircle,
  ChevronLeft,
  ChevronRight,
  ChevronsLeft,
  ChevronsRight,
  FolderGit2,
  Filter,
  MoreVertical,
  X,
  EyeOff,
  Eye,
  Settings2,
  Plus,
  RotateCcw,
} from "lucide-react";

const PAGE_SIZE = 15;

// Modal for editing branches
function BranchesModal({
  repo,
  availableBranches,
  onClose,
  onSave,
}: {
  repo: Repository;
  availableBranches: string[];
  onClose: () => void;
  onSave: (branches: string[]) => Promise<void>;
}) {
  const [selectedBranches, setSelectedBranches] = useState<string[]>(
    repo.branches?.length > 0 ? repo.branches : [repo.default_branch || "main"]
  );
  const [saving, setSaving] = useState(false);
  const [newBranch, setNewBranch] = useState("");

  const handleToggleBranch = (branch: string) => {
    setSelectedBranches(prev =>
      prev.includes(branch)
        ? prev.filter(b => b !== branch)
        : [...prev, branch]
    );
  };

  const handleAddCustomBranch = () => {
    if (newBranch.trim() && !selectedBranches.includes(newBranch.trim())) {
      setSelectedBranches(prev => [...prev, newBranch.trim()]);
      setNewBranch("");
    }
  };

  const handleSave = async () => {
    setSaving(true);
    try {
      await onSave(selectedBranches);
      onClose();
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50 p-4">
      <div className="bg-white dark:bg-gray-800 rounded-xl shadow-xl max-w-md w-full max-h-[80vh] overflow-hidden">
        <div className="px-6 py-4 border-b border-gray-200 dark:border-gray-700">
          <div className="flex items-center justify-between">
            <h2 className="text-lg font-semibold">Configure Branches</h2>
            <button
              type="button"
              onClick={onClose}
              className="p-1 hover:bg-gray-100 dark:hover:bg-gray-700 rounded-lg transition-colors"
              aria-label="Close modal"
            >
              <X className="w-5 h-5 text-gray-500" />
            </button>
          </div>
          <p className="text-sm text-gray-500 dark:text-gray-400 mt-1">
            Select which branches to index for <span className="font-medium text-gray-700 dark:text-gray-300">{repo.name}</span>
          </p>
        </div>

        <div className="px-6 py-4 max-h-[50vh] overflow-y-auto">
          {/* Available branches from git */}
          {availableBranches.length > 0 && (
            <div className="mb-4">
              <h3 className="text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">Available Branches</h3>
              <div className="space-y-1">
                {availableBranches.map(branch => (
                  <label
                    key={branch}
                    className="flex items-center gap-2 p-2 hover:bg-gray-50 dark:hover:bg-gray-700/50 rounded-lg cursor-pointer"
                  >
                    <input
                      type="checkbox"
                      checked={selectedBranches.includes(branch)}
                      onChange={() => handleToggleBranch(branch)}
                      className="w-4 h-4 text-blue-600 rounded border-gray-300 focus:ring-blue-500"
                    />
                    <span className="text-sm">
                      {branch}
                      {branch === repo.default_branch && (
                        <span className="ml-2 text-xs text-gray-500">(default)</span>
                      )}
                    </span>
                  </label>
                ))}
              </div>
            </div>
          )}

          {/* Currently selected branches not in available list */}
          {selectedBranches.filter(b => !availableBranches.includes(b)).length > 0 && (
            <div className="mb-4">
              <h3 className="text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">Custom Branches</h3>
              <div className="space-y-1">
                {selectedBranches
                  .filter(b => !availableBranches.includes(b))
                  .map(branch => (
                    <label
                      key={branch}
                      className="flex items-center gap-2 p-2 hover:bg-gray-50 dark:hover:bg-gray-700/50 rounded-lg cursor-pointer"
                    >
                      <input
                        type="checkbox"
                        checked={true}
                        onChange={() => handleToggleBranch(branch)}
                        className="w-4 h-4 text-blue-600 rounded border-gray-300 focus:ring-blue-500"
                      />
                      <span className="text-sm">{branch}</span>
                    </label>
                  ))}
              </div>
            </div>
          )}

          {/* Add custom branch */}
          <div>
            <h3 className="text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">Add Branch</h3>
            <div className="flex gap-2">
              <input
                id="custom-branch-input"
                type="text"
                value={newBranch}
                onChange={(e) => setNewBranch(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === "Enter") {
                    e.preventDefault();
                    handleAddCustomBranch();
                  }
                }}
                placeholder="e.g., feature/new-feature"
                aria-label="Custom branch name"
                className="flex-1 px-3 py-2 text-sm border border-gray-200 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 focus:outline-none focus:ring-2 focus:ring-blue-500"
              />
              <button
                type="button"
                onClick={handleAddCustomBranch}
                disabled={!newBranch.trim()}
                className="px-3 py-2 bg-gray-100 dark:bg-gray-700 hover:bg-gray-200 dark:hover:bg-gray-600 rounded-lg transition-colors disabled:opacity-50"
                aria-label="Add custom branch"
              >
                <Plus className="w-4 h-4" />
              </button>
            </div>
          </div>
        </div>

        <div className="px-6 py-4 border-t border-gray-200 dark:border-gray-700 flex justify-between items-center">
          <p className="text-xs text-gray-500 dark:text-gray-400">
            {selectedBranches.length} branch{selectedBranches.length !== 1 ? "es" : ""} selected
          </p>
          <div className="flex gap-2">
            <button
              type="button"
              onClick={onClose}
              className="px-4 py-2 text-sm font-medium text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 rounded-lg transition-colors"
            >
              Cancel
            </button>
            <button
              type="button"
              onClick={handleSave}
              disabled={saving || selectedBranches.length === 0}
              className="px-4 py-2 text-sm font-medium bg-blue-600 text-white hover:bg-blue-700 rounded-lg transition-colors disabled:opacity-50 flex items-center gap-2"
            >
              {saving && <Loader2 className="w-4 h-4 animate-spin" />}
              Save & Reindex
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}

// Dropdown menu component for row actions
function ActionMenu({
  repo,
  onSync,
  onDelete,
  onExclude,
  onInclude,
  onRestore,
  onConfigureBranches,
  syncingIds,
  readonly
}: {
  repo: Repository;
  onSync: (id: number) => void;
  onDelete: (id: number, name: string) => void;
  onExclude: (id: number) => void;
  onInclude: (id: number) => void;
  onRestore: (id: number) => void;
  onConfigureBranches: (repo: Repository) => void;
  syncingIds: Set<number>;
  readonly?: boolean;
}) {
  const [isOpen, setIsOpen] = useState(false);
  const [menuPosition, setMenuPosition] = useState({ top: 0, right: 0 });
  const buttonRef = useRef<HTMLButtonElement>(null);
  const menuRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!isOpen) return;
    function handleClickOutside(event: MouseEvent) {
      if (
        menuRef.current &&
        !menuRef.current.contains(event.target as Node) &&
        buttonRef.current &&
        !buttonRef.current.contains(event.target as Node)
      ) {
        setIsOpen(false);
      }
    }
    document.addEventListener("mousedown", handleClickOutside);
    return () => document.removeEventListener("mousedown", handleClickOutside);
  }, [isOpen]);

  const handleToggle = () => {
    if (!isOpen && buttonRef.current) {
      const rect = buttonRef.current.getBoundingClientRect();
      setMenuPosition({
        top: rect.bottom + 4,
        right: window.innerWidth - rect.right,
      });
    }
    setIsOpen(!isOpen);
  };

  return (
    <div className="relative">
      <button
        ref={buttonRef}
        onClick={handleToggle}
        className="p-2 hover:bg-gray-100 dark:hover:bg-gray-700 rounded-lg transition-all focus:outline-none focus:ring-2 focus:ring-blue-500/20"
      >
        <MoreVertical className="w-4 h-4 text-gray-500" />
      </button>
      {isOpen && (
        <div
          ref={menuRef}
          className="fixed w-36 bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg shadow-lg z-50"
          style={{ top: menuPosition.top, right: menuPosition.right }}
        >
          {repo.deleted ? (
            <button
              type="button"
              onClick={() => {
                onRestore(repo.id);
                setIsOpen(false);
              }}
              className="w-full flex items-center gap-2 px-3 py-2 text-sm text-left text-green-600 dark:text-green-400 hover:bg-green-50 dark:hover:bg-green-900/20 transition-colors rounded-lg"
            >
              <RotateCcw className="w-4 h-4" />
              <span>Restore</span>
            </button>
          ) : (
            <>
              {!repo.excluded && (
                <button
                  type="button"
                  onClick={() => {
                    onSync(repo.id);
                    setIsOpen(false);
                  }}
                  disabled={syncingIds.has(repo.id) || repo.status === "pending" || repo.status === "cloning" || repo.status === "indexing"}
                  className="w-full flex items-center gap-2 px-3 py-2 text-sm text-left hover:bg-gray-50 dark:hover:bg-gray-700 disabled:opacity-50 transition-colors rounded-t-lg"
                >
                  {syncingIds.has(repo.id) || repo.status === "pending" || repo.status === "cloning" || repo.status === "indexing" ? (
                    <Loader2 className="w-4 h-4 animate-spin text-gray-500 dark:text-gray-400" />
                  ) : (
                    <RefreshCw className="w-4 h-4 text-gray-500 dark:text-gray-400" />
                  )}
                  <span>{syncingIds.has(repo.id) ? "Syncing..." : repo.status === "pending" || repo.status === "cloning" || repo.status === "indexing" ? "In progress..." : "Sync"}</span>
                </button>
              )}
              {repo.excluded ? (
                <button
                  type="button"
                  onClick={() => {
                    onInclude(repo.id);
                    setIsOpen(false);
                  }}
                  className="w-full flex items-center gap-2 px-3 py-2 text-sm text-left text-green-600 dark:text-green-400 hover:bg-green-50 dark:hover:bg-green-900/20 transition-colors"
                >
                  <Eye className="w-4 h-4" />
                  <span>Include</span>
                </button>
              ) : (
                <>
                  <button
                    type="button"
                    onClick={() => {
                      onConfigureBranches(repo);
                      setIsOpen(false);
                    }}
                    className="w-full flex items-center gap-2 px-3 py-2 text-sm text-left hover:bg-gray-50 dark:hover:bg-gray-700 transition-colors"
                  >
                    <Settings2 className="w-4 h-4 text-gray-500 dark:text-gray-400" />
                    <span>Branches</span>
                  </button>
                  <button
                    type="button"
                    onClick={() => {
                      onExclude(repo.id);
                      setIsOpen(false);
                    }}
                    className="w-full flex items-center gap-2 px-3 py-2 text-sm text-left text-amber-600 dark:text-amber-400 hover:bg-amber-50 dark:hover:bg-amber-900/20 transition-colors"
                  >
                    <EyeOff className="w-4 h-4" />
                    <span>Exclude</span>
                  </button>
                </>
              )}
              {!readonly && (
                <button
                  type="button"
                  onClick={() => {
                    onDelete(repo.id, repo.name);
                    setIsOpen(false);
                  }}
                  className="w-full flex items-center gap-2 px-3 py-2 text-sm text-left text-red-500 dark:text-red-400 hover:bg-red-50 dark:hover:bg-red-900/20 transition-colors rounded-b-lg"
                >
                  <Trash2 className="w-4 h-4" />
                  <span>Delete</span>
                </button>
              )}
            </>
          )}
        </div>
      )}
    </div>
  );
}

export default function ReposClient() {
  const [reposState, setReposState] = useState({
    repos: [] as Repository[],
    totalCount: 0,
    allReposCount: 0,
    hasMore: false,
    loading: true,
    error: null as string | null,
    syncingIds: new Set<number>(),
    syncResult: null as { id: number; status: string; message: string } | null,
    bulkSyncing: false,
    bulkDeleting: false,
    reposReadOnly: false,
    hideReadOnlyBanner: false,
    settingsLoaded: false,
    branchesModalRepo: null as Repository | null,
    availableBranches: [] as string[],
    selectedIds: new Set<number>(),
    search: "",
    debouncedSearch: "",
    currentPage: 1,
    statusFilter: "",
    connectionFilter: null as number | null,
    connections: [] as Connection[],
    selectAllMatching: false,
    bulkExcluding: false,
    bulkIncluding: false,
    bulkRestoring: false,
  });

  const {
    repos,
    totalCount,
    allReposCount,
    loading,
    error,
    syncingIds,
    syncResult,
    bulkSyncing,
    bulkDeleting,
    reposReadOnly,
    hideReadOnlyBanner,
    settingsLoaded,
    branchesModalRepo,
    availableBranches,
    selectedIds,
    search,
    debouncedSearch,
    currentPage,
    statusFilter,
    connectionFilter,
    connections,
    selectAllMatching,
    bulkExcluding,
    bulkIncluding,
    bulkRestoring,
  } = reposState;

  useEffect(() => {
    const timer = setTimeout(() => {
      setReposState(prev => ({ ...prev, debouncedSearch: search, currentPage: 1 }));
    }, 300);
    return () => clearTimeout(timer);
  }, [search]);

  useEffect(() => {
    const initData = async () => {
      setReposState(prev => ({ ...prev, loading: true }));
      try {
        const [connectionsData, settingsData] = await Promise.all([
          api.listConnections().catch(() => []),
          api.getUISettings().catch(() => null),
        ]);

        let reposReadOnlyVal = false;
        let hideReadOnlyBannerVal = false;

        if (settingsData) {
          reposReadOnlyVal = settingsData.repos_readonly;
          hideReadOnlyBannerVal = settingsData.hide_readonly_banner;
        } else {
          const status = await api.getReposStatus().catch(() => null);
          if (status) reposReadOnlyVal = status.readonly;
        }

        const [reposData, allReposData] = await Promise.all([
          api.listRepos({
            connectionId: connectionFilter || undefined,
            search: debouncedSearch || undefined,
            status: statusFilter || undefined,
            limit: PAGE_SIZE,
            offset: (currentPage - 1) * PAGE_SIZE,
          }),
          (debouncedSearch || statusFilter || connectionFilter)
            ? api.listRepos({ limit: 1, offset: 0 })
            : null,
        ]);

        setReposState(prev => ({
          ...prev,
          connections: connectionsData,
          reposReadOnly: reposReadOnlyVal,
          hideReadOnlyBanner: hideReadOnlyBannerVal,
          settingsLoaded: true,
          repos: reposData.repos || [],
          totalCount: reposData.total_count,
          hasMore: reposData.has_more,
          allReposCount: allReposData ? allReposData.total_count : reposData.total_count,
          error: null,
          loading: false,
        }));
      } catch (err) {
        setReposState(prev => ({
          ...prev,
          error: err instanceof Error ? err.message : "Failed to load initial data",
          loading: false,
          settingsLoaded: true
        }));
      }
    };

    initData();
  }, [debouncedSearch, statusFilter, connectionFilter, currentPage]);

  const loadRepos = useCallback(async (silent = false) => {
    try {
      if (!silent) setReposState(prev => ({ ...prev, loading: true }));
      const [data, allData] = await Promise.all([
        api.listRepos({
          connectionId: connectionFilter || undefined,
          search: debouncedSearch || undefined,
          status: statusFilter || undefined,
          limit: PAGE_SIZE,
          offset: (currentPage - 1) * PAGE_SIZE,
        }),
        (debouncedSearch || statusFilter || connectionFilter)
          ? api.listRepos({ limit: 1, offset: 0 })
          : null,
      ]);
      
      setReposState(prev => ({
        ...prev,
        repos: data.repos || [],
        totalCount: data.total_count,
        hasMore: data.has_more,
        allReposCount: allData ? allData.total_count : data.total_count,
        error: null,
        loading: false,
      }));
    } catch (err) {
      if (!silent) {
        setReposState(prev => ({
          ...prev,
          error: err instanceof Error ? err.message : "Failed to load repositories",
          loading: false
        }));
      }
    }
  }, [debouncedSearch, statusFilter, connectionFilter, currentPage]);

  useEffect(() => {
    const hasActiveRepos = repos.some(r =>
      r.status === "pending" || r.status === "indexing" || r.status === "cloning"
    );
    if (!hasActiveRepos) return;

    const interval = setInterval(() => loadRepos(true), 3000);
    return () => clearInterval(interval);
  }, [repos, loadRepos]);

  useEffect(() => {
    setReposState(prev => ({ ...prev, selectedIds: new Set() }));
  }, [debouncedSearch, statusFilter, currentPage]);

  useEffect(() => {
    if (syncResult?.status === "ok") {
      const timer = setTimeout(() => setReposState(prev => ({ ...prev, syncResult: null })), 5000);
      return () => clearTimeout(timer);
    }
  }, [syncResult]);

  const totalPages = Math.ceil(totalCount / PAGE_SIZE);

  const handleSync = async (id: number) => {
    if (syncingIds.has(id)) return;
    const repo = repos.find(r => r.id === id);
    if (repo && (repo.status === "pending" || repo.status === "cloning" || repo.status === "indexing")) {
      return; 
    }

    setReposState(prev => ({
      ...prev,
      repos: prev.repos.map(r => r.id === id ? { ...r, status: "pending" as const } : r),
      syncingIds: new Set(prev.syncingIds).add(id),
      syncResult: {
        id,
        status: "info",
        message: "Syncing repository...",
      }
    }));

    try {
      await api.syncRepo(id);
      setReposState(prev => ({
        ...prev,
        syncResult: {
          id,
          status: "ok",
          message: "Sync started! Repository will be indexed shortly.",
        }
      }));
    } catch (err) {
      setReposState(prev => ({
        ...prev,
        syncResult: {
          id,
          status: "error",
          message: err instanceof Error ? err.message : "Failed to sync repository",
        }
      }));
      loadRepos();
    } finally {
      setReposState(prev => {
        const nextSyncing = new Set(prev.syncingIds);
        nextSyncing.delete(id);
        return { ...prev, syncingIds: nextSyncing };
      });
    }
  };

  const handleDelete = async (id: number, name: string) => {
    if (reposReadOnly) {
      setReposState(prev => ({ ...prev, error: "Repositories are read-only. Use exclude to hide repos from sync." }));
      return;
    }
    if (!confirm(`Are you sure you want to delete ${name}?`)) return;
    try {
      await api.deleteRepo(id);
      setReposState(prev => {
        const nextSelected = new Set(prev.selectedIds);
        nextSelected.delete(id);
        return { ...prev, selectedIds: nextSelected };
      });
      loadRepos();
    } catch (err) {
      setReposState(prev => ({ ...prev, error: err instanceof Error ? err.message : "Failed to delete repository" }));
    }
  };

  const handleExclude = async (id: number) => {
    try {
      await api.excludeRepo(id);
      setReposState(prev => ({
        ...prev,
        syncResult: {
          id,
          status: "ok",
          message: "Repository excluded from sync and indexing",
        }
      }));
      loadRepos();
    } catch (err) {
      setReposState(prev => ({ ...prev, error: err instanceof Error ? err.message : "Failed to exclude repository" }));
    }
  };

  const handleInclude = async (id: number) => {
    try {
      await api.includeRepo(id);
      setReposState(prev => ({
        ...prev,
        syncResult: {
          id,
          status: "ok",
          message: "Repository included - will be indexed on next sync",
        }
      }));
      loadRepos();
    } catch (err) {
      setReposState(prev => ({ ...prev, error: err instanceof Error ? err.message : "Failed to include repository" }));
    }
  };

  const handleRestore = async (id: number) => {
    try {
      await api.restoreRepo(id);
      setReposState(prev => ({
        ...prev,
        syncResult: {
          id,
          status: "ok",
          message: "Repository restored - will be indexed shortly",
        }
      }));
      loadRepos();
    } catch (err) {
      setReposState(prev => ({ ...prev, error: err instanceof Error ? err.message : "Failed to restore repository" }));
    }
  };

  const handleConfigureBranches = async (repo: Repository) => {
    try {
      const refs = await api.getRefs(repo.id);
      setReposState(prev => ({ ...prev, availableBranches: refs.branches || [], branchesModalRepo: repo }));
    } catch (err) {
      setReposState(prev => ({ ...prev, error: err instanceof Error ? err.message : "Failed to fetch branches" }));
    }
  };

  const handleSaveBranches = async (branches: string[]) => {
    if (!branchesModalRepo) return;
    try {
      await api.setRepoBranches(branchesModalRepo.id, branches);
      setReposState(prev => ({
        ...prev,
        branchesModalRepo: null,
        syncResult: {
          id: branchesModalRepo.id,
          status: "ok",
          message: `Branches updated. Starting sync to index ${branches.length} branch(es)...`,
        }
      }));
      await api.syncRepo(branchesModalRepo.id);
      loadRepos();
    } catch (err) {
      setReposState(prev => ({ ...prev, error: err instanceof Error ? err.message : "Failed to update branches" }));
    }
  };

  const toggleSelect = (id: number) => {
    setReposState(prev => {
      const next = new Set(prev.selectedIds);
      if (next.has(id)) {
        next.delete(id);
      } else {
        next.add(id);
      }
      return { ...prev, selectedIds: next, selectAllMatching: false };
    });
  };

  const toggleSelectAll = () => {
    setReposState(prev => ({
      ...prev,
      selectAllMatching: false,
      selectedIds: prev.selectedIds.size === prev.repos.length ? new Set() : new Set(prev.repos.map(r => r.id))
    }));
  };

  const isAllSelected = repos.length > 0 && selectedIds.size === repos.length;
  const isSomeSelected = selectedIds.size > 0;

  useEffect(() => {
    if (!selectAllMatching) {
      setReposState(prev => ({ ...prev, selectedIds: new Set() }));
    }
  }, [currentPage, selectAllMatching]);

  const getAllMatchingIds = async (): Promise<number[]> => {
    const result = await api.listRepos({
      connectionId: connectionFilter || undefined,
      search: debouncedSearch || undefined,
      status: statusFilter || undefined,
      limit: 10000, 
      offset: 0,
    });
    return result.repos.map(r => r.id);
  };

  const handleBulkSync = async () => {
    if (selectedIds.size === 0 && !selectAllMatching) return;
    setReposState(prev => ({ ...prev, bulkSyncing: true }));

    try {
      const idsToSync = selectAllMatching
        ? await getAllMatchingIds()
        : Array.from(selectedIds);

      const count = idsToSync.length;
      setReposState(prev => ({
        ...prev,
        syncResult: {
          id: -1,
          status: "info",
          message: `Syncing ${count} repositories...`,
        },
        repos: prev.repos.map(repo => idsToSync.includes(repo.id) ? { ...repo, status: "pending" as const } : repo)
      }));

      await Promise.all(idsToSync.map(id => api.syncRepo(id)));
      setReposState(prev => ({
        ...prev,
        syncResult: {
          id: -1,
          status: "ok",
          message: `Sync started for ${count} repositories! They will be indexed shortly.`,
        },
        selectedIds: new Set(),
        selectAllMatching: false
      }));
    } catch (err) {
      setReposState(prev => ({
        ...prev,
        syncResult: {
          id: -1,
          status: "error",
          message: err instanceof Error ? err.message : "Failed to sync repositories",
        }
      }));
      loadRepos();
    } finally {
      setReposState(prev => ({ ...prev, bulkSyncing: false }));
    }
  };

  const handleBulkDelete = async () => {
    if (selectedIds.size === 0 && !selectAllMatching) return;

    const idsToDelete = selectAllMatching
      ? await getAllMatchingIds()
      : Array.from(selectedIds);

    const count = idsToDelete.length;
    if (!confirm(`Are you sure you want to delete ${count} repositories?`)) return;

    setReposState(prev => ({ ...prev, bulkDeleting: true }));
    try {
      await Promise.all(idsToDelete.map(id => api.deleteRepo(id)));
      setReposState(prev => ({ ...prev, selectedIds: new Set(), selectAllMatching: false }));
      loadRepos();
    } catch (err) {
      setReposState(prev => ({ ...prev, error: err instanceof Error ? err.message : "Failed to delete repositories" }));
    } finally {
      setReposState(prev => ({ ...prev, bulkDeleting: false }));
    }
  };

  const handleBulkExclude = async () => {
    if (selectedIds.size === 0 && !selectAllMatching) return;

    const idsToExclude = selectAllMatching
      ? await getAllMatchingIds()
      : Array.from(selectedIds);

    const count = idsToExclude.length;
    if (!confirm(`Are you sure you want to exclude ${count} repositories from sync and indexing?`)) return;

    setReposState(prev => ({ ...prev, bulkExcluding: true }));
    try {
      await Promise.all(idsToExclude.map(id => api.excludeRepo(id)));
      setReposState(prev => ({
        ...prev,
        syncResult: {
          id: -1,
          status: "ok",
          message: `${count} repositories excluded from sync and indexing`,
        },
        selectedIds: new Set(),
        selectAllMatching: false
      }));
      loadRepos();
    } catch (err) {
      setReposState(prev => ({ ...prev, error: err instanceof Error ? err.message : "Failed to exclude repositories" }));
    } finally {
      setReposState(prev => ({ ...prev, bulkExcluding: false }));
    }
  };

  const handleBulkInclude = async () => {
    if (selectedIds.size === 0 && !selectAllMatching) return;

    const idsToInclude = selectAllMatching
      ? await getAllMatchingIds()
      : Array.from(selectedIds);

    const count = idsToInclude.length;
    if (!confirm(`Are you sure you want to include ${count} repositories for sync and indexing?`)) return;

    setReposState(prev => ({ ...prev, bulkIncluding: true }));
    try {
      await Promise.all(idsToInclude.map(id => api.includeRepo(id)));
      setReposState(prev => ({
        ...prev,
        syncResult: {
          id: -1,
          status: "ok",
          message: `${count} repositories included - they will be indexed on next sync`,
        },
        selectedIds: new Set(),
        selectAllMatching: false
      }));
      loadRepos();
    } catch (err) {
      setReposState(prev => ({ ...prev, error: err instanceof Error ? err.message : "Failed to include repositories" }));
    } finally {
      setReposState(prev => ({ ...prev, bulkIncluding: false }));
    }
  };

  const handleBulkRestore = async () => {
    if (selectedIds.size === 0 && !selectAllMatching) return;

    const idsToRestore = selectAllMatching
      ? await getAllMatchingIds()
      : Array.from(selectedIds);

    const count = idsToRestore.length;
    if (!confirm(`Are you sure you want to restore ${count} deleted repositories?`)) return;

    setReposState(prev => ({ ...prev, bulkRestoring: true }));
    try {
      await Promise.all(idsToRestore.map(id => api.restoreRepo(id)));
      setReposState(prev => ({
        ...prev,
        syncResult: {
          id: -1,
          status: "ok",
          message: `${count} repositories restored - they will be indexed shortly`,
        },
        selectedIds: new Set(),
        selectAllMatching: false
      }));
      loadRepos();
    } catch (err) {
      setReposState(prev => ({ ...prev, error: err instanceof Error ? err.message : "Failed to restore repositories" }));
    } finally {
      setReposState(prev => ({ ...prev, bulkRestoring: false }));
    }
  };

  const getStatusBadge = (status: string) => {
    const configs: Record<string, { bg: string; icon: React.ReactNode }> = {
      indexed: {
        bg: "bg-green-100 text-green-800 dark:bg-green-900/50 dark:text-green-300",
        icon: <CheckCircle2 className="w-3.5 h-3.5" />,
      },
      pending: {
        bg: "bg-yellow-100 text-yellow-800 dark:bg-yellow-900/50 dark:text-yellow-300",
        icon: <Clock className="w-3.5 h-3.5" />,
      },
      cloning: {
        bg: "bg-purple-100 text-purple-800 dark:bg-purple-900/50 dark:text-purple-300",
        icon: <Loader2 className="w-3.5 h-3.5 animate-spin" />,
      },
      indexing: {
        bg: "bg-blue-100 text-blue-800 dark:bg-blue-900/50 dark:text-blue-300",
        icon: <Loader2 className="w-3.5 h-3.5 animate-spin" />,
      },
      failed: {
        bg: "bg-red-100 text-red-800 dark:bg-red-900/50 dark:text-red-300",
        icon: <AlertCircle className="w-3.5 h-3.5" />,
      },
      excluded: {
        bg: "bg-gray-200 text-gray-600 dark:bg-gray-700 dark:text-gray-400",
        icon: <EyeOff className="w-3.5 h-3.5" />,
      },
      deleted: {
        bg: "bg-red-200 text-red-700 dark:bg-red-900/50 dark:text-red-400",
        icon: <Trash2 className="w-3.5 h-3.5" />,
      },
    };
    const config = configs[status] || {
      bg: "bg-gray-100 text-gray-800 dark:bg-gray-900/50 dark:text-gray-300",
      icon: <Clock className="w-3.5 h-3.5" />,
    };
    return (
      <span className={`inline-flex items-center gap-1.5 px-2.5 py-1 text-xs font-medium rounded-full ${config.bg}`}>
        {config.icon}
        {status}
      </span>
    );
  };

  const statuses = ["", "indexed", "pending", "cloning", "indexing", "failed", "excluded", "deleted"];

  return (
    <div className="h-full overflow-auto">
      <div className="container mx-auto px-4 py-6 max-w-full">
        <div className="max-w-6xl mx-auto">
          <div className="flex items-center justify-between mb-4 sm:mb-6">
            <div className="flex items-center gap-2 sm:gap-3">
              <FolderGit2 className="w-5 h-5 sm:w-6 sm:h-6 text-gray-600 dark:text-gray-400" />
              <h1 className="text-xl sm:text-2xl font-bold">Repositories</h1>
            </div>
            <button
              type="button"
              onClick={() => loadRepos()}
              className="flex items-center gap-2 px-2.5 sm:px-3 py-1.5 sm:py-2 text-sm font-medium bg-white hover:bg-gray-50 dark:bg-gray-800 dark:hover:bg-gray-700 border border-gray-200 dark:border-gray-700 rounded-lg transition-all shadow-sm focus:outline-none focus:ring-2 focus:ring-blue-500/20 focus:ring-offset-2"
            >
              <RefreshCw className="w-4 h-4 text-gray-500 dark:text-gray-400" />
              <span className="hidden sm:inline">Refresh</span>
            </button>
          </div>

          {reposReadOnly && !hideReadOnlyBanner && (
            <div className="mb-4 p-3 sm:p-4 bg-amber-50 dark:bg-amber-900/20 border border-amber-200 dark:border-amber-800 text-amber-700 dark:text-amber-400 rounded-lg">
              <div className="flex items-start gap-3">
                <AlertCircle className="w-5 h-5 sm:w-5 sm:h-5 flex-shrink-0 mt-0.5" />
                <div>
                  <p className="text-sm font-bold">
                    Read-only mode enabled
                  </p>
                  <p className="text-sm text-amber-700 dark:text-amber-300 mt-1">
                    Repositories are managed via sync.
                    Delete is disabled, but you can exclude repos from sync and indexing.
                  </p>
                </div>
              </div>
            </div>
          )}

          {error && (
            <div className="mb-4 p-3 sm:p-4 bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-xl text-sm text-red-700 dark:text-red-400 flex items-center gap-2 sm:gap-3">
              <AlertCircle className="w-4 h-4 sm:w-5 sm:h-5 flex-shrink-0" />
              {error}
            </div>
          )}

          {syncResult && (
            <div
              className={`mb-4 p-3 sm:p-4 rounded-xl flex items-center gap-2 sm:gap-3 text-sm ${syncResult.status === "ok"
                ? "bg-green-50 dark:bg-green-900/20 border border-green-200 dark:border-green-800 text-green-700 dark:text-green-400"
                : syncResult.status === "info"
                  ? "bg-blue-50 dark:bg-blue-900/20 border border-blue-200 dark:border-blue-800 text-blue-700 dark:text-blue-400"
                  : "bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 text-red-700 dark:text-red-400"
                }`}
            >
              {syncResult.status === "ok" ? (
                <CheckCircle2 className="w-4 h-4 sm:w-5 sm:h-5 flex-shrink-0" />
              ) : syncResult.status === "info" ? (
                <Loader2 className="w-4 h-4 sm:w-5 sm:h-5 flex-shrink-0 animate-spin" />
              ) : (
                <AlertCircle className="w-4 h-4 sm:w-5 sm:h-5 flex-shrink-0" />
              )}
              <span className="flex-1">{syncResult.message}</span>
              <button
                type="button"
                onClick={() => setReposState(prev => ({ ...prev, syncResult: null }))}
                className="p-1 hover:bg-black/10 dark:hover:bg-white/10 rounded transition-colors"
                aria-label="Dismiss message"
              >
                <X className="w-4 h-4" />
              </button>
            </div>
          )}

          <div className="mb-5 flex flex-col sm:flex-row gap-3 sm:gap-4 sm:items-center">
            <div className="flex-1 relative">
              <label htmlFor="repo-search" className="sr-only">Search repositories</label>
              <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-gray-400" />
              <input
                id="repo-search"
                type="text"
                value={search}
                onChange={(e) => setReposState(prev => ({ ...prev, search: e.target.value }))}
                placeholder="Search by repository..."
                className="w-full pl-9 pr-3 py-2 text-sm border border-gray-200 dark:border-gray-700 rounded-lg bg-white dark:bg-gray-800 focus:outline-none focus:border-blue-500 dark:focus:border-blue-500 shadow-sm"
              />
            </div>
            <div className="flex items-center gap-2 flex-shrink-0 flex-wrap sm:flex-nowrap">
              <div className="flex items-center gap-2 flex-1 sm:flex-initial">
                <Filter className="w-4 h-4 text-gray-400 hidden sm:block" />
                <label htmlFor="connection-filter" className="sr-only">Filter by connection</label>
                <select
                  id="connection-filter"
                  value={connectionFilter || ""}
                  onChange={(e) => setReposState(prev => ({
                    ...prev,
                    connectionFilter: e.target.value ? Number(e.target.value) : null,
                    currentPage: 1
                  }))}
                  className="w-[120px] sm:w-[155px] pl-2 sm:pl-3 pr-7 sm:pr-8 py-2 text-xs sm:text-sm border border-gray-200 dark:border-gray-700 rounded-lg bg-white dark:bg-gray-800 focus:outline-none focus:border-blue-500 dark:focus:border-blue-500 shadow-sm appearance-none bg-no-repeat bg-[length:16px_16px] bg-[position:right_8px_center] bg-[url('data:image/svg+xml;charset=UTF-8,%3Csvg%20xmlns%3D%22http%3A%2F%2Fwww.w3.org%2F2000%2Fsvg%22%20viewBox%3D%220%200%2020%2020%22%20fill%3D%22%236b7280%22%3E%3Cpath%20fill-rule%3D%22evenodd%22%20d%3D%22M5.293%207.293a1%201%200%20011.414%200L10%2010.586l3.293-3.293a1%201%200%20111.414%201.414l-4%204a1%201%200%2001-1.414%200l-4-4a1%201%200%20010-1.414z%22%20clip-rule%3D%22evenodd%22%2F%3E%3C%2Fsvg%3E')]"
                >
                  <option value="">All connections</option>
                  {connections.map((conn) => (
                    <option key={conn.id} value={conn.id}>
                      {conn.name}
                    </option>
                  ))}
                </select>
                <label htmlFor="status-filter" className="sr-only">Filter by status</label>
                <select
                  id="status-filter"
                  value={statusFilter}
                  onChange={(e) => setReposState(prev => ({
                    ...prev,
                    statusFilter: e.target.value,
                    currentPage: 1
                  }))}
                  className="w-[105px] sm:w-[130px] pl-2 sm:pl-3 pr-7 sm:pr-8 py-2 text-xs sm:text-sm border border-gray-200 dark:border-gray-700 rounded-lg bg-white dark:bg-gray-800 focus:outline-none focus:border-blue-500 dark:focus:border-blue-500 shadow-sm appearance-none bg-no-repeat bg-[length:16px_16px] bg-[position:right_8px_center] bg-[url('data:image/svg+xml;charset=UTF-8,%3Csvg%20xmlns%3D%22http%3A%2F%2Fwww.w3.org%2F2000%2Fsvg%22%20viewBox%3D%220%200%2020%2020%22%20fill%3D%22%236b7280%22%3E%3Cpath%20fill-rule%3D%22evenodd%22%20d%3D%22M5.293%207.293a1%201%200%20011.414%200L10%2010.586l3.293-3.293a1%201%200%20111.414%201.414l-4%204a1%201%200%2001-1.414%200l-4-4a1%201%200%20010-1.414z%22%20clip-rule%3D%22evenodd%22%2F%3E%3C%2Fsvg%3E')]"
                >
                  {statuses.map((status) => (
                    <option key={status} value={status}>
                      {status === "" ? "All statuses" : status.charAt(0).toUpperCase() + status.slice(1)}
                    </option>
                  ))}
                </select>
              </div>
              <div className="text-xs sm:text-sm text-gray-500 dark:text-gray-400 bg-gray-100 dark:bg-gray-800 px-2 sm:px-3 py-1.5 rounded-lg whitespace-nowrap">
                {totalCount}/{allReposCount}
              </div>
            </div>
          </div>

          {(isSomeSelected || selectAllMatching) && (
            <div className="mb-4 p-3 bg-blue-50 dark:bg-blue-900/20 border border-blue-200 dark:border-blue-800 rounded-lg">
              <div className="flex flex-wrap items-center gap-2 sm:gap-3">
                <div className="flex items-center gap-2">
                  <span className="text-sm font-medium text-blue-700 dark:text-blue-300">
                    {selectAllMatching ? (
                      <>All {totalCount} repos matching filter</>
                    ) : (
                      <>{selectedIds.size} selected</>
                    )}
                  </span>
                </div>

                {isAllSelected && !selectAllMatching && totalCount > repos.length && (
                  <button
                    type="button"
                    onClick={() => setReposState(prev => ({ ...prev, selectAllMatching: true }))}
                    className="text-sm text-blue-600 dark:text-blue-400 hover:underline font-medium"
                  >
                    Select all {totalCount} repos matching this filter
                  </button>
                )}

                {selectAllMatching && (
                  <button
                    type="button"
                    onClick={() => {
                      setReposState(prev => ({ ...prev, selectAllMatching: false, selectedIds: new Set() }));
                    }}
                    className="text-sm text-blue-600 dark:text-blue-400 hover:underline font-medium"
                  >
                    Clear selection
                  </button>
                )}

                <div className="flex-1" />
                <div className="flex items-center gap-2">
                  <button
                    type="button"
                    onClick={handleBulkSync}
                    disabled={bulkSyncing}
                    className="flex items-center gap-1.5 px-2 sm:px-3 py-1.5 text-sm font-medium border border-blue-500 dark:border-blue-400 text-blue-600 dark:text-blue-400 hover:bg-blue-50 dark:hover:bg-blue-900/30 rounded-lg disabled:opacity-50 transition-all focus:outline-none focus:ring-2 focus:ring-blue-500/20 focus:ring-offset-2"
                  >
                    {bulkSyncing ? (
                      <Loader2 className="w-4 h-4 animate-spin" />
                    ) : (
                      <RefreshCw className="w-4 h-4" />
                    )}
                    <span className="hidden sm:inline">{bulkSyncing ? "Syncing..." : "Sync"}</span>
                  </button>
                  <button
                    type="button"
                    onClick={handleBulkExclude}
                    disabled={bulkExcluding}
                    className="flex items-center gap-1.5 px-2 sm:px-3 py-1.5 text-sm font-medium border border-amber-500 dark:border-amber-400 text-amber-600 dark:text-amber-400 hover:bg-amber-50 dark:hover:bg-amber-900/30 rounded-lg disabled:opacity-50 transition-all focus:outline-none focus:ring-2 focus:ring-amber-500/20 focus:ring-offset-2"
                  >
                    {bulkExcluding ? (
                      <Loader2 className="w-4 h-4 animate-spin" />
                    ) : (
                      <EyeOff className="w-4 h-4" />
                    )}
                    <span className="hidden sm:inline">{bulkExcluding ? "Excluding..." : "Exclude"}</span>
                  </button>
                  <button
                    type="button"
                    onClick={handleBulkInclude}
                    disabled={bulkIncluding}
                    className="flex items-center gap-1.5 px-2 sm:px-3 py-1.5 text-sm font-medium border border-green-500 dark:border-green-400 text-green-600 dark:text-green-400 hover:bg-green-50 dark:hover:bg-green-900/30 rounded-lg disabled:opacity-50 transition-all focus:outline-none focus:ring-2 focus:ring-green-500/20 focus:ring-offset-2"
                  >
                    {bulkIncluding ? (
                      <Loader2 className="w-4 h-4 animate-spin" />
                    ) : (
                      <Eye className="w-4 h-4" />
                    )}
                    <span className="hidden sm:inline">{bulkIncluding ? "Including..." : "Include"}</span>
                  </button>
                  <button
                    type="button"
                    onClick={handleBulkRestore}
                    disabled={bulkRestoring}
                    className="flex items-center gap-1.5 px-2 sm:px-3 py-1.5 text-sm font-medium border border-green-500 dark:border-green-400 text-green-600 dark:text-green-400 hover:bg-green-50 dark:hover:bg-green-900/20 rounded-lg disabled:opacity-50 transition-all focus:outline-none focus:ring-2 focus:ring-green-500/20 focus:ring-offset-2"
                  >
                    {bulkRestoring ? (
                      <Loader2 className="w-4 h-4 animate-spin" />
                    ) : (
                      <RotateCcw className="w-4 h-4" />
                    )}
                    <span className="hidden sm:inline">{bulkRestoring ? "Restoring..." : "Restore"}</span>
                  </button>
                  {settingsLoaded && !reposReadOnly && (
                    <button
                      type="button"
                      onClick={handleBulkDelete}
                      disabled={bulkDeleting}
                      className="flex items-center gap-1.5 px-2 sm:px-3 py-1.5 text-sm font-medium border border-red-500 dark:border-red-400 text-red-600 dark:text-red-400 hover:bg-red-50 dark:hover:bg-red-900/30 rounded-lg disabled:opacity-50 transition-all focus:outline-none focus:ring-2 focus:ring-red-500/20 focus:ring-offset-2"
                    >
                      {bulkDeleting ? (
                        <Loader2 className="w-4 h-4 animate-spin" />
                      ) : (
                        <Trash2 className="w-4 h-4" />
                      )}
                      <span className="hidden sm:inline">{bulkDeleting ? "Deleting..." : "Delete"}</span>
                    </button>
                  )}
                  <button
                    type="button"
                    onClick={() => {
                      setReposState(prev => ({ ...prev, selectedIds: new Set(), selectAllMatching: false }));
                    }}
                    className="flex items-center gap-1.5 px-2 sm:px-3 py-1.5 text-sm font-medium border border-gray-300 dark:border-gray-600 text-gray-600 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 rounded-lg transition-all focus:outline-none focus:ring-2 focus:ring-gray-500/20 focus:ring-offset-2"
                  >
                    <X className="w-4 h-4" />
                    <span className="hidden sm:inline">Clear</span>
                  </button>
                </div>
              </div>
            </div>
          )}

          {loading ? (
            <div className="text-center py-16">
              <Loader2 className="w-8 h-8 animate-spin text-blue-600 mx-auto" />
              <p className="mt-3 text-gray-500 dark:text-gray-400">Loading repositories...</p>
            </div>
          ) : totalCount === 0 && !debouncedSearch && !statusFilter ? (
            <div className="text-center py-16 bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700">
              <FolderGit2 className="w-12 h-12 text-gray-300 dark:text-gray-600 mx-auto mb-4" />
              <p className="text-gray-500 dark:text-gray-400 mb-2">No repositories found</p>
              <p className="text-sm text-gray-400 dark:text-gray-500">
                Add a connection to start indexing repositories
              </p>
            </div>
          ) : repos.length === 0 ? (
            <div className="text-center py-16 bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700">
              <Search className="w-12 h-12 text-gray-300 dark:text-gray-600 mx-auto mb-4" />
              <p className="text-gray-500 dark:text-gray-400 mb-3">No repositories match your search</p>
              <button
                onClick={() => setReposState(prev => ({ ...prev, search: "", statusFilter: "" }))}
                className="text-blue-600 dark:text-blue-400 hover:underline text-sm font-medium"
              >
                Clear filters
              </button>
            </div>
          ) : (
            <>
              <div className="bg-white dark:bg-gray-800 rounded-xl shadow-sm border border-gray-200 dark:border-gray-700">
                <div className="w-full">
                  <table className="w-full divide-y divide-gray-200 dark:divide-gray-700 table-fixed">
                    <thead className="bg-gray-50 dark:bg-gray-800/80">
                      <tr>
                        <th className="w-10 px-3 py-3.5 text-center">
                          <input
                            type="checkbox"
                            checked={isAllSelected}
                            onChange={toggleSelectAll}
                            aria-label="Select all repositories"
                            className="w-4 h-4 rounded border-gray-300 text-blue-600 focus:ring-blue-500 dark:border-gray-600 dark:bg-gray-700"
                          />
                        </th>
                        <th className="px-4 py-3.5 text-left text-xs font-semibold text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                          Repository
                        </th>
                        <th className="w-20 sm:w-[100px] px-2 sm:px-4 py-3.5 text-left text-xs font-semibold text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                          Status
                        </th>
                        <th className="hidden md:table-cell w-[120px] px-4 py-3.5 text-left text-xs font-semibold text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                          Branch
                        </th>
                        <th className="hidden sm:table-cell w-[160px] px-4 py-3.5 text-left text-xs font-semibold text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                          Last Indexed
                        </th>
                        <th className="w-10 px-2 py-3.5">
                          <span className="sr-only">Actions</span>
                        </th>
                      </tr>
                    </thead>
                    <tbody className="divide-y divide-gray-100 dark:divide-gray-700/50">
                      {repos.map((repo) => (
                        <tr
                          key={repo.id}
                          className={`hover:bg-gray-50 dark:hover:bg-gray-700/30 transition-colors ${selectedIds.has(repo.id) ? "bg-blue-50 dark:bg-blue-900/10" : ""
                            }`}
                        >
                          <td className="w-10 px-3 py-4 text-center">
                            <input
                              type="checkbox"
                              checked={selectedIds.has(repo.id)}
                              onChange={() => toggleSelect(repo.id)}
                              aria-label={`Select ${repo.name}`}
                              className="w-4 h-4 rounded border-gray-300 text-blue-600 focus:ring-blue-500 dark:border-gray-600 dark:bg-gray-700"
                            />
                          </td>
                          <td className="px-4 py-4 overflow-hidden">
                            <div className="flex items-center gap-3 min-w-0">
                              <GitBranch className="w-5 h-5 text-gray-400 flex-shrink-0 hidden sm:block" />
                              <div className="min-w-0 flex-1">
                                <div className={`text-sm font-medium truncate ${repo.deleted ? "text-red-400 dark:text-red-500 line-through" : repo.excluded ? "text-gray-400 dark:text-gray-500" : ""}`}>{repo.name}</div>
                                <div className="text-xs text-gray-500 dark:text-gray-400 truncate">
                                  {repo.clone_url}
                                </div>
                              </div>
                            </div>
                          </td>
                          <td className="px-2 sm:px-4 py-4 whitespace-nowrap">
                            {getStatusBadge(repo.deleted ? "deleted" : repo.excluded ? "excluded" : repo.status)}
                          </td>
                          <td className="hidden md:table-cell px-4 py-4 text-sm text-gray-500 dark:text-gray-400 truncate">
                            {repo.branches?.length > 0
                              ? repo.branches.join(", ")
                              : repo.default_branch || "-"}
                          </td>
                          <td className="hidden sm:table-cell px-4 py-4 text-sm text-gray-500 dark:text-gray-400 whitespace-nowrap">
                            {repo.last_indexed
                              ? new Date(repo.last_indexed).toLocaleString()
                              : "Never"}
                          </td>
                          <td className="w-12 px-2 pr-4 py-4">
                            <ActionMenu
                              repo={repo}
                              onSync={handleSync}
                              onDelete={handleDelete}
                              onExclude={handleExclude}
                              onInclude={handleInclude}
                              onRestore={handleRestore}
                              onConfigureBranches={handleConfigureBranches}
                              syncingIds={syncingIds}
                              readonly={reposReadOnly}
                            />
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              </div>

              {totalPages > 1 && (
                <div className="mt-6 flex flex-col sm:flex-row items-center justify-between gap-3">
                  <div className="text-sm text-gray-500 dark:text-gray-400 text-center sm:text-left">
                    {(currentPage - 1) * PAGE_SIZE + 1}-{Math.min(currentPage * PAGE_SIZE, totalCount)} of {totalCount}
                  </div>
                  <nav className="flex items-center gap-1" aria-label="Pagination">
                    <button
                      type="button"
                      onClick={() => setReposState(prev => ({ ...prev, currentPage: 1 }))}
                      disabled={currentPage === 1}
                      className="hidden sm:flex p-2 text-sm border border-gray-200 dark:border-gray-700 rounded-lg hover:bg-gray-100 dark:hover:bg-gray-800 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
                      title="First page"
                    >
                      <ChevronsLeft className="w-4 h-4" />
                    </button>
                    <button
                      type="button"
                      onClick={() => setReposState(prev => ({ ...prev, currentPage: Math.max(1, prev.currentPage - 1) }))}
                      disabled={currentPage === 1}
                      className="p-2 text-sm border border-gray-200 dark:border-gray-700 rounded-lg hover:bg-gray-100 dark:hover:bg-gray-800 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
                      title="Previous page"
                    >
                      <ChevronLeft className="w-4 h-4" />
                    </button>
                    <span className="sm:hidden text-sm text-gray-600 dark:text-gray-400 px-2">
                      {currentPage} / {totalPages}
                    </span>
                    <div className="hidden sm:flex items-center gap-1 mx-1" role="group" aria-label="Page numbers">
                      {Array.from({ length: Math.min(5, totalPages) }, (_, i) => {
                        let pageNum: number;
                        if (totalPages <= 5) {
                          pageNum = i + 1;
                        } else if (currentPage <= 3) {
                          pageNum = i + 1;
                        } else if (currentPage >= totalPages - 2) {
                          pageNum = totalPages - 4 + i;
                        } else {
                          pageNum = currentPage - 2 + i;
                        }
                        return (
                          <button
                            key={pageNum}
                            type="button"
                            onClick={() => setReposState(prev => ({ ...prev, currentPage: pageNum }))}
                            aria-current={currentPage === pageNum ? "page" : undefined}
                            className={`w-9 h-9 text-sm border rounded-lg font-medium transition-colors ${currentPage === pageNum
                              ? "bg-blue-600 text-white border-blue-600 shadow-sm"
                              : "border-gray-200 dark:border-gray-700 hover:bg-gray-100 dark:hover:bg-gray-800"
                              }`}
                          >
                            {pageNum}
                          </button>
                        );
                      })}
                    </div>
                    <button
                      type="button"
                      onClick={() => setReposState(prev => ({ ...prev, currentPage: Math.min(totalPages, prev.currentPage + 1) }))}
                      disabled={currentPage === totalPages}
                      className="p-2 text-sm border border-gray-200 dark:border-gray-700 rounded-lg hover:bg-gray-100 dark:hover:bg-gray-800 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
                      title="Next page"
                    >
                      <ChevronRight className="w-4 h-4" />
                    </button>
                    <button
                      type="button"
                      onClick={() => setReposState(prev => ({ ...prev, currentPage: totalPages }))}
                      disabled={currentPage === totalPages}
                      className="hidden sm:flex p-2 text-sm border border-gray-200 dark:border-gray-700 rounded-lg hover:bg-gray-100 dark:hover:bg-gray-800 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
                      title="Last page"
                    >
                      <ChevronsRight className="w-4 h-4" />
                    </button>
                  </nav>
                </div>
              )}
            </>
          )}
        </div>
      </div>

      {branchesModalRepo && (
        <BranchesModal
          repo={branchesModalRepo}
          availableBranches={availableBranches}
          onClose={() => setReposState(prev => ({ ...prev, branchesModalRepo: null }))}
          onSave={handleSaveBranches}
        />
      )}
    </div>
  );
}
