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
    setSelectedBranches((prev) =>
      prev.includes(branch)
        ? prev.filter((b) => b !== branch)
        : [...prev, branch]
    );
  };

  const handleAddCustomBranch = () => {
    if (newBranch.trim() && !selectedBranches.includes(newBranch.trim())) {
      setSelectedBranches((prev) => [...prev, newBranch.trim()]);
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
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4">
      <div className="max-h-[80vh] w-full max-w-md overflow-hidden rounded-xl bg-white shadow-xl dark:bg-gray-800">
        <div className="border-b border-gray-200 px-6 py-4 dark:border-gray-700">
          <div className="flex items-center justify-between">
            <h2 className="text-lg font-semibold">Configure Branches</h2>
            <button
              type="button"
              onClick={onClose}
              className="rounded-lg p-1 transition-colors hover:bg-gray-100 dark:hover:bg-gray-700"
              aria-label="Close modal"
            >
              <X className="h-5 w-5 text-gray-500" />
            </button>
          </div>
          <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">
            Select which branches to index for{" "}
            <span className="font-medium text-gray-700 dark:text-gray-300">
              {repo.name}
            </span>
          </p>
        </div>

        <div className="max-h-[50vh] overflow-y-auto px-6 py-4">
          {/* Available branches from git */}
          {availableBranches.length > 0 && (
            <div className="mb-4">
              <h3 className="mb-2 text-sm font-medium text-gray-700 dark:text-gray-300">
                Available Branches
              </h3>
              <div className="space-y-1">
                {availableBranches.map((branch) => (
                  <label
                    key={branch}
                    className="flex cursor-pointer items-center gap-2 rounded-lg p-2 hover:bg-gray-50 dark:hover:bg-gray-700/50"
                  >
                    <input
                      type="checkbox"
                      checked={selectedBranches.includes(branch)}
                      onChange={() => handleToggleBranch(branch)}
                      className="h-4 w-4 rounded border-gray-300 text-blue-600 focus:ring-blue-500"
                    />
                    <span className="text-sm">
                      {branch}
                      {branch === repo.default_branch && (
                        <span className="ml-2 text-xs text-gray-500">
                          (default)
                        </span>
                      )}
                    </span>
                  </label>
                ))}
              </div>
            </div>
          )}

          {/* Currently selected branches not in available list */}
          {selectedBranches.filter((b) => !availableBranches.includes(b))
            .length > 0 && (
            <div className="mb-4">
              <h3 className="mb-2 text-sm font-medium text-gray-700 dark:text-gray-300">
                Custom Branches
              </h3>
              <div className="space-y-1">
                {selectedBranches
                  .filter((b) => !availableBranches.includes(b))
                  .map((branch) => (
                    <label
                      key={branch}
                      className="flex cursor-pointer items-center gap-2 rounded-lg p-2 hover:bg-gray-50 dark:hover:bg-gray-700/50"
                    >
                      <input
                        type="checkbox"
                        checked={true}
                        onChange={() => handleToggleBranch(branch)}
                        className="h-4 w-4 rounded border-gray-300 text-blue-600 focus:ring-blue-500"
                      />
                      <span className="text-sm">{branch}</span>
                    </label>
                  ))}
              </div>
            </div>
          )}

          {/* Add custom branch */}
          <div>
            <h3 className="mb-2 text-sm font-medium text-gray-700 dark:text-gray-300">
              Add Branch
            </h3>
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
                className="flex-1 rounded-lg border border-gray-200 bg-white px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 dark:border-gray-600 dark:bg-gray-700"
              />
              <button
                type="button"
                onClick={handleAddCustomBranch}
                disabled={!newBranch.trim()}
                className="rounded-lg bg-gray-100 px-3 py-2 transition-colors hover:bg-gray-200 disabled:opacity-50 dark:bg-gray-700 dark:hover:bg-gray-600"
                aria-label="Add custom branch"
              >
                <Plus className="h-4 w-4" />
              </button>
            </div>
          </div>
        </div>

        <div className="flex items-center justify-between border-t border-gray-200 px-6 py-4 dark:border-gray-700">
          <p className="text-xs text-gray-500 dark:text-gray-400">
            {selectedBranches.length} branch
            {selectedBranches.length !== 1 ? "es" : ""} selected
          </p>
          <div className="flex gap-2">
            <button
              type="button"
              onClick={onClose}
              className="rounded-lg px-4 py-2 text-sm font-medium text-gray-700 transition-colors hover:bg-gray-100 dark:text-gray-300 dark:hover:bg-gray-700"
            >
              Cancel
            </button>
            <button
              type="button"
              onClick={handleSave}
              disabled={saving || selectedBranches.length === 0}
              className="flex items-center gap-2 rounded-lg bg-blue-600 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-blue-700 disabled:opacity-50"
            >
              {saving && <Loader2 className="h-4 w-4 animate-spin" />}
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
  readonly,
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
        className="rounded-lg p-2 transition-all hover:bg-gray-100 focus:outline-none focus:ring-2 focus:ring-blue-500/20 dark:hover:bg-gray-700"
      >
        <MoreVertical className="h-4 w-4 text-gray-500" />
      </button>
      {isOpen && (
        <div
          ref={menuRef}
          className="fixed z-50 w-36 rounded-lg border border-gray-200 bg-white shadow-lg dark:border-gray-700 dark:bg-gray-800"
          style={{ top: menuPosition.top, right: menuPosition.right }}
        >
          {repo.deleted ? (
            <button
              type="button"
              onClick={() => {
                onRestore(repo.id);
                setIsOpen(false);
              }}
              className="flex w-full items-center gap-2 rounded-lg px-3 py-2 text-left text-sm text-green-600 transition-colors hover:bg-green-50 dark:text-green-400 dark:hover:bg-green-900/20"
            >
              <RotateCcw className="h-4 w-4" />
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
                  disabled={
                    syncingIds.has(repo.id) ||
                    repo.status === "pending" ||
                    repo.status === "cloning" ||
                    repo.status === "indexing"
                  }
                  className="flex w-full items-center gap-2 rounded-t-lg px-3 py-2 text-left text-sm transition-colors hover:bg-gray-50 disabled:opacity-50 dark:hover:bg-gray-700"
                >
                  {syncingIds.has(repo.id) ||
                  repo.status === "pending" ||
                  repo.status === "cloning" ||
                  repo.status === "indexing" ? (
                    <Loader2 className="h-4 w-4 animate-spin text-gray-500 dark:text-gray-400" />
                  ) : (
                    <RefreshCw className="h-4 w-4 text-gray-500 dark:text-gray-400" />
                  )}
                  <span>
                    {syncingIds.has(repo.id)
                      ? "Syncing..."
                      : repo.status === "pending" ||
                          repo.status === "cloning" ||
                          repo.status === "indexing"
                        ? "In progress..."
                        : "Sync"}
                  </span>
                </button>
              )}
              {repo.excluded ? (
                <button
                  type="button"
                  onClick={() => {
                    onInclude(repo.id);
                    setIsOpen(false);
                  }}
                  className="flex w-full items-center gap-2 px-3 py-2 text-left text-sm text-green-600 transition-colors hover:bg-green-50 dark:text-green-400 dark:hover:bg-green-900/20"
                >
                  <Eye className="h-4 w-4" />
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
                    className="flex w-full items-center gap-2 px-3 py-2 text-left text-sm transition-colors hover:bg-gray-50 dark:hover:bg-gray-700"
                  >
                    <Settings2 className="h-4 w-4 text-gray-500 dark:text-gray-400" />
                    <span>Branches</span>
                  </button>
                  <button
                    type="button"
                    onClick={() => {
                      onExclude(repo.id);
                      setIsOpen(false);
                    }}
                    className="flex w-full items-center gap-2 px-3 py-2 text-left text-sm text-amber-600 transition-colors hover:bg-amber-50 dark:text-amber-400 dark:hover:bg-amber-900/20"
                  >
                    <EyeOff className="h-4 w-4" />
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
                  className="flex w-full items-center gap-2 rounded-b-lg px-3 py-2 text-left text-sm text-red-500 transition-colors hover:bg-red-50 dark:text-red-400 dark:hover:bg-red-900/20"
                >
                  <Trash2 className="h-4 w-4" />
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
      setReposState((prev) => ({
        ...prev,
        debouncedSearch: search,
        currentPage: 1,
      }));
    }, 300);
    return () => clearTimeout(timer);
  }, [search]);

  useEffect(() => {
    const initData = async () => {
      setReposState((prev) => ({ ...prev, loading: true }));
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
          debouncedSearch || statusFilter || connectionFilter
            ? api.listRepos({ limit: 1, offset: 0 })
            : null,
        ]);

        setReposState((prev) => ({
          ...prev,
          connections: connectionsData,
          reposReadOnly: reposReadOnlyVal,
          hideReadOnlyBanner: hideReadOnlyBannerVal,
          settingsLoaded: true,
          repos: reposData.repos || [],
          totalCount: reposData.total_count,
          hasMore: reposData.has_more,
          allReposCount: allReposData
            ? allReposData.total_count
            : reposData.total_count,
          error: null,
          loading: false,
        }));
      } catch (err) {
        setReposState((prev) => ({
          ...prev,
          error:
            err instanceof Error ? err.message : "Failed to load initial data",
          loading: false,
          settingsLoaded: true,
        }));
      }
    };

    initData();
  }, [debouncedSearch, statusFilter, connectionFilter, currentPage]);

  const loadRepos = useCallback(
    async (silent = false) => {
      try {
        if (!silent) setReposState((prev) => ({ ...prev, loading: true }));
        const [data, allData] = await Promise.all([
          api.listRepos({
            connectionId: connectionFilter || undefined,
            search: debouncedSearch || undefined,
            status: statusFilter || undefined,
            limit: PAGE_SIZE,
            offset: (currentPage - 1) * PAGE_SIZE,
          }),
          debouncedSearch || statusFilter || connectionFilter
            ? api.listRepos({ limit: 1, offset: 0 })
            : null,
        ]);

        setReposState((prev) => ({
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
          setReposState((prev) => ({
            ...prev,
            error:
              err instanceof Error
                ? err.message
                : "Failed to load repositories",
            loading: false,
          }));
        }
      }
    },
    [debouncedSearch, statusFilter, connectionFilter, currentPage]
  );

  useEffect(() => {
    const hasActiveRepos = repos.some(
      (r) =>
        r.status === "pending" ||
        r.status === "indexing" ||
        r.status === "cloning"
    );
    if (!hasActiveRepos) return;

    const interval = setInterval(() => loadRepos(true), 3000);
    return () => clearInterval(interval);
  }, [repos, loadRepos]);

  useEffect(() => {
    setReposState((prev) => ({ ...prev, selectedIds: new Set() }));
  }, [debouncedSearch, statusFilter, currentPage]);

  useEffect(() => {
    if (syncResult?.status === "ok") {
      const timer = setTimeout(
        () => setReposState((prev) => ({ ...prev, syncResult: null })),
        5000
      );
      return () => clearTimeout(timer);
    }
  }, [syncResult]);

  const totalPages = Math.ceil(totalCount / PAGE_SIZE);

  const handleSync = async (id: number) => {
    if (syncingIds.has(id)) return;
    const repo = repos.find((r) => r.id === id);
    if (
      repo &&
      (repo.status === "pending" ||
        repo.status === "cloning" ||
        repo.status === "indexing")
    ) {
      return;
    }

    setReposState((prev) => ({
      ...prev,
      repos: prev.repos.map((r) =>
        r.id === id ? { ...r, status: "pending" as const } : r
      ),
      syncingIds: new Set(prev.syncingIds).add(id),
      syncResult: {
        id,
        status: "info",
        message: "Syncing repository...",
      },
    }));

    try {
      await api.syncRepo(id);
      setReposState((prev) => ({
        ...prev,
        syncResult: {
          id,
          status: "ok",
          message: "Sync started! Repository will be indexed shortly.",
        },
      }));
    } catch (err) {
      setReposState((prev) => ({
        ...prev,
        syncResult: {
          id,
          status: "error",
          message:
            err instanceof Error ? err.message : "Failed to sync repository",
        },
      }));
      loadRepos();
    } finally {
      setReposState((prev) => {
        const nextSyncing = new Set(prev.syncingIds);
        nextSyncing.delete(id);
        return { ...prev, syncingIds: nextSyncing };
      });
    }
  };

  const handleDelete = async (id: number, name: string) => {
    if (reposReadOnly) {
      setReposState((prev) => ({
        ...prev,
        error:
          "Repositories are read-only. Use exclude to hide repos from sync.",
      }));
      return;
    }
    if (!confirm(`Are you sure you want to delete ${name}?`)) return;
    try {
      await api.deleteRepo(id);
      setReposState((prev) => {
        const nextSelected = new Set(prev.selectedIds);
        nextSelected.delete(id);
        return { ...prev, selectedIds: nextSelected };
      });
      loadRepos();
    } catch (err) {
      setReposState((prev) => ({
        ...prev,
        error:
          err instanceof Error ? err.message : "Failed to delete repository",
      }));
    }
  };

  const handleExclude = async (id: number) => {
    try {
      await api.excludeRepo(id);
      setReposState((prev) => ({
        ...prev,
        syncResult: {
          id,
          status: "ok",
          message: "Repository excluded from sync and indexing",
        },
      }));
      loadRepos();
    } catch (err) {
      setReposState((prev) => ({
        ...prev,
        error:
          err instanceof Error ? err.message : "Failed to exclude repository",
      }));
    }
  };

  const handleInclude = async (id: number) => {
    try {
      await api.includeRepo(id);
      setReposState((prev) => ({
        ...prev,
        syncResult: {
          id,
          status: "ok",
          message: "Repository included - will be indexed on next sync",
        },
      }));
      loadRepos();
    } catch (err) {
      setReposState((prev) => ({
        ...prev,
        error:
          err instanceof Error ? err.message : "Failed to include repository",
      }));
    }
  };

  const handleRestore = async (id: number) => {
    try {
      await api.restoreRepo(id);
      setReposState((prev) => ({
        ...prev,
        syncResult: {
          id,
          status: "ok",
          message: "Repository restored - will be indexed shortly",
        },
      }));
      loadRepos();
    } catch (err) {
      setReposState((prev) => ({
        ...prev,
        error:
          err instanceof Error ? err.message : "Failed to restore repository",
      }));
    }
  };

  const handleConfigureBranches = async (repo: Repository) => {
    try {
      const refs = await api.getRefs(repo.id);
      setReposState((prev) => ({
        ...prev,
        availableBranches: refs.branches || [],
        branchesModalRepo: repo,
      }));
    } catch (err) {
      setReposState((prev) => ({
        ...prev,
        error: err instanceof Error ? err.message : "Failed to fetch branches",
      }));
    }
  };

  const handleSaveBranches = async (branches: string[]) => {
    if (!branchesModalRepo) return;
    try {
      await api.setRepoBranches(branchesModalRepo.id, branches);
      setReposState((prev) => ({
        ...prev,
        branchesModalRepo: null,
        syncResult: {
          id: branchesModalRepo.id,
          status: "ok",
          message: `Branches updated. Starting sync to index ${branches.length} branch(es)...`,
        },
      }));
      await api.syncRepo(branchesModalRepo.id);
      loadRepos();
    } catch (err) {
      setReposState((prev) => ({
        ...prev,
        error: err instanceof Error ? err.message : "Failed to update branches",
      }));
    }
  };

  const toggleSelect = (id: number) => {
    setReposState((prev) => {
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
    setReposState((prev) => ({
      ...prev,
      selectAllMatching: false,
      selectedIds:
        prev.selectedIds.size === prev.repos.length
          ? new Set()
          : new Set(prev.repos.map((r) => r.id)),
    }));
  };

  const isAllSelected = repos.length > 0 && selectedIds.size === repos.length;
  const isSomeSelected = selectedIds.size > 0;

  useEffect(() => {
    if (!selectAllMatching) {
      setReposState((prev) => ({ ...prev, selectedIds: new Set() }));
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
    return result.repos.map((r) => r.id);
  };

  const handleBulkSync = async () => {
    if (selectedIds.size === 0 && !selectAllMatching) return;
    setReposState((prev) => ({ ...prev, bulkSyncing: true }));

    try {
      const idsToSync = selectAllMatching
        ? await getAllMatchingIds()
        : Array.from(selectedIds);

      const count = idsToSync.length;
      setReposState((prev) => ({
        ...prev,
        syncResult: {
          id: -1,
          status: "info",
          message: `Syncing ${count} repositories...`,
        },
        repos: prev.repos.map((repo) =>
          idsToSync.includes(repo.id)
            ? { ...repo, status: "pending" as const }
            : repo
        ),
      }));

      await Promise.all(idsToSync.map((id) => api.syncRepo(id)));
      setReposState((prev) => ({
        ...prev,
        syncResult: {
          id: -1,
          status: "ok",
          message: `Sync started for ${count} repositories! They will be indexed shortly.`,
        },
        selectedIds: new Set(),
        selectAllMatching: false,
      }));
    } catch (err) {
      setReposState((prev) => ({
        ...prev,
        syncResult: {
          id: -1,
          status: "error",
          message:
            err instanceof Error ? err.message : "Failed to sync repositories",
        },
      }));
      loadRepos();
    } finally {
      setReposState((prev) => ({ ...prev, bulkSyncing: false }));
    }
  };

  const handleBulkDelete = async () => {
    if (selectedIds.size === 0 && !selectAllMatching) return;

    const idsToDelete = selectAllMatching
      ? await getAllMatchingIds()
      : Array.from(selectedIds);

    const count = idsToDelete.length;
    if (!confirm(`Are you sure you want to delete ${count} repositories?`))
      return;

    setReposState((prev) => ({ ...prev, bulkDeleting: true }));
    try {
      await Promise.all(idsToDelete.map((id) => api.deleteRepo(id)));
      setReposState((prev) => ({
        ...prev,
        selectedIds: new Set(),
        selectAllMatching: false,
      }));
      loadRepos();
    } catch (err) {
      setReposState((prev) => ({
        ...prev,
        error:
          err instanceof Error ? err.message : "Failed to delete repositories",
      }));
    } finally {
      setReposState((prev) => ({ ...prev, bulkDeleting: false }));
    }
  };

  const handleBulkExclude = async () => {
    if (selectedIds.size === 0 && !selectAllMatching) return;

    const idsToExclude = selectAllMatching
      ? await getAllMatchingIds()
      : Array.from(selectedIds);

    const count = idsToExclude.length;
    if (
      !confirm(
        `Are you sure you want to exclude ${count} repositories from sync and indexing?`
      )
    )
      return;

    setReposState((prev) => ({ ...prev, bulkExcluding: true }));
    try {
      await Promise.all(idsToExclude.map((id) => api.excludeRepo(id)));
      setReposState((prev) => ({
        ...prev,
        syncResult: {
          id: -1,
          status: "ok",
          message: `${count} repositories excluded from sync and indexing`,
        },
        selectedIds: new Set(),
        selectAllMatching: false,
      }));
      loadRepos();
    } catch (err) {
      setReposState((prev) => ({
        ...prev,
        error:
          err instanceof Error ? err.message : "Failed to exclude repositories",
      }));
    } finally {
      setReposState((prev) => ({ ...prev, bulkExcluding: false }));
    }
  };

  const handleBulkInclude = async () => {
    if (selectedIds.size === 0 && !selectAllMatching) return;

    const idsToInclude = selectAllMatching
      ? await getAllMatchingIds()
      : Array.from(selectedIds);

    const count = idsToInclude.length;
    if (
      !confirm(
        `Are you sure you want to include ${count} repositories for sync and indexing?`
      )
    )
      return;

    setReposState((prev) => ({ ...prev, bulkIncluding: true }));
    try {
      await Promise.all(idsToInclude.map((id) => api.includeRepo(id)));
      setReposState((prev) => ({
        ...prev,
        syncResult: {
          id: -1,
          status: "ok",
          message: `${count} repositories included - they will be indexed on next sync`,
        },
        selectedIds: new Set(),
        selectAllMatching: false,
      }));
      loadRepos();
    } catch (err) {
      setReposState((prev) => ({
        ...prev,
        error:
          err instanceof Error ? err.message : "Failed to include repositories",
      }));
    } finally {
      setReposState((prev) => ({ ...prev, bulkIncluding: false }));
    }
  };

  const handleBulkRestore = async () => {
    if (selectedIds.size === 0 && !selectAllMatching) return;

    const idsToRestore = selectAllMatching
      ? await getAllMatchingIds()
      : Array.from(selectedIds);

    const count = idsToRestore.length;
    if (
      !confirm(
        `Are you sure you want to restore ${count} deleted repositories?`
      )
    )
      return;

    setReposState((prev) => ({ ...prev, bulkRestoring: true }));
    try {
      await Promise.all(idsToRestore.map((id) => api.restoreRepo(id)));
      setReposState((prev) => ({
        ...prev,
        syncResult: {
          id: -1,
          status: "ok",
          message: `${count} repositories restored - they will be indexed shortly`,
        },
        selectedIds: new Set(),
        selectAllMatching: false,
      }));
      loadRepos();
    } catch (err) {
      setReposState((prev) => ({
        ...prev,
        error:
          err instanceof Error ? err.message : "Failed to restore repositories",
      }));
    } finally {
      setReposState((prev) => ({ ...prev, bulkRestoring: false }));
    }
  };

  const getStatusBadge = (status: string) => {
    const configs: Record<string, { bg: string; icon: React.ReactNode }> = {
      indexed: {
        bg: "bg-green-100 text-green-800 dark:bg-green-900/50 dark:text-green-300",
        icon: <CheckCircle2 className="h-3.5 w-3.5" />,
      },
      pending: {
        bg: "bg-yellow-100 text-yellow-800 dark:bg-yellow-900/50 dark:text-yellow-300",
        icon: <Clock className="h-3.5 w-3.5" />,
      },
      cloning: {
        bg: "bg-purple-100 text-purple-800 dark:bg-purple-900/50 dark:text-purple-300",
        icon: <Loader2 className="h-3.5 w-3.5 animate-spin" />,
      },
      indexing: {
        bg: "bg-blue-100 text-blue-800 dark:bg-blue-900/50 dark:text-blue-300",
        icon: <Loader2 className="h-3.5 w-3.5 animate-spin" />,
      },
      failed: {
        bg: "bg-red-100 text-red-800 dark:bg-red-900/50 dark:text-red-300",
        icon: <AlertCircle className="h-3.5 w-3.5" />,
      },
      excluded: {
        bg: "bg-gray-200 text-gray-600 dark:bg-gray-700 dark:text-gray-400",
        icon: <EyeOff className="h-3.5 w-3.5" />,
      },
      deleted: {
        bg: "bg-red-200 text-red-700 dark:bg-red-900/50 dark:text-red-400",
        icon: <Trash2 className="h-3.5 w-3.5" />,
      },
    };
    const config = configs[status] || {
      bg: "bg-gray-100 text-gray-800 dark:bg-gray-900/50 dark:text-gray-300",
      icon: <Clock className="h-3.5 w-3.5" />,
    };
    return (
      <span
        className={`inline-flex items-center gap-1.5 rounded-full px-2.5 py-1 text-xs font-medium ${config.bg}`}
      >
        {config.icon}
        {status}
      </span>
    );
  };

  const statuses = [
    "",
    "indexed",
    "pending",
    "cloning",
    "indexing",
    "failed",
    "excluded",
    "deleted",
  ];

  return (
    <div className="h-full overflow-auto">
      <div className="container mx-auto max-w-full px-4 py-6">
        <div className="mx-auto max-w-6xl">
          <div className="mb-4 flex items-center justify-between sm:mb-6">
            <div className="flex items-center gap-2 sm:gap-3">
              <FolderGit2 className="h-5 w-5 text-gray-600 dark:text-gray-400 sm:h-6 sm:w-6" />
              <h1 className="text-xl font-bold sm:text-2xl">Repositories</h1>
            </div>
            <button
              type="button"
              onClick={() => loadRepos()}
              className="flex items-center gap-2 rounded-lg border border-gray-200 bg-white px-2.5 py-1.5 text-sm font-medium shadow-sm transition-all hover:bg-gray-50 focus:outline-none focus:ring-2 focus:ring-blue-500/20 focus:ring-offset-2 dark:border-gray-700 dark:bg-gray-800 dark:hover:bg-gray-700 sm:px-3 sm:py-2"
            >
              <RefreshCw className="h-4 w-4 text-gray-500 dark:text-gray-400" />
              <span className="hidden sm:inline">Refresh</span>
            </button>
          </div>

          {reposReadOnly && !hideReadOnlyBanner && (
            <div className="mb-4 rounded-lg border border-amber-200 bg-amber-50 p-3 text-amber-700 dark:border-amber-800 dark:bg-amber-900/20 dark:text-amber-400 sm:p-4">
              <div className="flex items-start gap-3">
                <AlertCircle className="mt-0.5 h-5 w-5 flex-shrink-0 sm:h-5 sm:w-5" />
                <div>
                  <p className="text-sm font-bold">Read-only mode enabled</p>
                  <p className="mt-1 text-sm text-amber-700 dark:text-amber-300">
                    Repositories are managed via sync. Delete is disabled, but
                    you can exclude repos from sync and indexing.
                  </p>
                </div>
              </div>
            </div>
          )}

          {error && (
            <div className="mb-4 flex items-center gap-2 rounded-xl border border-red-200 bg-red-50 p-3 text-sm text-red-700 dark:border-red-800 dark:bg-red-900/20 dark:text-red-400 sm:gap-3 sm:p-4">
              <AlertCircle className="h-4 w-4 flex-shrink-0 sm:h-5 sm:w-5" />
              {error}
            </div>
          )}

          {syncResult && (
            <div
              className={`mb-4 flex items-center gap-2 rounded-xl p-3 text-sm sm:gap-3 sm:p-4 ${
                syncResult.status === "ok"
                  ? "border border-green-200 bg-green-50 text-green-700 dark:border-green-800 dark:bg-green-900/20 dark:text-green-400"
                  : syncResult.status === "info"
                    ? "border border-blue-200 bg-blue-50 text-blue-700 dark:border-blue-800 dark:bg-blue-900/20 dark:text-blue-400"
                    : "border border-red-200 bg-red-50 text-red-700 dark:border-red-800 dark:bg-red-900/20 dark:text-red-400"
              }`}
            >
              {syncResult.status === "ok" ? (
                <CheckCircle2 className="h-4 w-4 flex-shrink-0 sm:h-5 sm:w-5" />
              ) : syncResult.status === "info" ? (
                <Loader2 className="h-4 w-4 flex-shrink-0 animate-spin sm:h-5 sm:w-5" />
              ) : (
                <AlertCircle className="h-4 w-4 flex-shrink-0 sm:h-5 sm:w-5" />
              )}
              <span className="flex-1">{syncResult.message}</span>
              <button
                type="button"
                onClick={() =>
                  setReposState((prev) => ({ ...prev, syncResult: null }))
                }
                className="rounded p-1 transition-colors hover:bg-black/10 dark:hover:bg-white/10"
                aria-label="Dismiss message"
              >
                <X className="h-4 w-4" />
              </button>
            </div>
          )}

          <div className="mb-5 flex flex-col gap-3 sm:flex-row sm:items-center sm:gap-4">
            <div className="relative flex-1">
              <label htmlFor="repo-search" className="sr-only">
                Search repositories
              </label>
              <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-gray-400" />
              <input
                id="repo-search"
                type="text"
                value={search}
                onChange={(e) =>
                  setReposState((prev) => ({ ...prev, search: e.target.value }))
                }
                placeholder="Search by repository..."
                className="w-full rounded-lg border border-gray-200 bg-white py-2 pl-9 pr-3 text-sm shadow-sm focus:border-blue-500 focus:outline-none dark:border-gray-700 dark:bg-gray-800 dark:focus:border-blue-500"
              />
            </div>
            <div className="flex flex-shrink-0 flex-wrap items-center gap-2 sm:flex-nowrap">
              <div className="flex flex-1 items-center gap-2 sm:flex-initial">
                <Filter className="hidden h-4 w-4 text-gray-400 sm:block" />
                <label htmlFor="connection-filter" className="sr-only">
                  Filter by connection
                </label>
                <select
                  id="connection-filter"
                  value={connectionFilter || ""}
                  onChange={(e) =>
                    setReposState((prev) => ({
                      ...prev,
                      connectionFilter: e.target.value
                        ? Number(e.target.value)
                        : null,
                      currentPage: 1,
                    }))
                  }
                  className="w-[120px] appearance-none rounded-lg border border-gray-200 bg-white bg-[url('data:image/svg+xml;charset=UTF-8,%3Csvg%20xmlns%3D%22http%3A%2F%2Fwww.w3.org%2F2000%2Fsvg%22%20viewBox%3D%220%200%2020%2020%22%20fill%3D%22%236b7280%22%3E%3Cpath%20fill-rule%3D%22evenodd%22%20d%3D%22M5.293%207.293a1%201%200%20011.414%200L10%2010.586l3.293-3.293a1%201%200%20111.414%201.414l-4%204a1%201%200%2001-1.414%200l-4-4a1%201%200%20010-1.414z%22%20clip-rule%3D%22evenodd%22%2F%3E%3C%2Fsvg%3E')] bg-[length:16px_16px] bg-[position:right_8px_center] bg-no-repeat py-2 pl-2 pr-7 text-xs shadow-sm focus:border-blue-500 focus:outline-none dark:border-gray-700 dark:bg-gray-800 dark:focus:border-blue-500 sm:w-[155px] sm:pl-3 sm:pr-8 sm:text-sm"
                >
                  <option value="">All connections</option>
                  {connections.map((conn) => (
                    <option key={conn.id} value={conn.id}>
                      {conn.name}
                    </option>
                  ))}
                </select>
                <label htmlFor="status-filter" className="sr-only">
                  Filter by status
                </label>
                <select
                  id="status-filter"
                  value={statusFilter}
                  onChange={(e) =>
                    setReposState((prev) => ({
                      ...prev,
                      statusFilter: e.target.value,
                      currentPage: 1,
                    }))
                  }
                  className="w-[105px] appearance-none rounded-lg border border-gray-200 bg-white bg-[url('data:image/svg+xml;charset=UTF-8,%3Csvg%20xmlns%3D%22http%3A%2F%2Fwww.w3.org%2F2000%2Fsvg%22%20viewBox%3D%220%200%2020%2020%22%20fill%3D%22%236b7280%22%3E%3Cpath%20fill-rule%3D%22evenodd%22%20d%3D%22M5.293%207.293a1%201%200%20011.414%200L10%2010.586l3.293-3.293a1%201%200%20111.414%201.414l-4%204a1%201%200%2001-1.414%200l-4-4a1%201%200%20010-1.414z%22%20clip-rule%3D%22evenodd%22%2F%3E%3C%2Fsvg%3E')] bg-[length:16px_16px] bg-[position:right_8px_center] bg-no-repeat py-2 pl-2 pr-7 text-xs shadow-sm focus:border-blue-500 focus:outline-none dark:border-gray-700 dark:bg-gray-800 dark:focus:border-blue-500 sm:w-[130px] sm:pl-3 sm:pr-8 sm:text-sm"
                >
                  {statuses.map((status) => (
                    <option key={status} value={status}>
                      {status === ""
                        ? "All statuses"
                        : status.charAt(0).toUpperCase() + status.slice(1)}
                    </option>
                  ))}
                </select>
              </div>
              <div className="whitespace-nowrap rounded-lg bg-gray-100 px-2 py-1.5 text-xs text-gray-500 dark:bg-gray-800 dark:text-gray-400 sm:px-3 sm:text-sm">
                {totalCount}/{allReposCount}
              </div>
            </div>
          </div>

          {(isSomeSelected || selectAllMatching) && (
            <div className="mb-4 rounded-lg border border-blue-200 bg-blue-50 p-3 dark:border-blue-800 dark:bg-blue-900/20">
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

                {isAllSelected &&
                  !selectAllMatching &&
                  totalCount > repos.length && (
                    <button
                      type="button"
                      onClick={() =>
                        setReposState((prev) => ({
                          ...prev,
                          selectAllMatching: true,
                        }))
                      }
                      className="text-sm font-medium text-blue-600 hover:underline dark:text-blue-400"
                    >
                      Select all {totalCount} repos matching this filter
                    </button>
                  )}

                {selectAllMatching && (
                  <button
                    type="button"
                    onClick={() => {
                      setReposState((prev) => ({
                        ...prev,
                        selectAllMatching: false,
                        selectedIds: new Set(),
                      }));
                    }}
                    className="text-sm font-medium text-blue-600 hover:underline dark:text-blue-400"
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
                    className="flex items-center gap-1.5 rounded-lg border border-blue-500 px-2 py-1.5 text-sm font-medium text-blue-600 transition-all hover:bg-blue-50 focus:outline-none focus:ring-2 focus:ring-blue-500/20 focus:ring-offset-2 disabled:opacity-50 dark:border-blue-400 dark:text-blue-400 dark:hover:bg-blue-900/30 sm:px-3"
                  >
                    {bulkSyncing ? (
                      <Loader2 className="h-4 w-4 animate-spin" />
                    ) : (
                      <RefreshCw className="h-4 w-4" />
                    )}
                    <span className="hidden sm:inline">
                      {bulkSyncing ? "Syncing..." : "Sync"}
                    </span>
                  </button>
                  <button
                    type="button"
                    onClick={handleBulkExclude}
                    disabled={bulkExcluding}
                    className="flex items-center gap-1.5 rounded-lg border border-amber-500 px-2 py-1.5 text-sm font-medium text-amber-600 transition-all hover:bg-amber-50 focus:outline-none focus:ring-2 focus:ring-amber-500/20 focus:ring-offset-2 disabled:opacity-50 dark:border-amber-400 dark:text-amber-400 dark:hover:bg-amber-900/30 sm:px-3"
                  >
                    {bulkExcluding ? (
                      <Loader2 className="h-4 w-4 animate-spin" />
                    ) : (
                      <EyeOff className="h-4 w-4" />
                    )}
                    <span className="hidden sm:inline">
                      {bulkExcluding ? "Excluding..." : "Exclude"}
                    </span>
                  </button>
                  <button
                    type="button"
                    onClick={handleBulkInclude}
                    disabled={bulkIncluding}
                    className="flex items-center gap-1.5 rounded-lg border border-green-500 px-2 py-1.5 text-sm font-medium text-green-600 transition-all hover:bg-green-50 focus:outline-none focus:ring-2 focus:ring-green-500/20 focus:ring-offset-2 disabled:opacity-50 dark:border-green-400 dark:text-green-400 dark:hover:bg-green-900/30 sm:px-3"
                  >
                    {bulkIncluding ? (
                      <Loader2 className="h-4 w-4 animate-spin" />
                    ) : (
                      <Eye className="h-4 w-4" />
                    )}
                    <span className="hidden sm:inline">
                      {bulkIncluding ? "Including..." : "Include"}
                    </span>
                  </button>
                  <button
                    type="button"
                    onClick={handleBulkRestore}
                    disabled={bulkRestoring}
                    className="flex items-center gap-1.5 rounded-lg border border-green-500 px-2 py-1.5 text-sm font-medium text-green-600 transition-all hover:bg-green-50 focus:outline-none focus:ring-2 focus:ring-green-500/20 focus:ring-offset-2 disabled:opacity-50 dark:border-green-400 dark:text-green-400 dark:hover:bg-green-900/20 sm:px-3"
                  >
                    {bulkRestoring ? (
                      <Loader2 className="h-4 w-4 animate-spin" />
                    ) : (
                      <RotateCcw className="h-4 w-4" />
                    )}
                    <span className="hidden sm:inline">
                      {bulkRestoring ? "Restoring..." : "Restore"}
                    </span>
                  </button>
                  {settingsLoaded && !reposReadOnly && (
                    <button
                      type="button"
                      onClick={handleBulkDelete}
                      disabled={bulkDeleting}
                      className="flex items-center gap-1.5 rounded-lg border border-red-500 px-2 py-1.5 text-sm font-medium text-red-600 transition-all hover:bg-red-50 focus:outline-none focus:ring-2 focus:ring-red-500/20 focus:ring-offset-2 disabled:opacity-50 dark:border-red-400 dark:text-red-400 dark:hover:bg-red-900/30 sm:px-3"
                    >
                      {bulkDeleting ? (
                        <Loader2 className="h-4 w-4 animate-spin" />
                      ) : (
                        <Trash2 className="h-4 w-4" />
                      )}
                      <span className="hidden sm:inline">
                        {bulkDeleting ? "Deleting..." : "Delete"}
                      </span>
                    </button>
                  )}
                  <button
                    type="button"
                    onClick={() => {
                      setReposState((prev) => ({
                        ...prev,
                        selectedIds: new Set(),
                        selectAllMatching: false,
                      }));
                    }}
                    className="flex items-center gap-1.5 rounded-lg border border-gray-300 px-2 py-1.5 text-sm font-medium text-gray-600 transition-all hover:bg-gray-100 focus:outline-none focus:ring-2 focus:ring-gray-500/20 focus:ring-offset-2 dark:border-gray-600 dark:text-gray-300 dark:hover:bg-gray-700 sm:px-3"
                  >
                    <X className="h-4 w-4" />
                    <span className="hidden sm:inline">Clear</span>
                  </button>
                </div>
              </div>
            </div>
          )}

          {loading ? (
            <div className="py-16 text-center">
              <Loader2 className="mx-auto h-8 w-8 animate-spin text-blue-600" />
              <p className="mt-3 text-gray-500 dark:text-gray-400">
                Loading repositories...
              </p>
            </div>
          ) : totalCount === 0 && !debouncedSearch && !statusFilter ? (
            <div className="rounded-xl border border-gray-200 bg-white py-16 text-center dark:border-gray-700 dark:bg-gray-800">
              <FolderGit2 className="mx-auto mb-4 h-12 w-12 text-gray-300 dark:text-gray-600" />
              <p className="mb-2 text-gray-500 dark:text-gray-400">
                No repositories found
              </p>
              <p className="text-sm text-gray-400 dark:text-gray-500">
                Add a connection to start indexing repositories
              </p>
            </div>
          ) : repos.length === 0 ? (
            <div className="rounded-xl border border-gray-200 bg-white py-16 text-center dark:border-gray-700 dark:bg-gray-800">
              <Search className="mx-auto mb-4 h-12 w-12 text-gray-300 dark:text-gray-600" />
              <p className="mb-3 text-gray-500 dark:text-gray-400">
                No repositories match your search
              </p>
              <button
                onClick={() =>
                  setReposState((prev) => ({
                    ...prev,
                    search: "",
                    statusFilter: "",
                  }))
                }
                className="text-sm font-medium text-blue-600 hover:underline dark:text-blue-400"
              >
                Clear filters
              </button>
            </div>
          ) : (
            <>
              <div className="rounded-xl border border-gray-200 bg-white shadow-sm dark:border-gray-700 dark:bg-gray-800">
                <div className="w-full">
                  <table className="w-full table-fixed divide-y divide-gray-200 dark:divide-gray-700">
                    <thead className="bg-gray-50 dark:bg-gray-800/80">
                      <tr>
                        <th className="w-10 px-3 py-3.5 text-center">
                          <input
                            type="checkbox"
                            checked={isAllSelected}
                            onChange={toggleSelectAll}
                            aria-label="Select all repositories"
                            className="h-4 w-4 rounded border-gray-300 text-blue-600 focus:ring-blue-500 dark:border-gray-600 dark:bg-gray-700"
                          />
                        </th>
                        <th className="px-4 py-3.5 text-left text-xs font-semibold uppercase tracking-wider text-gray-500 dark:text-gray-400">
                          Repository
                        </th>
                        <th className="w-20 px-2 py-3.5 text-left text-xs font-semibold uppercase tracking-wider text-gray-500 dark:text-gray-400 sm:w-[100px] sm:px-4">
                          Status
                        </th>
                        <th className="hidden w-[120px] px-4 py-3.5 text-left text-xs font-semibold uppercase tracking-wider text-gray-500 dark:text-gray-400 md:table-cell">
                          Branch
                        </th>
                        <th className="hidden w-[160px] px-4 py-3.5 text-left text-xs font-semibold uppercase tracking-wider text-gray-500 dark:text-gray-400 sm:table-cell">
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
                          className={`transition-colors hover:bg-gray-50 dark:hover:bg-gray-700/30 ${
                            selectedIds.has(repo.id)
                              ? "bg-blue-50 dark:bg-blue-900/10"
                              : ""
                          }`}
                        >
                          <td className="w-10 px-3 py-4 text-center">
                            <input
                              type="checkbox"
                              checked={selectedIds.has(repo.id)}
                              onChange={() => toggleSelect(repo.id)}
                              aria-label={`Select ${repo.name}`}
                              className="h-4 w-4 rounded border-gray-300 text-blue-600 focus:ring-blue-500 dark:border-gray-600 dark:bg-gray-700"
                            />
                          </td>
                          <td className="overflow-hidden px-4 py-4">
                            <div className="flex min-w-0 items-center gap-3">
                              <GitBranch className="hidden h-5 w-5 flex-shrink-0 text-gray-400 sm:block" />
                              <div className="min-w-0 flex-1">
                                <div
                                  className={`truncate text-sm font-medium ${repo.deleted ? "text-red-400 line-through dark:text-red-500" : repo.excluded ? "text-gray-400 dark:text-gray-500" : ""}`}
                                >
                                  {repo.name}
                                </div>
                                <div className="truncate text-xs text-gray-500 dark:text-gray-400">
                                  {repo.clone_url}
                                </div>
                              </div>
                            </div>
                          </td>
                          <td className="whitespace-nowrap px-2 py-4 sm:px-4">
                            {getStatusBadge(
                              repo.deleted
                                ? "deleted"
                                : repo.excluded
                                  ? "excluded"
                                  : repo.status
                            )}
                          </td>
                          <td className="hidden truncate px-4 py-4 text-sm text-gray-500 dark:text-gray-400 md:table-cell">
                            {repo.branches?.length > 0
                              ? repo.branches.join(", ")
                              : repo.default_branch || "-"}
                          </td>
                          <td className="hidden whitespace-nowrap px-4 py-4 text-sm text-gray-500 dark:text-gray-400 sm:table-cell">
                            {repo.last_indexed
                              ? new Date(repo.last_indexed).toLocaleString()
                              : "Never"}
                          </td>
                          <td className="w-12 px-2 py-4 pr-4">
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
                <div className="mt-6 flex flex-col items-center justify-between gap-3 sm:flex-row">
                  <div className="text-center text-sm text-gray-500 dark:text-gray-400 sm:text-left">
                    {(currentPage - 1) * PAGE_SIZE + 1}-
                    {Math.min(currentPage * PAGE_SIZE, totalCount)} of{" "}
                    {totalCount}
                  </div>
                  <nav
                    className="flex items-center gap-1"
                    aria-label="Pagination"
                  >
                    <button
                      type="button"
                      onClick={() =>
                        setReposState((prev) => ({ ...prev, currentPage: 1 }))
                      }
                      disabled={currentPage === 1}
                      className="hidden rounded-lg border border-gray-200 p-2 text-sm transition-colors hover:bg-gray-100 disabled:cursor-not-allowed disabled:opacity-50 dark:border-gray-700 dark:hover:bg-gray-800 sm:flex"
                      title="First page"
                    >
                      <ChevronsLeft className="h-4 w-4" />
                    </button>
                    <button
                      type="button"
                      onClick={() =>
                        setReposState((prev) => ({
                          ...prev,
                          currentPage: Math.max(1, prev.currentPage - 1),
                        }))
                      }
                      disabled={currentPage === 1}
                      className="rounded-lg border border-gray-200 p-2 text-sm transition-colors hover:bg-gray-100 disabled:cursor-not-allowed disabled:opacity-50 dark:border-gray-700 dark:hover:bg-gray-800"
                      title="Previous page"
                    >
                      <ChevronLeft className="h-4 w-4" />
                    </button>
                    <span className="px-2 text-sm text-gray-600 dark:text-gray-400 sm:hidden">
                      {currentPage} / {totalPages}
                    </span>
                    <div
                      className="mx-1 hidden items-center gap-1 sm:flex"
                      role="group"
                      aria-label="Page numbers"
                    >
                      {Array.from(
                        { length: Math.min(5, totalPages) },
                        (_, i) => {
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
                              onClick={() =>
                                setReposState((prev) => ({
                                  ...prev,
                                  currentPage: pageNum,
                                }))
                              }
                              aria-current={
                                currentPage === pageNum ? "page" : undefined
                              }
                              className={`h-9 w-9 rounded-lg border text-sm font-medium transition-colors ${
                                currentPage === pageNum
                                  ? "border-blue-600 bg-blue-600 text-white shadow-sm"
                                  : "border-gray-200 hover:bg-gray-100 dark:border-gray-700 dark:hover:bg-gray-800"
                              }`}
                            >
                              {pageNum}
                            </button>
                          );
                        }
                      )}
                    </div>
                    <button
                      type="button"
                      onClick={() =>
                        setReposState((prev) => ({
                          ...prev,
                          currentPage: Math.min(
                            totalPages,
                            prev.currentPage + 1
                          ),
                        }))
                      }
                      disabled={currentPage === totalPages}
                      className="rounded-lg border border-gray-200 p-2 text-sm transition-colors hover:bg-gray-100 disabled:cursor-not-allowed disabled:opacity-50 dark:border-gray-700 dark:hover:bg-gray-800"
                      title="Next page"
                    >
                      <ChevronRight className="h-4 w-4" />
                    </button>
                    <button
                      type="button"
                      onClick={() =>
                        setReposState((prev) => ({
                          ...prev,
                          currentPage: totalPages,
                        }))
                      }
                      disabled={currentPage === totalPages}
                      className="hidden rounded-lg border border-gray-200 p-2 text-sm transition-colors hover:bg-gray-100 disabled:cursor-not-allowed disabled:opacity-50 dark:border-gray-700 dark:hover:bg-gray-800 sm:flex"
                      title="Last page"
                    >
                      <ChevronsRight className="h-4 w-4" />
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
          onClose={() =>
            setReposState((prev) => ({ ...prev, branchesModalRepo: null }))
          }
          onSave={handleSaveBranches}
        />
      )}
    </div>
  );
}
