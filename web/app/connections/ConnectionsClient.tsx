"use client";

import { useState, useEffect, useRef, useReducer } from "react";
import { api, Connection, Repository } from "@/lib/api";
import {
  Link2,
  Plus,
  X,
  TestTube,
  RefreshCw,
  Trash2,
  ChevronDown,
  ChevronUp,
  Loader2,
  CheckCircle2,
  AlertCircle,
  Github,
  Gitlab,
  GitBranch,
  FolderGit2,
  MoreVertical,
  EyeOff,
  Clock,
  Pencil,
} from "lucide-react";

// Dropdown menu component for connection actions
function ConnectionActionMenu({
  connection,
  onSync,
  onEdit,
  onDelete,
  syncing,
  readonly,
}: {
  connection: Connection;
  onSync: (id: number) => void;
  onEdit: (connection: Connection) => void;
  onDelete: (id: number) => void;
  syncing: number | null;
  readonly?: boolean;
}) {
  const [isOpen, setIsOpen] = useState(false);
  const menuRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!isOpen) return;
    function handleClickOutside(event: MouseEvent) {
      if (menuRef.current && !menuRef.current.contains(event.target as Node)) {
        setIsOpen(false);
      }
    }
    document.addEventListener("mousedown", handleClickOutside);
    return () => document.removeEventListener("mousedown", handleClickOutside);
  }, [isOpen]);

  return (
    <div className="relative" ref={menuRef}>
      <button
        onClick={() => setIsOpen(!isOpen)}
        className="rounded-lg p-2 transition-all hover:bg-gray-100 focus:outline-none focus:ring-2 focus:ring-blue-500/20 dark:hover:bg-gray-700"
      >
        <MoreVertical className="h-4 w-4 text-gray-500" />
      </button>
      {isOpen && (
        <div className="absolute right-0 top-full z-50 mt-1 w-40 rounded-lg border border-gray-200 bg-white shadow-lg dark:border-gray-700 dark:bg-gray-800">
          <button
            onClick={() => {
              onSync(connection.id);
              setIsOpen(false);
            }}
            disabled={syncing === connection.id}
            className="flex w-full items-center gap-2 rounded-t-lg px-3 py-2 text-left text-sm transition-colors hover:bg-gray-50 disabled:opacity-50 dark:hover:bg-gray-700"
          >
            {syncing === connection.id ? (
              <Loader2 className="h-4 w-4 animate-spin text-gray-500 dark:text-gray-400" />
            ) : (
              <RefreshCw className="h-4 w-4 text-gray-500 dark:text-gray-400" />
            )}
            <span>
              {syncing === connection.id ? "Syncing..." : "Sync Repos"}
            </span>
          </button>
          {!readonly && (
            <>
              <button
                onClick={() => {
                  onEdit(connection);
                  setIsOpen(false);
                }}
                className="flex w-full items-center gap-2 px-3 py-2 text-left text-sm transition-colors hover:bg-gray-50 dark:hover:bg-gray-700"
              >
                <Pencil className="h-4 w-4 text-gray-500 dark:text-gray-400" />
                <span>Edit</span>
              </button>
              <button
                onClick={() => {
                  onDelete(connection.id);
                  setIsOpen(false);
                }}
                className="flex w-full items-center gap-2 rounded-b-lg px-3 py-2 text-left text-sm text-red-500 transition-colors hover:bg-red-50 dark:text-red-400 dark:hover:bg-red-900/20"
              >
                <Trash2 className="h-4 w-4" />
                <span>Delete</span>
              </button>
            </>
          )}
        </div>
      )}
    </div>
  );
}

interface ConnectionWithRepos extends Connection {
  repos?: Repository[];
  reposLoading?: boolean;
}

interface IndexingStatus {
  connectionId: number;
  pending: number;
  running: number;
  completed: number;
  failed: number;
  total: number;
}

function mergeReducer<T>(state: T, update: Partial<T>): T {
  return { ...state, ...update };
}

// Extracted form sub-component to reduce main component size
function ConnectionForm({
  formName,
  formType,
  formUrl,
  formToken,
  formExcludeArchived,
  submitting,
  editingConnection,
  updateForm,
  getDefaultUrl,
  onSubmit,
}: {
  formName: string;
  formType: string;
  formUrl: string;
  formToken: string;
  formExcludeArchived: boolean;
  submitting: boolean;
  editingConnection: Connection | null;
  updateForm: (
    update: Partial<{
      showForm: boolean;
      formName: string;
      formType: string;
      formUrl: string;
      formToken: string;
      formExcludeArchived: boolean;
      submitting: boolean;
      editingConnection: Connection | null;
    }>
  ) => void;
  getDefaultUrl: (type: string) => string;
  onSubmit: (e: React.FormEvent) => void;
}) {
  return (
    <form
      onSubmit={onSubmit}
      className="mb-4 rounded-xl border border-gray-200 bg-white p-4 shadow-sm dark:border-gray-700 dark:bg-gray-800 sm:mb-6 sm:p-6"
    >
      <h2 className="mb-4 text-base font-semibold sm:mb-5 sm:text-lg">
        {editingConnection ? "Edit Connection" : "New Connection"}
      </h2>
      <div className="mb-4 grid grid-cols-1 gap-4 sm:mb-5 sm:grid-cols-2 sm:gap-5">
        <div>
          <label
            htmlFor="conn-name"
            className="mb-1.5 block text-sm font-medium text-gray-700 dark:text-gray-300"
          >
            Name
          </label>
          <input
            id="conn-name"
            type="text"
            value={formName}
            onChange={(e) => updateForm({ formName: e.target.value })}
            placeholder="My GitHub"
            required
            className="w-full rounded-lg border border-gray-200 bg-white px-3 py-2 text-sm focus:border-blue-500 focus:outline-none dark:border-gray-700 dark:bg-gray-800 dark:focus:border-blue-500"
          />
        </div>
        <div>
          <label
            htmlFor="conn-type"
            className="mb-1.5 block text-sm font-medium text-gray-700 dark:text-gray-300"
          >
            Type
          </label>
          <select
            id="conn-type"
            value={formType}
            onChange={(e) => {
              updateForm({
                formType: e.target.value,
                formUrl: getDefaultUrl(e.target.value),
              });
            }}
            className="w-full appearance-none rounded-lg border border-gray-200 bg-white bg-[url('data:image/svg+xml;charset=UTF-8,%3Csvg%20xmlns%3D%22http%3A%2F%2Fwww.w3.org%2F2000%2Fsvg%22%20viewBox%3D%220%200%2020%2020%22%20fill%3D%22%236b7280%22%3E%3Cpath%20fill-rule%3D%22evenodd%22%20d%3D%22M5.293%207.293a1%201%200%20011.414%200L10%2010.586l3.293-3.293a1%201%200%20111.414%201.414l-4%204a1%201%200%2001-1.414%200l-4-4a1%201%200%20010-1.414z%22%20clip-rule%3D%22evenodd%22%2F%3E%3C%2Fsvg%3E')] bg-[length:16px_16px] bg-[position:right_8px_center] bg-no-repeat py-2 pl-3 pr-8 text-sm focus:border-blue-500 focus:outline-none dark:border-gray-700 dark:bg-gray-800 dark:focus:border-blue-500"
          >
            <option value="github">GitHub</option>
            <option value="github_enterprise">GitHub Enterprise</option>
            <option value="gitlab">GitLab</option>
            <option value="gitea">Gitea</option>
            <option value="bitbucket">Bitbucket</option>
          </select>
        </div>
      </div>
      <div className="mb-5">
        <label
          htmlFor="conn-url"
          className="mb-1.5 block text-sm font-medium text-gray-700 dark:text-gray-300"
        >
          URL
        </label>
        <input
          id="conn-url"
          type="url"
          value={formUrl}
          onChange={(e) => updateForm({ formUrl: e.target.value })}
          placeholder={getDefaultUrl(formType)}
          className="w-full rounded-lg border border-gray-200 bg-white px-3 py-2 text-sm focus:border-blue-500 focus:outline-none dark:border-gray-700 dark:bg-gray-800 dark:focus:border-blue-500"
        />
      </div>
      <div className="mb-5">
        <label
          htmlFor="conn-token"
          className="mb-1.5 block text-sm font-medium text-gray-700 dark:text-gray-300"
        >
          Access Token
          {editingConnection && (
            <span className="ml-2 font-normal text-gray-500 dark:text-gray-400">
              (leave empty to keep current)
            </span>
          )}
        </label>
        <input
          id="conn-token"
          type="password"
          value={formToken}
          onChange={(e) => updateForm({ formToken: e.target.value })}
          placeholder={
            editingConnection
              ? "Enter new token to change"
              : "ghp_xxxx or glpat-xxxx"
          }
          required={!editingConnection}
          autoComplete="off" // Prevent browser autofill
          data-1p-ignore // 1Password
          data-lpignore="true" // LastPass
          data-form-type="other" // Bitwarden
          className="w-full rounded-lg border border-gray-200 bg-white px-3 py-2 text-sm focus:border-blue-500 focus:outline-none dark:border-gray-700 dark:bg-gray-800 dark:focus:border-blue-500"
        />
      </div>
      <div className="mb-5 pl-1">
        <label
          htmlFor="conn-exclude-archived"
          className="flex cursor-pointer items-center gap-3"
        >
          <input
            id="conn-exclude-archived"
            type="checkbox"
            checked={formExcludeArchived}
            onChange={(e) =>
              updateForm({ formExcludeArchived: e.target.checked })
            }
            className="h-4 w-4 rounded border-gray-300 text-blue-600 focus:ring-blue-500 dark:border-gray-600 dark:bg-gray-700"
          />
          <div>
            <span className="text-sm font-medium text-gray-700 dark:text-gray-300">
              Exclude archived repositories
            </span>
            <p className="text-xs text-gray-500 dark:text-gray-400">
              Archived repos will be excluded from sync and indexing
            </p>
          </div>
        </label>
      </div>
      <button
        type="submit"
        disabled={submitting}
        className="flex items-center gap-2 rounded-lg border border-blue-500 px-4 py-2 text-sm font-medium text-blue-600 transition-colors hover:bg-blue-50 disabled:opacity-50 dark:border-blue-400 dark:text-blue-400 dark:hover:bg-blue-900/30"
      >
        {submitting ? (
          <Loader2 className="h-4 w-4 animate-spin" />
        ) : editingConnection ? (
          <Pencil className="h-4 w-4" />
        ) : (
          <Plus className="h-4 w-4" />
        )}
        {submitting
          ? editingConnection
            ? "Updating..."
            : "Creating..."
          : editingConnection
            ? "Update Connection"
            : "Create Connection"}
      </button>
    </form>
  );
}

export default function ConnectionsClient() {
  // Group 1 - Connection data state
  const [connState, updateConn] = useReducer(mergeReducer, {
    connections: [] as ConnectionWithRepos[],
    loading: true,
    error: null as string | null,
    expandedConnection: null as number | null,
    indexingStatus: new Map<number, IndexingStatus>(),
  });
  const { connections, loading, error, expandedConnection, indexingStatus } =
    connState;
  // Ref to always have the latest connections for async callbacks (avoids stale closures)
  const connectionsRef = useRef(connections);
  connectionsRef.current = connections;

  // Group 2 - Form state
  const [formState, updateForm] = useReducer(mergeReducer, {
    showForm: false,
    formName: "",
    formType: "github",
    formUrl: "",
    formToken: "",
    formExcludeArchived: true,
    submitting: false,
    editingConnection: null as Connection | null,
  });
  const {
    showForm,
    formName,
    formType,
    formUrl,
    formToken,
    formExcludeArchived,
    submitting,
    editingConnection,
  } = formState;

  // Group 3 - UI feedback state
  const [uiState, updateUI] = useReducer(mergeReducer, {
    testing: null as number | null,
    testResult: null as { id: number; status: string; message?: string } | null,
    syncing: null as number | null,
    syncResult: null as { id: number; status: string; message?: string } | null,
    readonlyMode: false,
    hideReadOnlyBanner: false,
    settingsLoaded: false,
  });
  const {
    testing,
    testResult,
    syncing,
    syncResult,
    readonlyMode,
    hideReadOnlyBanner,
    settingsLoaded,
  } = uiState;

  // Poll for indexing job status
  useEffect(() => {
    const pollIndexingStatus = async () => {
      try {
        // Fetch all job states to calculate progress
        const [pendingResponse, runningResponse, completedResponse] =
          await Promise.all([
            api.listJobs({ type: "index", status: "pending", limit: 10000 }),
            api.listJobs({ type: "index", status: "running", limit: 100 }),
            api.listJobs({ type: "index", status: "completed", limit: 10000 }),
          ]);

        const pendingJobs = pendingResponse.jobs || [];
        const runningJobs = runningResponse.jobs || [];
        const completedJobs = completedResponse.jobs || [];

        // Find the oldest pending/running job per connection to determine sync start time
        const syncStartTimes = new Map<number, string>();
        for (const job of [...pendingJobs, ...runningJobs]) {
          const connId = job.payload?.connection_id as number;
          if (connId && job.created_at) {
            const existing = syncStartTimes.get(connId);
            if (!existing || job.created_at < existing) {
              syncStartTimes.set(connId, job.created_at);
            }
          }
        }

        // Fetch failed jobs only from current sync cycle (per connection)
        // Use the oldest sync start time across all connections, or last 24h if no active sync
        let oldestSyncStart: string | undefined;
        for (const time of syncStartTimes.values()) {
          if (!oldestSyncStart || time < oldestSyncStart) {
            oldestSyncStart = time;
          }
        }

        // If there's an active sync, get failed jobs from that time; otherwise use last 24h
        const createdAfter =
          oldestSyncStart ||
          new Date(Date.now() - 24 * 60 * 60 * 1000).toISOString();
        const failedResponse = await api.listJobs({
          type: "index",
          status: "failed",
          limit: 1000,
          created_after: createdAfter,
        });
        const failedJobs = failedResponse.jobs || [];

        // Group by connection ID
        const statusMap = new Map<number, IndexingStatus>();

        // Helper to update status for a connection
        const updateStatus = (
          connId: number,
          field: "pending" | "running" | "completed" | "failed"
        ) => {
          const current = statusMap.get(connId) || {
            connectionId: connId,
            pending: 0,
            running: 0,
            completed: 0,
            failed: 0,
            total: 0,
          };
          current[field]++;
          current.total++;
          statusMap.set(connId, current);
        };

        // Process all job types
        for (const job of pendingJobs) {
          const connId = job.payload?.connection_id as number;
          if (connId) updateStatus(connId, "pending");
        }

        for (const job of runningJobs) {
          const connId = job.payload?.connection_id as number;
          if (connId) updateStatus(connId, "running");
        }

        for (const job of completedJobs) {
          const connId = job.payload?.connection_id as number;
          if (connId) updateStatus(connId, "completed");
        }

        for (const job of failedJobs) {
          const connId = job.payload?.connection_id as number;
          if (connId) updateStatus(connId, "failed");
        }

        updateConn({ indexingStatus: statusMap });
      } catch (err) {
        // Silently fail - don't disrupt the UI
        console.error("Failed to poll indexing status:", err);
      }
    };

    // Poll immediately and then every 3 seconds
    pollIndexingStatus();
    const interval = setInterval(pollIndexingStatus, 3000);

    return () => clearInterval(interval);
  }, []);

  useEffect(() => {
    loadConnections();
    loadConnectionsStatus();
  }, []);

  const loadConnectionsStatus = async () => {
    try {
      const uiSettings = await api.getUISettings();
      updateUI({
        readonlyMode: uiSettings.connections_readonly,
        hideReadOnlyBanner: uiSettings.hide_readonly_banner,
        settingsLoaded: true,
      });
    } catch {
      // Fallback to old endpoint
      try {
        const status = await api.getConnectionsStatus();
        updateUI({ readonlyMode: status.readonly });
      } catch (err) {
        console.error("Failed to load connections status:", err);
      }
      updateUI({ settingsLoaded: true });
    }
  };

  const loadConnections = async () => {
    try {
      updateConn({ loading: true });
      const data = await api.listConnections();
      updateConn({ connections: data, error: null });
    } catch (err) {
      updateConn({
        error:
          err instanceof Error ? err.message : "Failed to load connections",
      });
    } finally {
      updateConn({ loading: false });
    }
  };

  const resetForm = () => {
    updateForm({
      showForm: false,
      editingConnection: null,
      formName: "",
      formType: "github",
      formUrl: "",
      formToken: "",
      formExcludeArchived: true,
    });
  };

  const handleEdit = (connection: Connection) => {
    updateForm({
      editingConnection: connection,
      formName: connection.name,
      formType: connection.type,
      formUrl: connection.url,
      formToken: "", // Don't pre-fill token for security
      formExcludeArchived: connection.exclude_archived,
      showForm: true,
    });
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    updateForm({ submitting: true });
    updateConn({ error: null });
    try {
      if (editingConnection) {
        // Update existing connection
        await api.updateConnection(editingConnection.id, {
          name: formName,
          type: formType,
          url: formUrl || getDefaultUrl(formType),
          token: formToken || undefined, // Only send token if provided
          exclude_archived: formExcludeArchived,
        });
      } else {
        // Create new connection
        await api.createConnection({
          name: formName,
          type: formType,
          url: formUrl || getDefaultUrl(formType),
          token: formToken,
          exclude_archived: formExcludeArchived,
        });
      }
      resetForm();
      loadConnections();
    } catch (err) {
      updateConn({
        error:
          err instanceof Error
            ? err.message
            : `Failed to ${editingConnection ? "update" : "create"} connection`,
      });
    } finally {
      updateForm({ submitting: false });
    }
  };

  const handleDelete = async (id: number) => {
    if (!confirm("Are you sure you want to delete this connection?")) return;
    try {
      await api.deleteConnection(id);
      loadConnections();
    } catch (err) {
      updateConn({
        error:
          err instanceof Error ? err.message : "Failed to delete connection",
      });
    }
  };

  const handleTest = async (id: number) => {
    updateUI({ testing: id, testResult: null });
    try {
      const result = await api.testConnection(id);
      updateUI({
        testResult: {
          id,
          status: result.status,
          message:
            result.status === "ok"
              ? result.message || "Connection validated successfully"
              : result.error,
        },
      });
    } catch (err) {
      updateUI({
        testResult: {
          id,
          status: "error",
          message: err instanceof Error ? err.message : "Test failed",
        },
      });
    } finally {
      updateUI({ testing: null });
    }
  };

  const handleSync = async (id: number) => {
    updateUI({
      syncing: id,
      syncResult: {
        id,
        status: "info",
        message: "Queueing sync job...",
      },
    });
    try {
      const result = await api.syncConnection(id);
      updateUI({
        syncResult: {
          id,
          status: "ok",
          message: result.message,
        },
      });
      // Reload repos after a short delay to show any newly added repos
      setTimeout(() => loadConnectionRepos(id), 2000);
    } catch (err) {
      updateUI({
        syncResult: {
          id,
          status: "error",
          message: err instanceof Error ? err.message : "Sync failed",
        },
      });
    } finally {
      updateUI({ syncing: null });
    }
  };

  const loadConnectionRepos = async (id: number) => {
    updateConn({
      connections: connectionsRef.current.map((c) =>
        c.id === id ? { ...c, reposLoading: true } : c
      ),
    });
    try {
      const repos = await api.listConnectionRepos(id);
      updateConn({
        connections: connectionsRef.current.map((c) =>
          c.id === id ? { ...c, repos, reposLoading: false } : c
        ),
      });
    } catch {
      // Intentionally ignored - fallback to default behavior
      updateConn({
        connections: connectionsRef.current.map((c) =>
          c.id === id ? { ...c, reposLoading: false } : c
        ),
      });
    }
  };

  const toggleExpand = (id: number) => {
    if (expandedConnection === id) {
      updateConn({ expandedConnection: null });
    } else {
      updateConn({ expandedConnection: id });
      const conn = connections.find((c) => c.id === id);
      if (!conn?.repos) {
        loadConnectionRepos(id);
      }
    }
  };

  const getDefaultUrl = (type: string) => {
    switch (type) {
      case "github":
        return "https://github.com";
      case "gitlab":
        return "https://gitlab.com";
      case "gitea":
        return "";
      case "bitbucket":
        return "https://bitbucket.org";
      default:
        return "";
    }
  };

  const getTypeIcon = (type: string) => {
    // Using consistent icons since we can't use brand-specific ones
    switch (type) {
      case "github":
      case "github_enterprise":
        return <Github className="h-5 w-5" />;
      case "gitlab":
        return <Gitlab className="h-5 w-5" />;
      case "gitea":
        return <FolderGit2 className="h-5 w-5" />;
      case "bitbucket":
        return <FolderGit2 className="h-5 w-5" />;
      default:
        return <FolderGit2 className="h-5 w-5" />;
    }
  };

  return (
    <div className="h-full overflow-auto">
      <div className="container mx-auto max-w-full px-4 py-6">
        <div className="mx-auto max-w-6xl">
          <div className="mb-4 flex items-center justify-between sm:mb-6">
            <div className="flex items-center gap-2 sm:gap-3">
              <Link2 className="h-5 w-5 text-gray-600 dark:text-gray-400 sm:h-6 sm:w-6" />
              <h1 className="text-xl font-bold sm:text-2xl">Connections</h1>
            </div>
            <div className="flex items-center gap-2">
              <div
                className="pointer-events-none flex select-none items-center gap-2 rounded-lg px-2.5 py-1.5 text-sm font-medium opacity-0 sm:px-3 sm:py-2"
                aria-hidden="true"
              >
                <RefreshCw className="h-4 w-4" />
                <span className="hidden sm:inline">
                  Easter Egg! Ths is only meant to balance the layout. c:
                </span>
              </div>
              {settingsLoaded && !readonlyMode && (
                <button
                  onClick={() => {
                    if (showForm) {
                      resetForm();
                    } else {
                      updateForm({ showForm: true });
                    }
                  }}
                  className={`flex items-center gap-1.5 rounded-lg px-2.5 py-1.5 text-sm font-medium transition-colors sm:gap-2 sm:px-3 sm:py-2 ${
                    showForm
                      ? "border border-gray-300 text-gray-600 hover:bg-gray-100 dark:border-gray-600 dark:text-gray-300 dark:hover:bg-gray-700"
                      : "border border-blue-500 text-blue-600 hover:bg-blue-50 dark:border-blue-400 dark:text-blue-400 dark:hover:bg-blue-900/30"
                  }`}
                >
                  {showForm ? (
                    <X className="h-4 w-4" />
                  ) : (
                    <Plus className="h-4 w-4" />
                  )}
                  <span className="hidden sm:inline">
                    {showForm ? "Cancel" : "Add Connection"}
                  </span>
                  <span className="sm:hidden">
                    {showForm ? "Cancel" : "Add"}
                  </span>
                </button>
              )}
            </div>
          </div>

          {readonlyMode && !hideReadOnlyBanner && (
            <div className="mb-4 rounded-lg border border-amber-200 bg-amber-50 p-3 text-amber-700 dark:border-amber-800 dark:bg-amber-900/20 dark:text-amber-400 sm:p-4">
              <div className="flex items-start gap-3">
                <AlertCircle className="mt-0.5 h-5 w-5 flex-shrink-0 sm:h-5 sm:w-5" />
                <div>
                  <p className="text-sm font-bold">Read-only mode enabled</p>
                  <p className="mt-1 text-sm text-amber-700 dark:text-amber-300">
                    Connections are managed via configuration file. Create,
                    update, and delete operations are disabled.
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

          {showForm && !readonlyMode && (
            <ConnectionForm
              formName={formName}
              formType={formType}
              formUrl={formUrl}
              formToken={formToken}
              formExcludeArchived={formExcludeArchived}
              submitting={submitting}
              editingConnection={editingConnection}
              updateForm={updateForm}
              getDefaultUrl={getDefaultUrl}
              onSubmit={handleSubmit}
            />
          )}

          {loading ? (
            <div className="py-16 text-center">
              <Loader2 className="mx-auto h-8 w-8 animate-spin text-blue-600" />
              <p className="mt-3 text-gray-500 dark:text-gray-400">
                Loading connections...
              </p>
            </div>
          ) : connections.length === 0 ? (
            <div className="rounded-xl border border-gray-200 bg-white py-16 text-center dark:border-gray-700 dark:bg-gray-800">
              <Link2 className="mx-auto mb-4 h-12 w-12 text-gray-300 dark:text-gray-600" />
              <p className="mb-2 text-gray-500 dark:text-gray-400">
                No connections configured
              </p>
              <p className="text-sm text-gray-400 dark:text-gray-500">
                Add a code host connection to start indexing repositories
              </p>
            </div>
          ) : (
            <div className="space-y-4">
              {connections.map((conn) => (
                <div
                  key={conn.id}
                  className="rounded-xl border border-gray-200 bg-white shadow-sm dark:border-gray-700 dark:bg-gray-800"
                >
                  <div className="p-4 sm:p-5">
                    <div className="flex items-center justify-between gap-2 sm:gap-4">
                      <div className="flex min-w-0 flex-1 items-center gap-2 sm:gap-4">
                        <div className="flex-shrink-0 rounded-lg bg-gray-100 p-2 text-gray-600 dark:bg-gray-700 dark:text-gray-300 sm:p-2.5">
                          {getTypeIcon(conn.type)}
                        </div>
                        <div className="min-w-0 flex-1">
                          <h3 className="truncate text-sm font-medium sm:text-base">
                            {conn.name}
                          </h3>
                          <p className="truncate text-xs text-gray-500 dark:text-gray-400">
                            <span className="capitalize">{conn.type}</span> •{" "}
                            <span className="hidden sm:inline">{conn.url}</span>
                            <span className="sm:hidden">
                              {new URL(conn.url).hostname}
                            </span>
                            {conn.exclude_archived && (
                              <span className="ml-2 text-amber-600 dark:text-amber-400">
                                • Excludes archived
                              </span>
                            )}
                          </p>
                        </div>
                      </div>
                      <div className="flex flex-shrink-0 items-center gap-1 sm:gap-2">
                        <button
                          onClick={() => handleTest(conn.id)}
                          disabled={testing === conn.id}
                          className="flex items-center gap-1.5 rounded-lg border border-gray-200 bg-white p-1.5 text-xs shadow-sm transition-all hover:bg-gray-50 focus:outline-none focus:ring-2 focus:ring-blue-500/20 dark:border-gray-600 dark:bg-gray-700 dark:hover:bg-gray-600 sm:px-3 sm:py-1.5 sm:text-sm"
                          title="Test connection"
                        >
                          {testing === conn.id ? (
                            <Loader2 className="h-4 w-4 animate-spin" />
                          ) : (
                            <TestTube className="h-4 w-4 text-gray-500 dark:text-gray-400" />
                          )}
                          <span className="hidden sm:inline">
                            {testing === conn.id ? "Testing..." : "Test"}
                          </span>
                        </button>
                        <button
                          onClick={() => toggleExpand(conn.id)}
                          className="flex items-center gap-1.5 rounded-lg border border-gray-200 bg-white p-1.5 text-xs shadow-sm transition-all hover:bg-gray-50 focus:outline-none focus:ring-2 focus:ring-blue-500/20 dark:border-gray-600 dark:bg-gray-700 dark:hover:bg-gray-600 sm:px-3 sm:py-1.5 sm:text-sm"
                          title={
                            expandedConnection === conn.id
                              ? "Hide repos"
                              : "Show repos"
                          }
                        >
                          {expandedConnection === conn.id ? (
                            <ChevronUp className="h-4 w-4 text-gray-500 dark:text-gray-400" />
                          ) : (
                            <ChevronDown className="h-4 w-4 text-gray-500 dark:text-gray-400" />
                          )}
                          <span className="hidden sm:inline">
                            {expandedConnection === conn.id ? "Hide" : "Repos"}
                          </span>
                        </button>
                        <ConnectionActionMenu
                          connection={conn}
                          onSync={handleSync}
                          onEdit={handleEdit}
                          onDelete={handleDelete}
                          syncing={syncing}
                          readonly={readonlyMode}
                        />
                      </div>
                    </div>
                    {testResult && testResult.id === conn.id && (
                      <div
                        className={`mt-4 flex items-center gap-2 rounded-lg p-3 text-sm ${
                          testResult.status === "ok"
                            ? "bg-green-50 text-green-700 dark:bg-green-900/20 dark:text-green-400"
                            : "bg-red-50 text-red-700 dark:bg-red-900/20 dark:text-red-400"
                        }`}
                      >
                        {testResult.status === "ok" ? (
                          <CheckCircle2 className="h-4 w-4" />
                        ) : (
                          <AlertCircle className="h-4 w-4" />
                        )}
                        {testResult.message}
                      </div>
                    )}
                    {syncResult && syncResult.id === conn.id && (
                      <div
                        className={`mt-4 flex items-center gap-2 rounded-lg p-3 text-sm ${
                          syncResult.status === "ok"
                            ? "bg-green-50 text-green-700 dark:bg-green-900/20 dark:text-green-400"
                            : syncResult.status === "info"
                              ? "bg-blue-50 text-blue-700 dark:bg-blue-900/20 dark:text-blue-400"
                              : "bg-red-50 text-red-700 dark:bg-red-900/20 dark:text-red-400"
                        }`}
                      >
                        {syncResult.status === "ok" ? (
                          <CheckCircle2 className="h-4 w-4" />
                        ) : syncResult.status === "info" ? (
                          <Loader2 className="h-4 w-4 flex-shrink-0 animate-spin sm:h-5 sm:w-5" />
                        ) : (
                          <AlertCircle className="h-4 w-4" />
                        )}
                        {syncResult.message}
                      </div>
                    )}
                    {/* Indexing progress indicator - replaces syncResult when active */}
                    {indexingStatus.has(conn.id) &&
                      (() => {
                        const status = indexingStatus.get(conn.id)!;
                        const inProgress = status.pending + status.running;
                        const progress =
                          status.total > 0
                            ? Math.round(
                                ((status.completed + status.failed) /
                                  status.total) *
                                  100
                              )
                            : 0;

                        // Show progress bar while indexing
                        if (inProgress > 0) {
                          // Clear any syncResult for this connection when indexing is active
                          if (
                            syncResult?.id === conn.id &&
                            syncResult?.status === "info"
                          ) {
                            setTimeout(() => updateUI({ syncResult: null }), 0);
                          }
                          return (
                            <div className="mt-4 rounded-lg bg-purple-50 p-3 text-purple-700 dark:bg-purple-900/20 dark:text-purple-400">
                              <div className="flex items-center gap-2 text-sm">
                                <Loader2 className="h-4 w-4 animate-spin" />
                                <span>
                                  Indexing repositories:{" "}
                                  {status.completed + status.failed}/
                                  {status.total}
                                  {status.running > 0 &&
                                    ` (${status.running} running)`}
                                  {status.failed > 0 &&
                                    ` (${status.failed} failed)`}
                                </span>
                              </div>
                              <div className="mt-2 h-1.5 overflow-hidden rounded-full bg-purple-200 dark:bg-purple-800">
                                <div
                                  className="h-full bg-purple-600 transition-all duration-300 dark:bg-purple-400"
                                  style={{ width: `${progress}%` }}
                                />
                              </div>
                            </div>
                          );
                        }

                        // Show summary when all jobs are done (only if there are failed jobs)
                        if (status.failed > 0) {
                          return (
                            <div className="mt-4 rounded-lg bg-amber-50 p-3 text-amber-700 dark:bg-amber-900/20 dark:text-amber-400">
                              <div className="flex items-center gap-2 text-sm">
                                <AlertCircle className="h-4 w-4" />
                                <span>
                                  Indexing completed with errors:{" "}
                                  {status.completed} succeeded, {status.failed}{" "}
                                  failed
                                </span>
                              </div>
                            </div>
                          );
                        }

                        return null;
                      })()}
                  </div>

                  {/* Expanded repos list */}
                  {expandedConnection === conn.id && (
                    <div className="border-t border-gray-100 bg-gray-50/50 p-3 dark:border-gray-700 dark:bg-gray-800/50 sm:p-5">
                      {conn.reposLoading ? (
                        <div className="py-6 text-center">
                          <Loader2 className="mx-auto h-5 w-5 animate-spin text-blue-600" />
                          <p className="mt-2 text-sm text-gray-500">
                            Loading repositories...
                          </p>
                        </div>
                      ) : conn.repos && conn.repos.length > 0 ? (
                        <div>
                          <h4 className="mb-3 flex items-center gap-2 text-sm font-medium text-gray-700 dark:text-gray-300">
                            <FolderGit2 className="h-4 w-4" />
                            Repositories ({conn.repos.length})
                          </h4>
                          <div className="max-h-64 space-y-2 overflow-y-auto">
                            {conn.repos.map((repo) => (
                              <div
                                key={repo.id}
                                className="flex items-center justify-between gap-2 rounded-lg border border-gray-100 bg-white p-2 dark:border-gray-600 dark:bg-gray-700/50 sm:p-3"
                              >
                                <div className="flex min-w-0 items-center gap-2">
                                  <GitBranch className="h-4 w-4 flex-shrink-0 text-gray-400" />
                                  <span
                                    className={`truncate text-xs sm:text-sm ${repo.excluded ? "text-gray-400 dark:text-gray-500" : ""}`}
                                  >
                                    {repo.name}
                                  </span>
                                </div>
                                <span
                                  className={`inline-flex flex-shrink-0 items-center gap-1 rounded-full px-2 py-0.5 text-xs ${
                                    repo.excluded
                                      ? "bg-gray-200 text-gray-600 dark:bg-gray-700 dark:text-gray-400"
                                      : repo.status === "indexed"
                                        ? "bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400"
                                        : repo.status === "indexing" ||
                                            repo.status === "cloning"
                                          ? "bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400"
                                          : repo.status === "pending"
                                            ? "bg-yellow-100 text-yellow-700 dark:bg-yellow-900/30 dark:text-yellow-400"
                                            : repo.status === "failed"
                                              ? "bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-400"
                                              : "bg-gray-100 text-gray-600 dark:bg-gray-600 dark:text-gray-300"
                                  }`}
                                >
                                  {repo.excluded ? (
                                    <>
                                      <EyeOff className="h-3 w-3" />
                                      excluded
                                    </>
                                  ) : (
                                    <>
                                      {repo.status === "indexed" && (
                                        <CheckCircle2 className="h-3 w-3" />
                                      )}
                                      {(repo.status === "indexing" ||
                                        repo.status === "cloning") && (
                                        <Loader2 className="h-3 w-3 animate-spin" />
                                      )}
                                      {repo.status === "pending" && (
                                        <Clock className="h-3 w-3" />
                                      )}
                                      {repo.status === "failed" && (
                                        <AlertCircle className="h-3 w-3" />
                                      )}
                                      {repo.status || "pending"}
                                    </>
                                  )}
                                </span>
                              </div>
                            ))}
                          </div>
                        </div>
                      ) : (
                        <div className="py-6 text-center text-sm text-gray-500">
                          <FolderGit2 className="mx-auto mb-2 h-8 w-8 text-gray-300 dark:text-gray-600" />
                          <span className="hidden sm:inline">
                            No repositories synced yet. Click "Sync Repos" to
                            fetch repositories.
                          </span>
                          <span className="sm:hidden">
                            No repositories synced yet. Tap the menu to sync.
                          </span>
                        </div>
                      )}
                    </div>
                  )}
                </div>
              ))}
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
