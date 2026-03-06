// API client for code-search backend

const API_URL = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";

/**
 * Encode a file path for use in URLs.
 * This properly encodes each path segment to handle special characters like +, #, etc.
 * Unlike encodeURIComponent, this preserves forward slashes as path separators.
 */
export function encodeFilePath(path: string): string {
  if (!path) return "";
  return path
    .split("/")
    .map((segment) => encodeURIComponent(segment))
    .join("/");
}

/**
 * Build a browse URL for a file or directory.
 * Properly encodes the path to handle special characters.
 */
export function buildBrowseUrl(
  repoId: number,
  path?: string,
  options?: { ref?: string; line?: number }
): string {
  let url = `/repos/${repoId}/browse`;
  if (path) {
    url += `/${encodeFilePath(path)}`;
  }
  const params = new URLSearchParams();
  if (options?.ref) params.set("ref", options.ref);
  if (options?.line) params.set("line", String(options.line));
  const queryString = params.toString();
  if (queryString) url += `?${queryString}`;
  return url;
}

/**
 * Check if a file is an image based on its extension.
 */
export function isImageFile(path: string): boolean {
  const ext = path.split(".").pop()?.toLowerCase();
  return ["png", "jpg", "jpeg", "gif", "webp", "svg", "ico", "bmp"].includes(
    ext || ""
  );
}

/**
 * Check if a file is a PDF.
 */
export function isPdfFile(path: string): boolean {
  return path.toLowerCase().endsWith(".pdf");
}

/**
 * Check if a file is a video.
 */
export function isVideoFile(path: string): boolean {
  const ext = path.split(".").pop()?.toLowerCase();
  return ["mp4", "webm", "ogg", "mov"].includes(ext || "");
}

/**
 * Check if a file is audio.
 */
export function isAudioFile(path: string): boolean {
  const ext = path.split(".").pop()?.toLowerCase();
  return ["mp3", "wav", "ogg", "flac", "aac", "m4a"].includes(ext || "");
}

/**
 * Get the raw file URL for binary content.
 */
export function getRawFileUrl(
  repoId: number,
  path: string,
  ref?: string
): string {
  const params = new URLSearchParams();
  params.set("path", path);
  if (ref) params.set("ref", ref);
  return `${API_URL}/api/v1/repos/by-id/${repoId}/raw?${params.toString()}`;
}

// Types
export interface SearchRequest {
  query: string;
  is_regex?: boolean;
  case_sensitive?: boolean;
  repos?: string[];
  languages?: string[];
  file_patterns?: string[];
  limit?: number;
  context_lines?: number;
}

export interface SearchResult {
  repo: string;
  file: string;
  line: number;
  column: number;
  content: string;
  context: {
    before: string[];
    after: string[];
  };
  language: string;
  match_start: number;
  match_end: number;
}

export interface SearchResponse {
  results: SearchResult[];
  total_count: number;
  truncated?: boolean;
  duration: string;
  stats?: {
    files_searched: number;
    repos_searched: number;
  };
}

// Streaming search event types
export interface SearchStreamEvent {
  type: "progress" | "result" | "error" | "done";
  message?: string;
  result?: SearchResult;
  error?: string;
  total_count?: number;
  truncated?: boolean;
  duration?: string;
  stats?: {
    files_searched: number;
    repos_searched: number;
  };
}

export interface Repository {
  id: number;
  name: string;
  clone_url: string;
  branches: string[];
  default_branch?: string;
  status: string;
  last_indexed?: string;
  excluded?: boolean;
  deleted?: boolean;
}

export interface RepoListOptions {
  connectionId?: number;
  search?: string;
  status?: string;
  limit?: number;
  offset?: number;
}

export interface RepoListResponse {
  repos: Repository[];
  total_count: number;
  limit: number;
  offset: number;
  has_more: boolean;
}

export interface Connection {
  id: number;
  name: string;
  type: string;
  url: string;
  exclude_archived: boolean;
  created_at: string;
}

export interface JobProgress {
  current: number;
  total: number;
  message?: string;
}

// Job payload can contain various fields depending on job type
export interface JobPayload {
  // Index job fields
  repo_name?: string;
  repo_id?: number;
  connection_id?: number;
  // Replace job fields
  search_pattern?: string;
  old_pattern?: string;
  replace_with?: string;
  new_pattern?: string;
  // Allow additional fields
  [key: string]: unknown;
}

export interface Job {
  id: string;
  type: string;
  status: string;
  payload: JobPayload;
  result?: Record<string, unknown>;
  error?: string;
  started_at?: string;
  completed_at?: string;
  created_at: string;
  updated_at: string;
  progress?: JobProgress;
}

export interface JobListResponse {
  jobs: Job[];
  total_count: number;
  limit: number;
  offset: number;
  has_more: boolean;
}

export interface JobListOptions {
  type?: string;
  status?: string;
  exclude_status?: string;
  repo?: string;
  created_after?: string; // RFC3339 or Unix timestamp
  limit?: number;
  offset?: number;
}

export interface ReplacePreviewRequest {
  search_pattern: string;
  replace_with: string;
  is_regex?: boolean;
  case_sensitive?: boolean;
  file_patterns?: string[];
  repos?: string[];
  limit?: number; // Max results, 0 means default (1000)
}

export interface ReplaceMatch {
  repository_id: number;
  repository_name: string;
  file_path: string;
}

// PreviewMatch includes additional display fields from search results
export interface PreviewMatch {
  repository_id: number;
  repo: string;
  file: string;
  language: string;
  line: number;
  content: string;
  match_start: number;
  match_end: number;
  connection_id?: number;
  connection_name?: string;
  connection_has_token?: boolean;
}

export interface ReplaceExecuteRequest extends ReplacePreviewRequest {
  matches: ReplaceMatch[]; // Required - from preview response
  // MR options (MR is always created - never commits directly to main)
  branch_name?: string;
  mr_title?: string;
  mr_description?: string;
  // User-provided tokens for repos without server-side auth (map of connection_id -> token)
  user_tokens?: Record<string, string>;
}

export interface RepoSuggestion {
  name: string;
  display_name: string;
  full_name: string;
  status: string;
}

export interface LanguageSuggestion {
  name: string;
}

export interface FilterSuggestion {
  keyword: string;
  description: string;
  example: string;
}

export interface SearchSuggestionsResponse {
  repos: RepoSuggestion[];
  languages: LanguageSuggestion[];
  filters: FilterSuggestion[];
}

// File browsing types
export interface TreeEntry {
  name: string;
  type: "file" | "dir";
  path: string;
  size?: number;
  language?: string;
}

export interface TreeResponse {
  entries: TreeEntry[];
  path: string;
  ref: string;
}

export interface BlobResponse {
  content: string;
  path: string;
  size: number;
  language: string;
  language_mode: string;
  binary: boolean;
  ref: string;
}

export interface RefsResponse {
  branches: string[];
  tags: string[];
  default_branch: string;
}

export type ActivePane = "primary" | "secondary";

export interface PaneFile {
  repoId: number;
  path: string;
  ref?: string;
}

export interface PaneTab {
  id: string;
  file: PaneFile;
}

// File symbol types
export interface FileSymbol {
  name: string;
  kind: string;
  line: number;
  column: number;
  signature?: string;
  parent?: string;
}

export interface FileSymbolsResponse {
  symbols: FileSymbol[];
  path: string;
}

// Symbol types
export interface Symbol {
  name: string;
  kind: string;
  repo: string;
  file: string;
  line: number;
  column: number;
  signature?: string;
  language: string;
}

export interface SymbolReference {
  repo: string;
  file: string;
  line: number;
  column: number;
  context: string;
}

// SCIP types for precise code navigation
export interface SCIPOccurrence {
  symbol: string;
  filePath: string;
  startLine: number; // 0-indexed
  startCol: number; // 0-indexed
  endLine: number;
  endCol: number;
  role: number; // Bitmask: 1=Definition, 2=Import, 4=WriteAccess, 8=ReadAccess
  syntaxKind: number;
  context?: string; // The source line content
}

export interface SCIPSymbolInfo {
  symbol: string;
  documentation?: string;
  kind?: number;
  displayName?: string;
  enclosingSymbol?: string;
  relationships?: string[];
}

export interface SCIPStatusResponse {
  has_index: boolean;
  available_indexers: Record<string, boolean>;
  stats?: Record<string, unknown>;
}

export interface SCIPDefinitionResponse {
  found: boolean;
  symbol?: string;
  definition?: SCIPOccurrence;
  info?: SCIPSymbolInfo;
  external: boolean;
}

export interface SCIPReferencesResponse {
  found: boolean;
  symbol?: string;
  definition?: SCIPOccurrence;
  references: SCIPOccurrence[];
  totalCount: number;
}

export interface SCIPIndexResponse {
  success: boolean;
  language: string;
  duration: string;
  files: number;
  symbols: number;
  occurrences: number;
  error?: string;
  output?: string;
}

export interface SCIPIndexersResponse {
  available: Record<string, boolean>;
  supported: string[];
  instructions: Record<string, string>;
}


// API Client
class ApiClient {
  private baseUrl: string;

  constructor(baseUrl: string) {
    this.baseUrl = baseUrl;
  }

  private async request<T>(
    endpoint: string,
    options: RequestInit = {}
  ): Promise<T> {
    const url = `${this.baseUrl}${endpoint}`;
    const response = await fetch(url, {
      ...options,
      credentials: "include",
      headers: {
        "Content-Type": "application/json",
        ...options.headers,
      },
    });

    if (!response.ok) {
      const errorText = await response.text();
      let errorMessage = errorText || `API error: ${response.status}`;
      // Try to parse JSON error response and extract message
      try {
        const errorJson = JSON.parse(errorText);
        if (errorJson.message) {
          errorMessage = errorJson.message;
        }
      } catch {
        // Not JSON, use raw text
      }
      throw new Error(errorMessage);
    }

    if (response.status === 204) {
      return {} as T;
    }

    return response.json();
  }

  // Search
  async search(req: SearchRequest): Promise<SearchResponse> {
    return this.request("/api/v1/search", {
      method: "POST",
      body: JSON.stringify(req),
    });
  }

  // Streaming search - yields results as they arrive from the server
  async *searchStream(
    req: SearchRequest,
    signal?: AbortSignal
  ): AsyncGenerator<SearchStreamEvent, void, unknown> {
    const headers: Record<string, string> = {
      "Content-Type": "application/json",
      Accept: "text/event-stream",
    };

    const response = await fetch(`${this.baseUrl}/api/v1/search/stream`, {
      method: "POST",
      headers,
      credentials: "include",
      body: JSON.stringify(req),
      signal,
    });

    if (!response.ok) {
      throw new Error(
        `Search failed: ${response.status} ${response.statusText}`
      );
    }

    const reader = response.body?.getReader();
    if (!reader) {
      throw new Error("No response body");
    }

    const decoder = new TextDecoder();
    let buffer = "";

    try {
      while (true) {
        const { done, value } = await reader.read();
        if (done) break;

        buffer += decoder.decode(value, { stream: true });
        const lines = buffer.split("\n");
        buffer = lines.pop() || "";

        for (const line of lines) {
          if (line.startsWith("data: ")) {
            try {
              const data = JSON.parse(line.slice(6));
              yield data as SearchStreamEvent;
            } catch {
              // Ignore parse errors for incomplete data
            }
          }
        }
      }
    } finally {
      reader.releaseLock();
    }
  }

  // Repositories
  async getReposStatus(): Promise<{ readonly: boolean }> {
    return this.request("/api/v1/repos/status");
  }

  async listRepos(options?: RepoListOptions): Promise<RepoListResponse> {
    const params = new URLSearchParams();
    if (options?.connectionId)
      params.set("connection_id", options.connectionId.toString());
    if (options?.search) params.set("search", options.search);
    if (options?.status) params.set("status", options.status);
    if (options?.limit) params.set("limit", options.limit.toString());
    if (options?.offset) params.set("offset", options.offset.toString());
    const queryString = params.toString();
    return this.request(`/api/v1/repos${queryString ? `?${queryString}` : ""}`);
  }

  async getRepo(id: number): Promise<Repository> {
    return this.request(`/api/v1/repos/by-id/${id}`);
  }

  async lookupRepoByName(name: string): Promise<Repository> {
    return this.request(
      `/api/v1/repos/lookup?name=${encodeURIComponent(name)}`
    );
  }

  async addRepo(data: {
    connection_id: number;
    name: string;
    clone_url: string;
    default_branch?: string;
    branches?: string[];
  }): Promise<Repository> {
    return this.request("/api/v1/repos", {
      method: "POST",
      body: JSON.stringify(data),
    });
  }

  async deleteRepo(id: number): Promise<void> {
    return this.request(`/api/v1/repos/by-id/${id}`, {
      method: "DELETE",
    });
  }

  async excludeRepo(id: number): Promise<{ message: string; id: number }> {
    return this.request(`/api/v1/repos/by-id/${id}/exclude`, {
      method: "POST",
    });
  }

  async includeRepo(id: number): Promise<{ message: string; id: number }> {
    return this.request(`/api/v1/repos/by-id/${id}/include`, {
      method: "POST",
    });
  }

  async restoreRepo(id: number): Promise<{ message: string; id: number }> {
    return this.request(`/api/v1/repos/by-id/${id}/restore`, {
      method: "POST",
    });
  }

  async syncRepo(
    id: number
  ): Promise<{ status: string; job_id: string; message: string }> {
    return this.request(`/api/v1/repos/by-id/${id}/sync`, {
      method: "POST",
    });
  }

  async setRepoBranches(
    id: number,
    branches: string[]
  ): Promise<{ status: string; repo_id: number; branches: string[] }> {
    return this.request(`/api/v1/repos/by-id/${id}/branches`, {
      method: "PUT",
      body: JSON.stringify({ branches }),
    });
  }

  // Connections
  async getConnectionsStatus(): Promise<{
    readonly: boolean;
    message?: string;
  }> {
    return this.request("/api/v1/connections/status");
  }

  async listConnections(): Promise<Connection[]> {
    return this.request("/api/v1/connections");
  }

  async getConnection(id: number): Promise<Connection> {
    return this.request(`/api/v1/connections/${id}`);
  }

  async createConnection(data: {
    name: string;
    type: string;
    url: string;
    token: string;
    exclude_archived?: boolean;
  }): Promise<Connection> {
    return this.request("/api/v1/connections", {
      method: "POST",
      body: JSON.stringify(data),
    });
  }

  async updateConnection(
    id: number,
    data: {
      name: string;
      type: string;
      url: string;
      token?: string;
      exclude_archived?: boolean;
    }
  ): Promise<Connection> {
    return this.request(`/api/v1/connections/${id}`, {
      method: "PUT",
      body: JSON.stringify(data),
    });
  }

  async deleteConnection(id: number): Promise<void> {
    return this.request(`/api/v1/connections/${id}`, {
      method: "DELETE",
    });
  }

  async testConnection(
    id: number
  ): Promise<{ status: string; message?: string; error?: string }> {
    return this.request(`/api/v1/connections/${id}/test`, {
      method: "POST",
    });
  }

  async syncConnection(id: number): Promise<{
    status: string;
    message: string;
    job_id: string;
  }> {
    return this.request(`/api/v1/connections/${id}/sync`, {
      method: "POST",
    });
  }

  async listConnectionRepos(id: number): Promise<Repository[]> {
    return this.request(`/api/v1/connections/${id}/repos`);
  }

  // Replace
  async replacePreview(req: ReplacePreviewRequest): Promise<{
    matches: PreviewMatch[];
    total_count: number;
    truncated: boolean;
    limit: number;
    duration: string;
    stats: {
      files_searched: number;
      repos_searched: number;
    };
  }> {
    return this.request("/api/v1/replace/preview", {
      method: "POST",
      body: JSON.stringify(req),
    });
  }

  async replaceExecute(req: ReplaceExecuteRequest): Promise<{
    job_id: string;
    status: string;
    message: string;
  }> {
    return this.request("/api/v1/replace/execute", {
      method: "POST",
      body: JSON.stringify(req),
    });
  }

  async listReplaceJobs(): Promise<Job[]> {
    return this.request("/api/v1/replace/jobs");
  }

  async getReplaceJob(id: string): Promise<Job> {
    return this.request(`/api/v1/replace/jobs/${id}`);
  }

  // Jobs
  async listJobs(options?: JobListOptions): Promise<JobListResponse> {
    const params = new URLSearchParams();
    if (options?.type) params.set("type", options.type);
    if (options?.status) params.set("status", options.status);
    if (options?.exclude_status)
      params.set("exclude_status", options.exclude_status);
    if (options?.repo) params.set("repo", options.repo);
    if (options?.created_after)
      params.set("created_after", options.created_after);
    if (options?.limit) params.set("limit", options.limit.toString());
    if (options?.offset) params.set("offset", options.offset.toString());
    const queryString = params.toString();
    return this.request(`/api/v1/jobs${queryString ? `?${queryString}` : ""}`);
  }

  async getJob(id: string): Promise<Job> {
    return this.request(`/api/v1/jobs/${id}`);
  }

  async cancelJob(id: string): Promise<{ status: string; message: string }> {
    return this.request(`/api/v1/jobs/${id}/cancel`, {
      method: "POST",
    });
  }

  // Search Suggestions
  async getSearchSuggestions(): Promise<SearchSuggestionsResponse> {
    return this.request("/api/v1/search/suggestions");
  }

  // Health
  async health(): Promise<{ status: string }> {
    return this.request("/health");
  }

  async ready(): Promise<{ status: string; checks: Record<string, string> }> {
    return this.request("/ready");
  }

  // File browsing
  async getTree(
    repoId: number,
    path?: string,
    ref?: string
  ): Promise<TreeResponse> {
    const params = new URLSearchParams();
    if (path) params.set("path", path);
    if (ref) params.set("ref", ref);
    const queryString = params.toString();
    return this.request(
      `/api/v1/repos/by-id/${repoId}/tree${
        queryString ? `?${queryString}` : ""
      }`
    );
  }

  async getBlob(
    repoId: number,
    path: string,
    ref?: string
  ): Promise<BlobResponse> {
    const params = new URLSearchParams();
    params.set("path", path);
    if (ref) params.set("ref", ref);
    return this.request(
      `/api/v1/repos/by-id/${repoId}/blob?${params.toString()}`
    );
  }

  async getRefs(repoId: number): Promise<RefsResponse> {
    return this.request(`/api/v1/repos/by-id/${repoId}/refs`);
  }

  async getFileSymbols(
    repoId: number,
    path: string
  ): Promise<FileSymbolsResponse> {
    const params = new URLSearchParams({ path });
    return this.request(
      `/api/v1/repos/by-id/${repoId}/symbols?${params.toString()}`
    );
  }

  // Symbols
  async findSymbols(options: {
    name: string;
    kind?: string;
    repos?: string[];
    language?: string;
    limit?: number;
  }): Promise<Symbol[]> {
    return this.request("/api/v1/symbols/find", {
      method: "POST",
      body: JSON.stringify(options),
    });
  }

  async findReferences(options: {
    symbol: string;
    repos?: string[];
    language?: string;
    limit?: number;
  }): Promise<SymbolReference[]> {
    return this.request("/api/v1/symbols/refs", {
      method: "POST",
      body: JSON.stringify(options),
    });
  }

  // SCIP - Precise Code Navigation
  async getSCIPStatus(repoId: number): Promise<SCIPStatusResponse> {
    return this.request(`/api/v1/scip/repos/${repoId}/status`);
  }

  async getSCIPDefinition(
    repoId: number,
    filePath: string,
    line: number,
    column: number
  ): Promise<SCIPDefinitionResponse> {
    return this.request(`/api/v1/scip/repos/${repoId}/definition`, {
      method: "POST",
      body: JSON.stringify({ filePath, line, column }),
    });
  }

  async getSCIPReferences(
    repoId: number,
    filePath: string,
    line: number,
    column: number,
    limit?: number
  ): Promise<SCIPReferencesResponse> {
    return this.request(`/api/v1/scip/repos/${repoId}/references`, {
      method: "POST",
      body: JSON.stringify({ filePath, line, column, limit }),
    });
  }

  async indexSCIP(
    repoId: number,
    language?: string
  ): Promise<SCIPIndexResponse> {
    return this.request(`/api/v1/scip/repos/${repoId}/index`, {
      method: "POST",
      body: JSON.stringify({ language }),
    });
  }

  async uploadSCIPIndex(
    repoId: number,
    data: ArrayBuffer
  ): Promise<{
    success: boolean;
    message: string;
    stats: SCIPStatusResponse["stats"];
  }> {
    const url = `${this.baseUrl}/api/v1/scip/repos/${repoId}/upload`;
    const response = await fetch(url, {
      method: "POST",
      headers: {
        "Content-Type": "application/octet-stream",
      },
      body: data,
    });

    if (!response.ok) {
      const errorText = await response.text();
      throw new Error(errorText || `API error: ${response.status}`);
    }

    return response.json();
  }

  async clearSCIPIndex(
    repoId: number
  ): Promise<{ success: boolean; message: string }> {
    return this.request(`/api/v1/scip/repos/${repoId}/index`, {
      method: "DELETE",
    });
  }

  async getSCIPIndexers(): Promise<SCIPIndexersResponse> {
    return this.request("/api/v1/scip/indexers");
  }

  // UI Settings
  async getUISettings(): Promise<{
    hide_readonly_banner: boolean;
    hide_file_navigator: boolean;
    disable_browse_api: boolean;
    connections_readonly: boolean;
    repos_readonly: boolean;
    enable_streaming: boolean;
    hide_repos_page: boolean;
    hide_connections_page: boolean;
    hide_jobs_page: boolean;
    hide_replace_page: boolean;
  }> {
    return this.request("/api/v1/ui/settings");
  }
}

export const api = new ApiClient(API_URL);
