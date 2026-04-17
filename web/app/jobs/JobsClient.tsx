"use client";

import { useState, useEffect, useCallback } from "react";
import { api, Job, Connection } from "@/lib/api";
import {
  Zap,
  RefreshCw,
  BookOpen,
  RefreshCcw,
  Loader2,
  CheckCircle2,
  Clock,
  AlertCircle,
  XCircle,
  Filter,
  Search,
  ChevronDown,
  ChevronLeft,
  ChevronRight,
  ChevronsLeft,
  ChevronsRight,
  BrushCleaning,
} from "lucide-react";

const PAGE_SIZE = 15;

// Redact sensitive fields from payload for display
// eslint-disable-next-line @typescript-eslint/no-explicit-any
function redactSensitiveFields(obj: any): any {
  if (obj === null || obj === undefined) return obj;
  if (typeof obj !== "object") return obj;
  if (Array.isArray(obj)) return obj.map(redactSensitiveFields);

  const sensitiveKeys = [
    "user_tokens",
    "user_token",
    "token",
    "password",
    "secret",
    "api_key",
    "apikey",
  ];
  const result: Record<string, unknown> = {};

  for (const [key, value] of Object.entries(obj)) {
    const lowerKey = key.toLowerCase();
    if (sensitiveKeys.some((sk) => lowerKey.includes(sk))) {
      // Redact sensitive values
      if (typeof value === "object" && value !== null) {
        // For maps like user_tokens, show keys but redact values
        result[key] = Object.fromEntries(
          Object.keys(value as object).map((k) => [k, "***REDACTED***"])
        );
      } else if (value) {
        result[key] = "***REDACTED***";
      }
    } else {
      result[key] = redactSensitiveFields(value);
    }
  }

  return result;
}

export default function JobsClient() {
  const [jobsState, setJobsState] = useState({
    jobs: [] as Job[],
    connections: new Map<number, string>(),
    loading: true,
    error: null as string | null,
    typeFilter: "",
    statusFilter: "",
    repoSearch: "",
    totalCount: 0,
    allJobsCount: 0,
    currentPage: 1,
    hasMore: false,
  });

  const {
    jobs,
    connections,
    loading,
    error,
    typeFilter,
    statusFilter,
    repoSearch,
    totalCount,
    allJobsCount,
    currentPage,
  } = jobsState;

  const setCurrentPage = (val: number | ((p: number) => number)) =>
    setJobsState((prev) => ({
      ...prev,
      currentPage: typeof val === "function" ? val(prev.currentPage) : val,
    }));
  const setError = (val: string | null) =>
    setJobsState((prev) => ({ ...prev, error: val }));

  // Known job types and statuses
  const knownTypes = ["index", "sync", "replace", "cleanup"];
  const knownStatuses = ["pending", "running", "completed", "failed"];

  // Load connections once to map IDs to names
  useEffect(() => {
    const loadConnections = async () => {
      try {
        const data = await api.listConnections();
        const connMap = new Map<number, string>();
        data.forEach((conn: Connection) => {
          connMap.set(conn.id, conn.name);
        });
        setJobsState((prev) => ({ ...prev, connections: connMap }));
      } catch {
        // Ignore errors loading connections
      }
    };
    loadConnections();
  }, []);

  const loadJobs = useCallback(
    async (showLoading = true) => {
      try {
        if (showLoading) setJobsState((prev) => ({ ...prev, loading: true }));

        // Build the filter options
        const filterOpts = {
          type: typeFilter || undefined,
          status: statusFilter || undefined,
          repo: repoSearch || undefined,
        };

        // Load filtered jobs and total count in parallel
        const [data, allData] = await Promise.all([
          api.listJobs({
            ...filterOpts,
            limit: PAGE_SIZE,
            offset: (currentPage - 1) * PAGE_SIZE,
          }),
          // Fetch total count WITHOUT filters to show X/total
          api.listJobs({ limit: 1, offset: 0 }),
        ]);

        setJobsState((prev) => ({
          ...prev,
          jobs: data.jobs || [],
          totalCount: data.total_count || 0,
          hasMore: data.has_more || false,
          allJobsCount: allData.total_count || 0,
          error: null,
          loading: false,
        }));
      } catch (err) {
        setJobsState((prev) => ({
          ...prev,
          error: err instanceof Error ? err.message : "Failed to load jobs",
          loading: false,
        }));
      }
    },
    [typeFilter, statusFilter, repoSearch, currentPage]
  );

  useEffect(() => {
    loadJobs();
    // Poll for updates every 5 seconds (silent refresh, no loading spinner)
    const interval = setInterval(() => {
      loadJobs(false);
    }, 5000);
    return () => clearInterval(interval);
  }, [loadJobs]);

  const handleCancel = async (id: string) => {
    try {
      await api.cancelJob(id);
      loadJobs();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to cancel job");
    }
  };

  const getStatusBadge = (status: string) => {
    const configs: Record<string, { bg: string; icon: React.ReactNode }> = {
      pending: {
        bg: "bg-yellow-100 text-yellow-800 dark:bg-yellow-900/50 dark:text-yellow-300",
        icon: <Clock className="h-3.5 w-3.5" />,
      },
      running: {
        bg: "bg-blue-100 text-blue-800 dark:bg-blue-900/50 dark:text-blue-300",
        icon: <Loader2 className="h-3.5 w-3.5 animate-spin" />,
      },
      completed: {
        bg: "bg-green-100 text-green-800 dark:bg-green-900/50 dark:text-green-300",
        icon: <CheckCircle2 className="h-3.5 w-3.5" />,
      },
      failed: {
        bg: "bg-red-100 text-red-800 dark:bg-red-900/50 dark:text-red-300",
        icon: <AlertCircle className="h-3.5 w-3.5" />,
      },
    };
    const config = configs[status] || configs.pending;
    return (
      <span
        className={`inline-flex items-center gap-1.5 rounded-full px-2.5 py-1 text-xs font-medium ${config.bg}`}
      >
        {config.icon}
        {status}
      </span>
    );
  };

  const getTypeIcon = (type: string) => {
    switch (type) {
      case "index":
        return <BookOpen className="h-5 w-5" />;
      case "replace":
        return <RefreshCcw className="h-5 w-5" />;
      case "sync":
        return <RefreshCw className="h-5 w-5" />;
      case "cleanup":
        return <BrushCleaning className="h-5 w-5" />;
      default:
        return <Zap className="h-5 w-5" />;
    }
  };

  const formatTime = (dateStr: string) => {
    const date = new Date(dateStr);
    return date.toLocaleString();
  };

  const formatDuration = (startedAt?: string, completedAt?: string) => {
    if (!startedAt) return null;
    const start = new Date(startedAt).getTime();
    const end = completedAt ? new Date(completedAt).getTime() : Date.now();
    const durationMs = end - start;

    if (durationMs < 1000) return `${durationMs}ms`;
    if (durationMs < 60000) return `${(durationMs / 1000).toFixed(1)}s`;
    const minutes = Math.floor(durationMs / 60000);
    const seconds = Math.floor((durationMs % 60000) / 1000);
    return `${minutes}m ${seconds}s`;
  };

  const getPayloadSummary = (job: Job) => {
    if (!job.payload) return null;
    const p = job.payload;

    if (job.type === "index" && p.repo_name) {
      const connName = p.connection_id
        ? connections.get(p.connection_id)
        : null;
      return connName
        ? `Repository: ${p.repo_name} (${connName})`
        : `Repository: ${p.repo_name}`;
    }
    if (job.type === "cleanup" && p.repository_name) {
      return `Repository: ${p.repository_name}`;
    }
    if (job.type === "sync" && p.connection_id) {
      const connName = connections.get(p.connection_id);
      return connName
        ? `Connection: ${connName}`
        : `Connection ID: ${p.connection_id}`;
    }
    if (job.type === "replace") {
      return `"${p.search_pattern || p.old_pattern}" → "${p.replace_with || p.new_pattern}"`;
    }
    return null;
  };

  const getResultSummary = (job: Job) => {
    if (!job.result) return null;
    const r = job.result;

    if (r.files_modified !== undefined) {
      return `${r.files_modified} files modified`;
    }
    if (r.repos_synced !== undefined) {
      return `${r.repos_synced} repos synced`;
    }
    if (r.indexed !== undefined) {
      return r.indexed ? "Successfully indexed" : "Indexing skipped";
    }
    return null;
  };

  return (
    <div className="h-full overflow-auto">
      <div className="container mx-auto max-w-full px-4 py-6">
        <div className="mx-auto max-w-6xl">
          <div className="mb-4 flex items-center justify-between sm:mb-6">
            <div className="flex items-center gap-2 sm:gap-3">
              <Zap className="h-5 w-5 text-gray-600 dark:text-gray-400 sm:h-6 sm:w-6" />
              <h1 className="text-xl font-bold sm:text-2xl">Jobs</h1>
            </div>
            <button
              type="button"
              onClick={() => loadJobs()}
              className="flex items-center gap-2 rounded-lg border border-gray-200 bg-white px-2.5 py-1.5 text-sm font-medium shadow-sm transition-all hover:bg-gray-50 focus:outline-none focus:ring-2 focus:ring-blue-500/20 focus:ring-offset-2 dark:border-gray-700 dark:bg-gray-800 dark:hover:bg-gray-700 sm:px-3 sm:py-2"
            >
              <RefreshCw className="h-4 w-4 text-gray-500 dark:text-gray-400" />
              <span className="hidden sm:inline">Refresh</span>
            </button>
          </div>

          {error && (
            <div className="mb-4 flex items-center gap-2 rounded-xl border border-red-200 bg-red-50 p-3 text-sm text-red-700 dark:border-red-800 dark:bg-red-900/20 dark:text-red-400 sm:gap-3 sm:p-4">
              <AlertCircle className="h-4 w-4 flex-shrink-0 sm:h-5 sm:w-5" />
              {error}
            </div>
          )}

          {/* Search and Filter Controls */}
          <div className="mb-5 flex flex-col gap-3 sm:flex-row sm:items-center sm:gap-4">
            <div className="relative flex-1">
              <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-gray-400" />
              <label htmlFor="job-search" className="sr-only">
                Search jobs by repository
              </label>
              <input
                id="job-search"
                type="text"
                value={repoSearch}
                onChange={(e) =>
                  setJobsState((prev) => ({
                    ...prev,
                    repoSearch: e.target.value,
                    currentPage: 1,
                  }))
                }
                placeholder="Search by repository..."
                className="w-full rounded-lg border border-gray-200 bg-white py-2 pl-9 pr-3 text-sm shadow-sm focus:border-blue-500 focus:outline-none dark:border-gray-700 dark:bg-gray-800 dark:focus:border-blue-500"
              />
            </div>
            <div className="flex flex-shrink-0 flex-wrap items-center gap-2 sm:flex-nowrap">
              <div className="flex flex-1 items-center gap-2 sm:flex-initial">
                <Filter className="hidden h-4 w-4 text-gray-400 sm:block" />
                <select
                  value={typeFilter}
                  onChange={(e) =>
                    setJobsState((prev) => ({
                      ...prev,
                      typeFilter: e.target.value,
                      currentPage: 1,
                    }))
                  }
                  className="w-[90px] appearance-none rounded-lg border border-gray-200 bg-white bg-[url('data:image/svg+xml;charset=UTF-8,%3Csvg%20xmlns%3D%22http%3A%2F%2Fwww.w3.org%2F2000%2Fsvg%22%20viewBox%3D%220%200%2020%2020%22%20fill%3D%22%236b7280%22%3E%3Cpath%20fill-rule%3D%22evenodd%22%20d%3D%22M5.293%207.293a1%201%200%20011.414%200L10%2010.586l3.293-3.293a1%201%200%20111.414%201.414l-4%204a1%201%200%2001-1.414%200l-4-4a1%201%200%20010-1.414z%22%20clip-rule%3D%22evenodd%22%2F%3E%3C%2Fsvg%3E')] bg-[length:16px_16px] bg-[position:right_8px_center] bg-no-repeat py-2 pl-2 pr-7 text-xs shadow-sm focus:border-blue-500 focus:outline-none dark:border-gray-700 dark:bg-gray-800 dark:focus:border-blue-500 sm:w-[110px] sm:pl-3 sm:pr-8 sm:text-sm"
                >
                  <option value="">All Types</option>
                  {knownTypes.map((type) => (
                    <option key={type} value={type}>
                      {type.charAt(0).toUpperCase() + type.slice(1)}
                    </option>
                  ))}
                </select>
                <select
                  value={statusFilter}
                  onChange={(e) =>
                    setJobsState((prev) => ({
                      ...prev,
                      statusFilter: e.target.value,
                      currentPage: 1,
                    }))
                  }
                  className="w-[105px] appearance-none rounded-lg border border-gray-200 bg-white bg-[url('data:image/svg+xml;charset=UTF-8,%3Csvg%20xmlns%3D%22http%3A%2F%2Fwww.w3.org%2F2000%2Fsvg%22%20viewBox%3D%220%200%2020%2020%22%20fill%3D%22%236b7280%22%3E%3Cpath%20fill-rule%3D%22evenodd%22%20d%3D%22M5.293%207.293a1%201%200%20011.414%200L10%2010.586l3.293-3.293a1%201%200%20111.414%201.414l-4%204a1%201%200%2001-1.414%200l-4-4a1%201%200%20010-1.414z%22%20clip-rule%3D%22evenodd%22%2F%3E%3C%2Fsvg%3E')] bg-[length:16px_16px] bg-[position:right_8px_center] bg-no-repeat py-2 pl-2 pr-7 text-xs shadow-sm focus:border-blue-500 focus:outline-none dark:border-gray-700 dark:bg-gray-800 dark:focus:border-blue-500 sm:w-[130px] sm:pl-3 sm:pr-8 sm:text-sm"
                >
                  <option value="">All statuses</option>
                  {knownStatuses.map((status) => (
                    <option key={status} value={status}>
                      {status.charAt(0).toUpperCase() + status.slice(1)}
                    </option>
                  ))}
                </select>
              </div>
              <div className="whitespace-nowrap rounded-lg bg-gray-100 px-2 py-1.5 text-xs text-gray-500 dark:bg-gray-800 dark:text-gray-400 sm:px-3 sm:text-sm">
                {totalCount}/{allJobsCount}
              </div>
            </div>
          </div>

          {loading ? (
            <div className="py-16 text-center">
              <Loader2 className="mx-auto h-8 w-8 animate-spin text-blue-600" />
              <p className="mt-3 text-gray-500 dark:text-gray-400">
                Loading jobs...
              </p>
            </div>
          ) : jobs.length === 0 ? (
            <div className="rounded-xl border border-gray-200 bg-white py-16 text-center dark:border-gray-700 dark:bg-gray-800">
              <Zap className="mx-auto mb-4 h-12 w-12 text-gray-300 dark:text-gray-600" />
              <p className="text-gray-500 dark:text-gray-400">No jobs found</p>
            </div>
          ) : (
            <div className="space-y-3">
              {jobs.map((job) => (
                <div
                  key={job.id}
                  className="rounded-xl border border-gray-200 bg-white p-3 shadow-sm transition-shadow hover:shadow-md dark:border-gray-700 dark:bg-gray-800 sm:p-5"
                >
                  <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
                    <div className="flex items-start gap-3 sm:gap-4">
                      <div className="rounded-lg bg-gray-100 p-2 text-gray-600 dark:bg-gray-700 dark:text-gray-300 sm:p-2.5">
                        {getTypeIcon(job.type)}
                      </div>
                      <div className="min-w-0 flex-1">
                        <div className="flex flex-wrap items-center gap-2 sm:gap-3">
                          <span className="text-sm font-semibold capitalize">
                            {job.type} Job
                          </span>
                          {getStatusBadge(job.status)}
                          {job.started_at && (
                            <span className="rounded bg-gray-100 px-2 py-0.5 text-xs text-gray-500 dark:bg-gray-700 dark:text-gray-400">
                              {formatDuration(job.started_at, job.completed_at)}
                            </span>
                          )}
                        </div>

                        {/* Progress bar for running jobs */}
                        {job.status === "running" &&
                          job.progress &&
                          job.progress.total > 0 && (
                            <div className="mt-2">
                              <div className="mb-1 flex items-center justify-between text-xs text-gray-500 dark:text-gray-400">
                                <span className="mr-2 truncate">
                                  {job.progress.message || "Processing..."}
                                </span>
                                <span className="flex-shrink-0">
                                  {Math.round(
                                    (job.progress.current /
                                      job.progress.total) *
                                      100
                                  )}
                                  %
                                </span>
                              </div>
                              <div className="h-2 w-full rounded-full bg-gray-200 dark:bg-gray-700">
                                <div
                                  className="h-2 rounded-full bg-blue-600 transition-all duration-300"
                                  style={{
                                    width: `${Math.min(100, (job.progress.current / job.progress.total) * 100)}%`,
                                  }}
                                />
                              </div>
                            </div>
                          )}

                        {/* Payload summary */}
                        {getPayloadSummary(job) && (
                          <p className="mt-1.5 truncate text-sm text-gray-600 dark:text-gray-300">
                            {getPayloadSummary(job)}
                          </p>
                        )}

                        {/* Result summary */}
                        {getResultSummary(job) && (
                          <p className="mt-1 text-sm text-green-600 dark:text-green-400">
                            ✓ {getResultSummary(job)}
                          </p>
                        )}

                        {/* Job ID - hidden on mobile */}
                        <p className="mt-2 hidden truncate font-mono text-xs text-gray-400 dark:text-gray-500 sm:block">
                          ID: {job.id}
                        </p>
                      </div>
                    </div>
                    <div className="flex flex-shrink-0 items-center justify-between gap-2 pl-11 sm:ml-4 sm:flex-col sm:items-end sm:justify-start sm:gap-0 sm:pl-0">
                      <div className="text-left sm:text-right">
                        <p className="text-xs text-gray-500 dark:text-gray-400">
                          {formatTime(job.created_at)}
                        </p>
                      </div>
                      {job.status === "pending" && (
                        <button
                          onClick={() => handleCancel(job.id)}
                          className="inline-flex items-center gap-1.5 rounded-lg border border-red-500 px-2 py-1 text-xs font-medium text-red-600 transition-colors hover:bg-red-50 dark:border-red-400 dark:text-red-400 dark:hover:bg-red-900/20 sm:mt-2 sm:px-3 sm:py-1.5 sm:text-sm"
                        >
                          <XCircle className="h-4 w-4" />
                          Cancel
                        </button>
                      )}
                    </div>
                  </div>
                  {job.error && (
                    <div className="mt-4 flex items-start gap-2 rounded-lg bg-red-50 p-3 text-sm text-red-700 dark:bg-red-900/20 dark:text-red-400">
                      <AlertCircle className="mt-0.5 h-4 w-4 flex-shrink-0" />
                      <span>{job.error}</span>
                    </div>
                  )}
                  {(job.payload || job.result) && (
                    <details className="group mt-4">
                      <summary className="flex cursor-pointer items-center gap-1 text-sm text-gray-500 hover:text-gray-700 dark:hover:text-gray-300">
                        <ChevronDown className="h-4 w-4 transition-transform group-open:rotate-180" />
                        View details
                      </summary>
                      <div className="mt-2 flex flex-col gap-4">
                        {job.payload && (
                          <div>
                            <p className="mb-1 text-xs font-medium text-gray-500 dark:text-gray-400">
                              Payload
                            </p>
                            <pre className="max-w-full overflow-x-auto rounded-lg bg-gray-100 p-3 font-mono text-xs text-gray-800 dark:bg-gray-900 dark:text-gray-200">
                              {JSON.stringify(
                                redactSensitiveFields(job.payload),
                                null,
                                2
                              )}
                            </pre>
                          </div>
                        )}
                        {job.result && (
                          <div>
                            <p className="mb-1 text-xs font-medium text-gray-500 dark:text-gray-400">
                              Result
                            </p>
                            <pre className="overflow-x-auto rounded-lg bg-gray-100 p-3 font-mono text-xs text-gray-800 dark:bg-gray-900 dark:text-gray-200">
                              {JSON.stringify(job.result, null, 2)}
                            </pre>
                          </div>
                        )}
                      </div>
                    </details>
                  )}
                </div>
              ))}
            </div>
          )}

          {/* Pagination */}
          {totalCount > PAGE_SIZE &&
            (() => {
              const totalPages = Math.ceil(totalCount / PAGE_SIZE);
              return (
                <div className="mt-6 flex flex-col items-center justify-between gap-3 sm:flex-row">
                  <div className="text-center text-sm text-gray-500 dark:text-gray-400 sm:text-left">
                    {(currentPage - 1) * PAGE_SIZE + 1}-
                    {Math.min(currentPage * PAGE_SIZE, totalCount)} of{" "}
                    {totalCount}
                  </div>
                  <div className="flex items-center gap-1">
                    <button
                      onClick={() => setCurrentPage(1)}
                      disabled={currentPage === 1}
                      className="hidden rounded-lg border border-gray-200 p-2 text-sm transition-colors hover:bg-gray-100 disabled:cursor-not-allowed disabled:opacity-50 dark:border-gray-700 dark:hover:bg-gray-700 sm:flex"
                      title="First page"
                    >
                      <ChevronsLeft className="h-4 w-4" />
                    </button>
                    <button
                      onClick={() => setCurrentPage((p) => Math.max(1, p - 1))}
                      disabled={currentPage === 1}
                      className="rounded-lg border border-gray-200 p-2 text-sm transition-colors hover:bg-gray-100 disabled:cursor-not-allowed disabled:opacity-50 dark:border-gray-700 dark:hover:bg-gray-700"
                      title="Previous page"
                    >
                      <ChevronLeft className="h-4 w-4" />
                    </button>
                    {/* Mobile page indicator */}
                    <span className="px-2 text-sm text-gray-600 dark:text-gray-400 sm:hidden">
                      {currentPage} / {totalPages}
                    </span>
                    {/* Desktop page buttons */}
                    <div className="mx-1 hidden items-center gap-1 sm:flex">
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
                              onClick={() => setCurrentPage(pageNum)}
                              className={`h-9 w-9 rounded-lg border text-sm font-medium transition-colors ${
                                currentPage === pageNum
                                  ? "border-blue-600 bg-blue-600 text-white shadow-sm"
                                  : "border-gray-200 hover:bg-gray-100 dark:border-gray-700 dark:hover:bg-gray-700"
                              }`}
                            >
                              {pageNum}
                            </button>
                          );
                        }
                      )}
                    </div>
                    <button
                      onClick={() =>
                        setCurrentPage((p) => Math.min(totalPages, p + 1))
                      }
                      disabled={currentPage === totalPages}
                      className="rounded-lg border border-gray-200 p-2 text-sm transition-colors hover:bg-gray-100 disabled:cursor-not-allowed disabled:opacity-50 dark:border-gray-700 dark:hover:bg-gray-700"
                      title="Next page"
                    >
                      <ChevronRight className="h-4 w-4" />
                    </button>
                    <button
                      onClick={() => setCurrentPage(totalPages)}
                      disabled={currentPage === totalPages}
                      className="hidden rounded-lg border border-gray-200 p-2 text-sm transition-colors hover:bg-gray-100 disabled:cursor-not-allowed disabled:opacity-50 dark:border-gray-700 dark:hover:bg-gray-700 sm:flex"
                      title="Last page"
                    >
                      <ChevronsRight className="h-4 w-4" />
                    </button>
                  </div>
                </div>
              );
            })()}
        </div>
      </div>
    </div>
  );
}
