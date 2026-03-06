"use client";

import { useState, useEffect, useRef, useCallback } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { api, SearchSuggestionsResponse, ReplaceMatch, PreviewMatch } from "@/lib/api";
import { useActiveContext } from "@/hooks/useContexts";
import { ContextSwitcher } from "@/components/ContextSwitcher";
import { RefreshCw, FolderGit2, FileCode, ExternalLink, ChevronDown, ChevronRight, ChevronUp, Eye, Play, Loader2, Filter, GitBranch, Code, History, X, Clock, Search, Key, AlertCircle, AlertTriangle } from "lucide-react";

interface FileGroup {
  repo: string;
  file: string;
  language: string;
  matches: PreviewMatch[];
}

interface Suggestion {
  type: "filter" | "repo" | "language";
  value: string;
  description?: string;
  icon?: React.ReactNode;
}

interface SearchHistoryItem {
  query: string;
  timestamp: number;
}

const SEARCH_HISTORY_KEY = "code-search-history";
const MAX_HISTORY_ITEMS = 50;

function getSearchHistory(): SearchHistoryItem[] {
  if (typeof window === "undefined") return [];
  try {
    const stored = localStorage.getItem(SEARCH_HISTORY_KEY);
    return stored ? JSON.parse(stored) : [];
  } catch {
    return [];
  }
}

function saveSearchHistory(history: SearchHistoryItem[]) {
  if (typeof window === "undefined") return;
  try {
    localStorage.setItem(SEARCH_HISTORY_KEY, JSON.stringify(history.slice(0, MAX_HISTORY_ITEMS)));
  } catch {
    // Ignore storage errors
  }
}

function addToSearchHistory(query: string) {
  if (!query.trim()) return;
  const history = getSearchHistory();
  // Remove duplicates
  const filtered = history.filter(h => h.query !== query);
  // Add new item at the beginning
  const newHistory = [{ query, timestamp: Date.now() }, ...filtered];
  saveSearchHistory(newHistory);
}

// Component to render syntax-highlighted query
function HighlightedQuery({ query }: { query: string }) {
  // Parse the query and highlight filter prefixes and regex patterns
  const parts: React.ReactNode[] = [];

  const CYAN_500 = "#06b6d4";
  const ORANGE_500 = "#f97316";
  const RED_500 = "#ef4444";
  const PURPLE_500 = "#a855f7";

  const tokenRegex = new RegExp('(/[^/\\\\s]+/)|((repo:|file:|lang:|content:|case:|sym:|symbol:|branch:|rev:|path:|type:|-repo:|-file:|-lang:|-content:|-path:)([^\\\\s]*))', 'g');

  let lastIndex = 0;
  let match;

  while ((match = tokenRegex.exec(query)) !== null) {
    if (match.index > lastIndex) {
      parts.push(
        <span key={`text-${lastIndex}`} className="text-gray-900 dark:text-white">
          {query.slice(lastIndex, match.index)}
        </span>
      );
    }

    if (match[1]) {
      parts.push(
        <span key={`regex-${match.index}`} style={{ color: PURPLE_500 }}>
          {match[1]}
        </span>
      );
    } else if (match[2]) {
      const prefix = match[3];
      const value = match[4];
      const isNegation = prefix.startsWith("-");

      let color = CYAN_500;
      if (isNegation) {
        color = RED_500;
      } else if (prefix === "case:") {
        color = ORANGE_500;
      }

      parts.push(
        <span key={`filter-${match.index}`}>
          <span style={{ color }}>{prefix}</span>
          <span className="text-gray-900 dark:text-white">{value}</span>
        </span>
      );
    }

    lastIndex = match.index + match[0].length;
  }

  if (lastIndex < query.length) {
    parts.push(
      <span key={`text-${lastIndex}`} className="text-gray-900 dark:text-white">
        {query.slice(lastIndex)}
      </span>
    );
  }

  if (parts.length === 0 && query) {
    return <span className="text-gray-900 dark:text-white">{query}</span>;
  }

  return <>{parts}</>;
}

function PreviewResults({
  preview,
  replaceWith,
  truncated,
  totalCount,
  duration,
}: {
  preview: PreviewMatch[];
  searchPattern: string;
  replaceWith: string;
  truncated?: boolean;
  totalCount?: number;
  duration?: string;
  stats?: { files_searched: number; repos_searched: number } | null;
}) {
  const groupedResults = preview.reduce((acc, match) => {
    const key = `${match.repo}:${match.file}`;
    if (!acc[key]) {
      acc[key] = {
        repo: match.repo,
        file: match.file,
        language: match.language,
        matches: [],
      };
    }
    acc[key].matches.push(match);
    return acc;
  }, {} as Record<string, FileGroup>);

  const groupedList = Object.values(groupedResults);
  const uniqueReposWithMatches = new Set(groupedList.map(g => g.repo)).size;

  const highlightReplacement = (match: PreviewMatch) => {
    const { content, match_start, match_end } = match;

    if (match_start === undefined || match_end === undefined || match_start === match_end) {
      return content;
    }

    const parts: React.ReactNode[] = [];

    if (match_start > 0) {
      parts.push(content.slice(0, match_start));
    }

    const matchedText = content.slice(match_start, match_end);
    parts.push(
      <span key={match_start}>
        <del className="bg-red-200 dark:bg-red-900/50 text-red-800 dark:text-red-200 line-through">
          {matchedText}
        </del>
        <ins className="bg-green-200 dark:bg-green-900/50 text-green-800 dark:text-green-200 no-underline">
          {replaceWith}
        </ins>
      </span>
    );

    if (match_end < content.length) {
      parts.push(content.slice(match_end));
    }

    return parts.length > 0 ? parts : content;
  };

  return (
    <div>
      <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-2 mb-5 px-1">
        <div className="flex items-center gap-2 text-sm text-gray-600 dark:text-gray-400">
          <span className="font-semibold text-gray-900 dark:text-white">{totalCount?.toLocaleString() || preview.length}</span>
          <span>results in</span>
          {duration && <span className="font-mono text-blue-600 dark:text-blue-400">{duration}</span>}
        </div>
        <div className="flex items-center gap-4 text-sm text-gray-500 dark:text-gray-400">
          <span className="flex items-center gap-1.5">
            <FileCode className="w-4 h-4" />
            {groupedList.length} files
          </span>
          <span className="flex items-center gap-1.5">
            <FolderGit2 className="w-4 h-4" />
            {uniqueReposWithMatches} repos
          </span>
        </div>
      </div>

      {truncated && (
        <div className="mb-4 p-3 bg-amber-50 dark:bg-amber-900/20 border border-amber-200 dark:border-amber-800 rounded-lg flex items-center gap-2 text-sm text-amber-700 dark:text-amber-400">
          <AlertTriangle className="w-4 h-4 flex-shrink-0" />
          <span>
            Results truncated. Showing {preview.length.toLocaleString()} of {(totalCount || 0).toLocaleString()} matches.
            Use filters or set a limit to narrow your search.
          </span>
        </div>
      )}

      {preview.length === 0 ? (
        <div className="text-center py-12 bg-gray-50 dark:bg-gray-800/50 rounded-xl">
          <p className="text-gray-500 dark:text-gray-400">No matches found</p>
        </div>
      ) : (
        <div className="space-y-3">
          {groupedList.map((group) => (
            <FileReplaceCard
              key={`${group.repo}:${group.file}`}
              group={group}
              highlightReplacement={highlightReplacement}
            />
          ))}
        </div>
      )}
    </div>
  );
}

function FileReplaceCard({
  group,
  highlightReplacement,
}: {
  group: FileGroup;
  highlightReplacement: (match: PreviewMatch) => React.ReactNode;
}) {
  const [collapsed, setCollapsed] = useState(false);
  const sortedMatches = [...group.matches].sort((a, b) => a.line - b.line);

  const getFileUrl = (line: number) => {
    const repo = group.repo;
    const file = group.file;

    if (repo.includes("github.com")) {
      return `https://${repo}/blob/HEAD/${file}#L${line}`;
    } else if (repo.includes("gitlab")) {
      return `https://${repo}/-/blob/HEAD/${file}#L${line}`;
    } else if (repo.includes("bitbucket")) {
      return `https://${repo}/src/HEAD/${file}#lines-${line}`;
    }
    return `https://${repo}/-/blob/HEAD/${file}#L${line}`;
  };

  return (
    <div className="bg-white dark:bg-gray-800 rounded-xl shadow-sm border border-gray-200 dark:border-gray-700 overflow-hidden hover:shadow-md transition-shadow">
      <div className="flex flex-wrap items-center gap-2 sm:gap-3 px-3 sm:px-4 py-3 bg-gray-50 dark:bg-gray-800/80">
        <button
          type="button"
          onClick={() => setCollapsed(!collapsed)}
          className="text-gray-400 hover:text-gray-600 dark:hover:text-gray-300"
          title={collapsed ? "Expand" : "Collapse"}
        >
          {collapsed ? <ChevronRight className="w-4 h-4" /> : <ChevronDown className="w-4 h-4" />}
        </button>
        <FolderGit2 className="w-4 h-4 text-blue-500 hidden sm:block" />
        <a
          href={getFileUrl(sortedMatches[0]?.line || 1)}
          target="_blank"
          rel="noopener noreferrer"
          className="text-xs sm:text-sm font-medium text-blue-600 dark:text-blue-400 hover:underline truncate max-w-[120px] sm:max-w-none"
          onClick={(e) => e.stopPropagation()}
        >
          {group.repo}
        </a>
        <span className="text-gray-300 dark:text-gray-600 hidden sm:inline">/</span>
        <FileCode className="w-4 h-4 text-gray-400 hidden sm:block" />
        <a
          href={getFileUrl(sortedMatches[0]?.line || 1)}
          target="_blank"
          rel="noopener noreferrer"
          className="text-xs sm:text-sm text-gray-700 dark:text-gray-300 hover:text-blue-600 dark:hover:text-blue-400 hover:underline flex-1 truncate"
          onClick={(e) => e.stopPropagation()}
        >
          {group.file}
        </a>
        <span className="text-xs px-2 py-0.5 bg-amber-100 dark:bg-amber-900/30 text-amber-700 dark:text-amber-300 rounded-full font-medium whitespace-nowrap">
          {sortedMatches.length} {sortedMatches.length === 1 ? 'replacement' : 'replacements'}
        </span>
        {group.language && (
          <span className="text-xs px-2 py-0.5 bg-gray-200 dark:bg-gray-700 rounded-full font-medium hidden sm:inline">
            {group.language}
          </span>
        )}
        <a
          href={getFileUrl(sortedMatches[0]?.line || 1)}
          target="_blank"
          rel="noopener noreferrer"
          className="text-gray-400 hover:text-blue-600 dark:hover:text-blue-400 hidden sm:block"
          onClick={(e) => e.stopPropagation()}
          title="View on code host"
        >
          <ExternalLink className="w-4 h-4" />
        </a>
      </div>

      {!collapsed && (
        <div className="border-t border-gray-100 dark:border-gray-700/50 overflow-x-auto">
          <table className="w-full text-sm font-mono">
            <tbody>
              {sortedMatches.map((match) => (
                <tr key={`${match.line}-${match.match_start}`} className="hover:bg-gray-50 dark:hover:bg-gray-700/30 border-b border-gray-100 dark:border-gray-700/50 last:border-0">
                  <td className="py-1 px-3 text-right text-gray-400 dark:text-gray-500 select-none w-12 bg-gray-50 dark:bg-gray-800/50">
                    <a
                      href={getFileUrl(match.line)}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="hover:text-blue-600 dark:hover:text-blue-400"
                    >
                      {match.line}
                    </a>
                  </td>
                  <td className="py-1 px-4 whitespace-pre text-gray-800 dark:text-gray-200">
                    {highlightReplacement(match)}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}

export default function ReplaceClient() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const { activeContext, getRepoFilter } = useActiveContext();

  const [state, setState] = useState({
    searchPattern: "",
    replaceWith: "",
    filePatterns: "",
    repos: "",
    mrTitle: "",
    showOptions: false,
    preview: null as PreviewMatch[] | null,
    previewTruncated: false,
    previewTotalCount: 0,
    previewDuration: "",
    previewStats: null as { files_searched: number; repos_searched: number } | null,
    loading: false,
    executing: false,
    result: null as { job_id: string; message: string } | null,
    error: null as string | null,
    reposReadOnly: false,
    hideReadOnlyBanner: false,
    userTokens: {} as Record<string, string>,
    showTokenModal: false,
    previewStale: false,
  });

  const lastPreviewParams = useRef<string>("");
  const isManualSearch = useRef(false);

  const filePatternsRef = useRef(state.filePatterns);
  const reposRef = useRef(state.repos);
  const filePatternsInputRef = useRef<HTMLInputElement>(null);
  const reposInputRef = useRef<HTMLInputElement>(null);

  const [showSuggestions, setShowSuggestions] = useState(false);
  const [suggestions, setSuggestions] = useState<SearchSuggestionsResponse | null>(null);
  const [filteredSuggestions, setFilteredSuggestions] = useState<Suggestion[]>([]);
  const [selectedIndex, setSelectedIndex] = useState(0);
  const inputRef = useRef<HTMLInputElement>(null);
  const highlightRef = useRef<HTMLDivElement>(null);
  const suggestionsRef = useRef<HTMLDivElement>(null);

  const [showHistory, setShowHistory] = useState(false);
  const [searchHistory, setSearchHistory] = useState<SearchHistoryItem[]>([]);
  const [historyFilter, setHistoryFilter] = useState("");
  const [historySelectedIndex, setHistorySelectedIndex] = useState(0);
  const historyRef = useRef<HTMLDivElement>(null);
  const historyButtonRef = useRef<HTMLButtonElement>(null);

  useEffect(() => {
    setSearchHistory(getSearchHistory());
    api.getUISettings().then(settings => {
      setState(prev => ({
        ...prev,
        reposReadOnly: settings.repos_readonly,
        hideReadOnlyBanner: settings.hide_readonly_banner,
      }));
    }).catch(() => {
      api.getReposStatus().then(status => 
        setState(prev => ({ ...prev, reposReadOnly: status.readonly }))
      ).catch(() => { });
    });

    inputRef.current?.focus();
  }, []);

  useEffect(() => {
    if (isManualSearch.current) {
      isManualSearch.current = false;
      return;
    }

    const q = searchParams.get("q") || "";
    const replace = searchParams.get("replace") || "";
    const files = searchParams.get("files") || "";
    const reposParam = searchParams.get("repos") || "";

    if (q) {
      const executePreview = async () => {
        setState(prev => ({ 
          ...prev, 
          searchPattern: q,
          replaceWith: replace,
          filePatterns: files,
          repos: reposParam,
          showOptions: !!(files || reposParam),
          loading: true, 
          error: null 
        }));

        filePatternsRef.current = files;
        reposRef.current = reposParam;

        let effectiveRepos = reposParam;
        let repoPatternForQuery = "";
        if (activeContext && activeContext.repos.length > 0) {
          if (reposParam) {
            const userPatterns = reposParam.split(",").map(r => r.trim().toLowerCase());
            const filtered = activeContext.repos.filter(repo =>
              userPatterns.some(pattern => repo.name.toLowerCase().includes(pattern))
            );
            if (filtered.length > 0) {
              effectiveRepos = filtered.map(r => r.name).join(",");
              repoPatternForQuery = `(${filtered.map(r => r.name.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")).join("|")})`;
            } else {
              effectiveRepos = activeContext.repos.map(r => r.name).join(",");
              repoPatternForQuery = `(${activeContext.repos.map(r => r.name.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")).join("|")})`;
            }
          } else {
            effectiveRepos = activeContext.repos.map(r => r.name).join(",");
            repoPatternForQuery = `(${activeContext.repos.map(r => r.name.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")).join("|")})`;
          }
        } else if (reposParam) {
          const repoList = reposParam.split(",").map(r => r.trim()).filter(Boolean);
          if (repoList.length > 1) {
            repoPatternForQuery = `(${repoList.map(r => r.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")).join("|")})`;
          } else if (repoList.length === 1) {
            repoPatternForQuery = repoList[0];
          }
        }

        try {
          let searchPatternWithFilter = q;
          if (repoPatternForQuery) {
            searchPatternWithFilter = `repo:${repoPatternForQuery} ${q}`;
          }

          const response = await api.replacePreview({
            search_pattern: searchPatternWithFilter,
            replace_with: replace,
            file_patterns: files ? files.split(",").map((p) => p.trim()) : undefined,
            repos: undefined,
            limit: 0,
          });

          let filtered = response.matches;
          if (files) {
            const patterns = files.split(",").map(p => p.trim()).filter(Boolean);
            if (patterns.length > 0) {
              filtered = filtered.filter(match => {
                return patterns.some(pattern => {
                  const regexPattern = pattern
                    .replace(/\./g, "\\.")
                    .replace(/\*/g, ".*")
                    .replace(/\?/g, ".");
                  const regex = new RegExp(regexPattern, 'i');
                  return regex.test(match.file);
                });
              });
            }
          }
          if (effectiveRepos) {
            const repoPatterns = effectiveRepos.split(",").map(r => r.trim()).filter(Boolean);
            if (repoPatterns.length > 0) {
              filtered = filtered.filter(match => {
                return repoPatterns.some(pattern => {
                  return match.repo.toLowerCase().includes(pattern.toLowerCase());
                });
              });
            }
          }

          setState(prev => ({
            ...prev,
            preview: filtered,
            previewTruncated: response.truncated,
            previewTotalCount: response.total_count,
            previewDuration: response.duration,
            previewStats: response.stats,
            previewStale: false,
            loading: false,
          }));

          lastPreviewParams.current = JSON.stringify({
            searchPattern: q,
            filePatterns: files,
            repos: reposParam
          });
        } catch (err) {
          setState(prev => ({
            ...prev,
            error: err instanceof Error ? err.message : "Preview failed",
            preview: null,
            previewTruncated: false,
            previewTotalCount: 0,
            previewDuration: "",
            previewStats: null,
            loading: false,
          }));
        }
      };

      executePreview();
    }
  }, [searchParams, activeContext, getRepoFilter]);

  const filteredHistory = searchHistory.filter(h =>
    h.query.toLowerCase().includes(historyFilter.toLowerCase())
  );

  useEffect(() => {
    api.getSearchSuggestions()
      .then(setSuggestions)
      .catch(console.error);
  }, []);

  const updateSuggestions = useCallback((value: string) => {
    if (!suggestions) return;

    const parts = value.split(/\s+/);
    const lastPart = parts[parts.length - 1] || "";
    const newSuggestions: Suggestion[] = [];

    if (lastPart.startsWith("repo:")) {
      const search = lastPart.slice(5).toLowerCase();
      suggestions.repos
        .filter(r => r.display_name.toLowerCase().includes(search))
        .slice(0, 8)
        .forEach(r => newSuggestions.push({
          type: "repo",
          value: `repo:${r.name}`,
          description: r.display_name,
          icon: <GitBranch className="w-4 h-4 text-blue-500" />,
        }));
    } else if (lastPart.startsWith("lang:")) {
      const search = lastPart.slice(5).toLowerCase();
      suggestions.languages
        .filter(l => l.name.toLowerCase().includes(search))
        .slice(0, 8)
        .forEach(l => newSuggestions.push({
          type: "language",
          value: `lang:${l.name}`,
          icon: <Code className="w-4 h-4 text-green-500" />,
        }));
    } else if (lastPart === "" || !lastPart.includes(":")) {
      const search = lastPart.toLowerCase();
      suggestions.filters
        .filter(f => f.keyword.toLowerCase().includes(search) || f.description.toLowerCase().includes(search))
        .forEach(f => newSuggestions.push({
          type: "filter",
          value: f.keyword,
          description: f.description,
          icon: <Filter className="w-4 h-4 text-purple-500" />,
        }));
    }

    setFilteredSuggestions(newSuggestions);
    setSelectedIndex(0);
  }, [suggestions]);

  useEffect(() => {
    if (showSuggestions && filteredSuggestions.length > 0) {
      const selectedElement = document.querySelector(`[data-replace-suggestion-index="${selectedIndex}"]`);
      selectedElement?.scrollIntoView({ block: "nearest", behavior: "smooth" });
    }
  }, [selectedIndex, showSuggestions, filteredSuggestions.length]);

  const handleInputChange = (value: string) => {
    setState(prev => ({ 
      ...prev, 
      searchPattern: value,
      previewStale: !!prev.preview // Mark stale if we have a preview
    }));
    updateSuggestions(value);
    setShowSuggestions(true);

    requestAnimationFrame(() => {
      if (inputRef.current && highlightRef.current) {
        highlightRef.current.style.transform = `translateX(-${inputRef.current.scrollLeft}px)`;
      }
    });
  };

  const selectSuggestion = (suggestion: Suggestion) => {
    const parts = state.searchPattern.split(/\s+/);
    parts[parts.length - 1] = suggestion.value;

    const newQuery = parts.join(" ") + (suggestion.type === "filter" && !suggestion.value.endsWith(":") ? " " : "");
    setState(prev => ({ 
      ...prev, 
      searchPattern: newQuery,
      previewStale: !!prev.preview
    }));
    setShowSuggestions(false);
    inputRef.current?.focus();

    requestAnimationFrame(() => {
      if (inputRef.current) {
        inputRef.current.setSelectionRange(newQuery.length, newQuery.length);
        inputRef.current.scrollLeft = inputRef.current.scrollWidth;
        if (highlightRef.current) {
          highlightRef.current.style.transform = `translateX(-${inputRef.current.scrollLeft}px)`;
        }
      }
    });
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (!showSuggestions || filteredSuggestions.length === 0) {
      if (e.key === "Enter") return;
    }

    switch (e.key) {
      case "ArrowDown":
        e.preventDefault();
        setSelectedIndex(i => Math.min(i + 1, filteredSuggestions.length - 1));
        break;
      case "ArrowUp":
        e.preventDefault();
        setSelectedIndex(i => Math.max(i - 1, 0));
        break;
      case "Tab":
        if (filteredSuggestions[selectedIndex]) {
          e.preventDefault();
          selectSuggestion(filteredSuggestions[selectedIndex]);
        }
        break;
      case "Enter":
        if (showSuggestions && filteredSuggestions[selectedIndex]) {
          e.preventDefault();
          selectSuggestion(filteredSuggestions[selectedIndex]);
        }
        break;
      case "Escape":
        e.preventDefault();
        if (showSuggestions) {
          setShowSuggestions(false);
        } else {
          inputRef.current?.blur();
        }
        break;
    }
  };

  useEffect(() => {
    if (!showSuggestions && !showHistory) return;
    const handleClickOutside = (e: MouseEvent) => {
      if (
        showSuggestions &&
        suggestionsRef.current &&
        !suggestionsRef.current.contains(e.target as Node) &&
        inputRef.current &&
        !inputRef.current.contains(e.target as Node)
      ) {
        setShowSuggestions(false);
      }
      if (
        showHistory &&
        historyRef.current &&
        !historyRef.current.contains(e.target as Node) &&
        historyButtonRef.current &&
        !historyButtonRef.current.contains(e.target as Node)
      ) {
        setShowHistory(false);
        setHistoryFilter("");
      }
    };
    document.addEventListener("mousedown", handleClickOutside);
    return () => document.removeEventListener("mousedown", handleClickOutside);
  }, [showSuggestions, showHistory]);

  const selectHistoryItem = (item: SearchHistoryItem) => {
    setState(prev => ({ ...prev, searchPattern: item.query }));
    setShowHistory(false);
    setHistoryFilter("");
    inputRef.current?.focus();
  };

  const deleteHistoryItem = (e: React.SyntheticEvent, queryToDelete: string) => {
    e.stopPropagation();
    const newHistory = searchHistory.filter(h => h.query !== queryToDelete);
    setSearchHistory(newHistory);
    saveSearchHistory(newHistory);
  };

  const clearAllHistory = () => {
    setSearchHistory([]);
    saveSearchHistory([]);
    setShowHistory(false);
  };

  const handleHistoryKeyDown = (e: React.KeyboardEvent) => {
    switch (e.key) {
      case "ArrowDown":
        e.preventDefault();
        setHistorySelectedIndex(i => Math.min(i + 1, filteredHistory.length - 1));
        break;
      case "ArrowUp":
        e.preventDefault();
        setHistorySelectedIndex(i => Math.max(i - 1, 0));
        break;
      case "Enter":
        e.preventDefault();
        if (filteredHistory[historySelectedIndex]) {
          selectHistoryItem(filteredHistory[historySelectedIndex]);
        }
        break;
      case "Escape":
        e.preventDefault();
        setShowHistory(false);
        setHistoryFilter("");
        break;
    }
  };

  const filterMatchesWithContext = (matches: PreviewMatch[], filePatterns: string, repoFilter: string): PreviewMatch[] => {
    let filtered = matches;

    if (filePatterns) {
      const patterns = filePatterns.split(",").map(p => p.trim()).filter(Boolean);
      if (patterns.length > 0) {
        filtered = filtered.filter(match => {
          return patterns.some(pattern => {
            const regexPattern = pattern
              .replace(/\./g, "\\.")
              .replace(/\*/g, ".*")
              .replace(/\?/g, ".");
            const regex = new RegExp(regexPattern, 'i');
            return regex.test(match.file);
          });
        });
      }
    }

    if (repoFilter) {
      const repoPatterns = repoFilter.split(",").map(r => r.trim()).filter(Boolean);
      if (repoPatterns.length > 0) {
        filtered = filtered.filter(match => {
          return repoPatterns.some(pattern => {
            return match.repo.toLowerCase().includes(pattern.toLowerCase());
          });
        });
      }
    }

    return filtered;
  };

  const handlePreview = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!state.searchPattern.trim()) return;

    setShowSuggestions(false);
    setShowHistory(false);
    setState(prev => ({ ...prev, loading: true, error: null, result: null }));

    addToSearchHistory(state.searchPattern);
    setSearchHistory(getSearchHistory());

    const currentFilePatterns = filePatternsInputRef.current?.value ?? filePatternsRef.current;
    const currentRepos = reposInputRef.current?.value ?? reposRef.current;

    let effectiveRepos = currentRepos;
    let repoPatternForQuery = ""; 
    if (activeContext && activeContext.repos.length > 0) {
      if (currentRepos) {
        const userPatterns = currentRepos.split(",").map(r => r.trim().toLowerCase());
        const filtered = activeContext.repos.filter(repo =>
          userPatterns.some(pattern => repo.name.toLowerCase().includes(pattern))
        );
        if (filtered.length > 0) {
          effectiveRepos = filtered.map(r => r.name).join(",");
          repoPatternForQuery = `(${filtered.map(r => r.name.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")).join("|")})`;
        } else {
          effectiveRepos = activeContext.repos.map(r => r.name).join(",");
          repoPatternForQuery = `(${activeContext.repos.map(r => r.name.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")).join("|")})`;
        }
      } else {
        effectiveRepos = activeContext.repos.map(r => r.name).join(",");
        repoPatternForQuery = `(${activeContext.repos.map(r => r.name.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")).join("|")})`;
      }
    } else if (currentRepos) {
      const repoList = currentRepos.split(",").map(r => r.trim()).filter(Boolean);
      if (repoList.length > 1) {
        repoPatternForQuery = `(${repoList.map(r => r.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")).join("|")})`;
      } else if (repoList.length === 1) {
        repoPatternForQuery = repoList[0];
      }
    }

    isManualSearch.current = true;

    const params = new URLSearchParams();
    params.set("q", state.searchPattern);
    if (state.replaceWith) params.set("replace", state.replaceWith);
    if (currentFilePatterns) params.set("files", currentFilePatterns);
    if (currentRepos) params.set("repos", currentRepos);
    router.push(`/replace?${params.toString()}`, { scroll: false });

    try {
      let searchPatternWithFilter = state.searchPattern;
      if (repoPatternForQuery) {
        searchPatternWithFilter = `repo:${repoPatternForQuery} ${state.searchPattern}`;
      }

      const response = await api.replacePreview({
        search_pattern: searchPatternWithFilter,
        replace_with: state.replaceWith,
        file_patterns: currentFilePatterns ? currentFilePatterns.split(",").map((p) => p.trim()) : undefined,
        repos: undefined,
        limit: 0,
      });

      const filteredMatches = filterMatchesWithContext(response.matches, currentFilePatterns, effectiveRepos);
      setState(prev => ({
        ...prev,
        preview: filteredMatches,
        previewTruncated: response.truncated,
        previewTotalCount: response.total_count,
        previewDuration: response.duration,
        previewStats: response.stats,
        previewStale: false,
      }));

      lastPreviewParams.current = JSON.stringify({
        searchPattern: state.searchPattern,
        filePatterns: currentFilePatterns,
        repos: currentRepos
      });
    } catch (err) {
      setState(prev => ({
        ...prev,
        error: err instanceof Error ? err.message : "Preview failed",
        preview: null,
        previewTruncated: false,
        previewTotalCount: 0,
        previewDuration: "",
        previewStats: null,
      }));
    } finally {
      setState(prev => ({ ...prev, loading: false }));
    }
  };

  const handleExecute = async () => {
    if (!state.preview || state.preview.length === 0) {
      setState(prev => ({ ...prev, error: "No matches to execute. Run preview first." }));
      return;
    }

    const connectionsNeedingTokens = new Map<string, { name: string; repos: Set<string> }>();
    for (const m of state.preview) {
      if (m.connection_has_token === false && m.connection_id !== undefined) {
        const connId = String(m.connection_id);
        if (!connectionsNeedingTokens.has(connId)) {
          connectionsNeedingTokens.set(connId, {
            name: m.connection_name || `Connection ${m.connection_id}`,
            repos: new Set(),
          });
        }
        connectionsNeedingTokens.get(connId)!.repos.add(m.repo);
      }
    }

    const missingTokens = [...connectionsNeedingTokens.keys()].filter(
      connId => !state.userTokens[connId]?.trim()
    );

    if (missingTokens.length > 0) {
      setState(prev => ({ ...prev, showTokenModal: true }));
      return;
    }

    if (state.reposReadOnly && Object.keys(state.userTokens).length === 0) {
      setState(prev => ({ ...prev, showTokenModal: true }));
      return;
    }

    if (!confirm("Are you sure you want to execute this replacement? This will create Merge Requests.")) return;

    setState(prev => ({ ...prev, executing: true, error: null }));
    try {
      const matchMap = new Map<string, ReplaceMatch>();
      for (const m of state.preview) {
        const key = `${m.repo}:${m.file}`;
        if (!matchMap.has(key)) {
          matchMap.set(key, {
            repository_id: m.repository_id,
            repository_name: m.repo,
            file_path: m.file,
          });
        }
      }
      const matches = Array.from(matchMap.values());

      const tokenMap: Record<string, string> = {};
      for (const [connId, token] of Object.entries(state.userTokens)) {
        if (token.trim()) {
          tokenMap[connId] = token;
        }
      }

      const response = await api.replaceExecute({
        search_pattern: state.searchPattern,
        replace_with: state.replaceWith,
        matches: matches,
        mr_title: state.mrTitle || `Replace "${state.searchPattern}" with "${state.replaceWith}"`,
        user_tokens: Object.keys(tokenMap).length > 0 ? tokenMap : undefined,
      });
      setState(prev => ({ ...prev, result: response, preview: null }));
    } catch (err) {
      setState(prev => ({ ...prev, error: err instanceof Error ? err.message : "Execution failed" }));
    } finally {
      setState(prev => ({ ...prev, executing: false }));
    }
  };

  const connectionsWithoutToken = state.preview
    ? [...state.preview
      .filter(m => m.connection_has_token === false && m.connection_id !== undefined)
      .reduce((acc, m) => {
        const key = String(m.connection_id);
        if (!acc.has(key)) {
          acc.set(key, {
            connectionId: String(m.connection_id),
            connectionName: m.connection_name || `Connection ${m.connection_id}`,
            repos: new Set<string>(),
          });
        }
        acc.get(key)!.repos.add(m.repo);
        return acc;
      }, new Map<string, { connectionId: string; connectionName: string; repos: Set<string> }>())
      .values()
    ].map(c => ({ ...c, repos: [...c.repos] }))
    : [];

  const handleSaveTokens = () => {
    setState(prev => ({ ...prev, showTokenModal: false }));
    handleExecute();
  };

  const allTokensProvided = connectionsWithoutToken.every(
    conn => state.userTokens[conn.connectionId]?.trim()
  );

  return (
    <div className="h-full overflow-auto">
      <div className="container mx-auto px-4 py-6 max-w-full">
        <div className="max-w-6xl mx-auto">
          <div className="flex items-center justify-between mb-4 sm:mb-6">
            <div className="flex items-center gap-2 sm:gap-3">
              <RefreshCw className="w-5 h-5 sm:w-6 sm:h-6 text-gray-600 dark:text-gray-400" />
              <h1 className="text-xl sm:text-2xl font-bold">Search & Replace</h1>
            </div>
            <div
              className="flex items-center gap-2 px-2.5 sm:px-3 py-1.5 sm:py-2 text-sm font-medium rounded-lg opacity-0 pointer-events-none select-none"
              aria-hidden="true"
            >
              <RefreshCw className="w-4 h-4" />
              <span className="hidden sm:inline">Easter Egg! Ths is only meant to balance the layout. c:</span>
            </div>
          </div>

          {state.reposReadOnly && !state.hideReadOnlyBanner && (
            <div className="mb-4 p-3 sm:p-4 bg-amber-50 dark:bg-amber-900/20 border border-amber-200 dark:border-amber-800 text-amber-700 dark:text-amber-400 rounded-lg">
              <div className="flex items-start gap-3">
                <AlertCircle className="w-5 h-5 sm:w-5 sm:h-5 flex-shrink-0 mt-0.5" />
                <div>
                  <p className="text-sm font-bold">
                    Read-only mode enabled
                  </p>
                  <p className="text-sm text-amber-700 dark:text-amber-300 mt-1">
                    Repositories are configured in read-only mode. To create Merge Requests, you&apos;ll need to provide your personal access token when executing replacements.
                    The token will only be used for the current session and is not stored.
                  </p>
                </div>
              </div>
            </div>
          )}

          {state.showTokenModal && (
            <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50">
              <div className="bg-white dark:bg-gray-800 rounded-xl shadow-xl p-6 max-w-lg w-full mx-4">
                <div className="flex items-center gap-3 mb-4">
                  <Key className="w-6 h-6 text-blue-500" />
                  <h2 className="text-lg font-semibold">Personal Access Token{connectionsWithoutToken.length > 1 ? 's' : ''} Required</h2>
                </div>
                {connectionsWithoutToken.length > 0 ? (
                  <>
                    <p className="text-sm text-gray-600 dark:text-gray-400 mb-4">
                      {connectionsWithoutToken.length === 1
                        ? "The following code host doesn't have authentication configured. To create Merge Requests, please provide your personal access token:"
                        : "The following code hosts don't have authentication configured. To create Merge Requests, please provide a personal access token for each:"}
                    </p>
                    <div className="space-y-4 max-h-80 overflow-auto">
                      {connectionsWithoutToken.map(conn => (
                        <div key={conn.connectionId} className="bg-gray-50 dark:bg-gray-900 rounded-lg p-3">
                          <div className="flex items-center gap-2 mb-2">
                            <GitBranch className="w-4 h-4 text-blue-500" />
                            <span className="font-medium text-sm">{conn.connectionName}</span>
                            <span className="text-xs text-gray-500">({conn.repos.length} repo{conn.repos.length > 1 ? 's' : ''})</span>
                          </div>
                          <ul className="mb-2 text-xs text-gray-500 dark:text-gray-400 pl-6 max-h-16 overflow-auto">
                            {conn.repos.slice(0, 3).map(repo => (
                              <li key={repo} className="truncate">• {repo}</li>
                            ))}
                            {conn.repos.length > 3 && (
                              <li className="text-gray-400">... and {conn.repos.length - 3} more</li>
                            )}
                          </ul>
                          <input
                            type="password"
                            value={state.userTokens[conn.connectionId] || ''}
                            onChange={(e) => setState(prev => ({
                              ...prev,
                              userTokens: {
                                ...prev.userTokens,
                                [conn.connectionId]: e.target.value
                              }
                            }))}
                            placeholder={`Token for ${conn.connectionName}`}
                            aria-label={`Access token for ${conn.connectionName}`}
                            className="w-full px-3 py-2 text-sm border border-gray-200 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 focus:outline-none focus:border-blue-500 dark:focus:border-blue-500"
                          />
                        </div>
                      ))}
                    </div>
                  </>
                ) : (
                  <p className="text-sm text-gray-600 dark:text-gray-400 mb-4">
                    Repositories are in read-only mode. To create Merge Requests, please provide your personal access token.
                  </p>
                )}
                <p className="text-xs text-gray-500 dark:text-gray-500 mt-3 mb-3">
                  These tokens are used for the current session only and are not stored in your browser.
                </p>
                <div className="flex justify-end gap-2">
                  <button
                    onClick={() => setState(prev => ({ ...prev, showTokenModal: false }))}
                    className="px-4 py-2 text-sm font-medium text-gray-600 dark:text-gray-400 hover:bg-gray-100 dark:hover:bg-gray-700 rounded-lg transition-colors"
                  >
                    Cancel
                  </button>
                  <button
                    onClick={handleSaveTokens}
                    disabled={!allTokensProvided}
                    className="px-4 py-2 text-sm font-medium bg-blue-600 text-white hover:bg-blue-700 rounded-lg transition-colors disabled:opacity-50"
                  >
                    Continue
                  </button>
                </div>
              </div>
            </div>
          )}

          <form onSubmit={handlePreview} className="mb-6">
            <div className="mb-3">
              <ContextSwitcher labelPrefix="Replacing in" />
            </div>

            <div className="flex flex-col sm:flex-row gap-2 mb-2">
              <div className="flex-1 grid grid-cols-1 sm:grid-cols-3 gap-2">
                <div className="relative sm:col-span-2">
                  <div className="relative bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 shadow-sm hover:shadow-md transition-shadow focus-within:ring-2 focus-within:ring-blue-500 focus-within:border-transparent">
                    <button
                      ref={historyButtonRef}
                      type="button"
                      onClick={() => {
                        setShowHistory(!showHistory);
                        setShowSuggestions(false);
                        setHistoryFilter("");
                        setHistorySelectedIndex(0);
                      }}
                      className={`absolute left-2 top-1/2 -translate-y-1/2 p-1 rounded transition-colors z-10 ${showHistory
                        ? "text-blue-500 bg-blue-50 dark:bg-blue-900/30"
                        : "text-gray-400 hover:text-gray-600 dark:hover:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700"
                        }`}
                      title="Search history"
                    >
                      <History className="w-4 h-4" />
                    </button>

                    {showHistory && (
                      <div
                        ref={historyRef}
                        className="absolute top-full left-0 right-0 mt-1 bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-xl shadow-lg z-50 overflow-hidden"
                      >
                        <div className="p-2 border-b border-gray-100 dark:border-gray-700">
                          <div className="relative">
                            <Search className="absolute left-2.5 top-1/2 -translate-y-1/2 w-3.5 h-3.5 text-gray-400" />
                            <input
                              type="text"
                              value={historyFilter}
                              onChange={(e) => {
                                setHistoryFilter(e.target.value);
                                setHistorySelectedIndex(0);
                              }}
                              onKeyDown={handleHistoryKeyDown}
                              placeholder="Filter history..."
                              aria-label="Filter search history"
                              className="w-full pl-8 pr-3 py-1.5 text-sm bg-gray-50 dark:bg-gray-700/50 border border-gray-200 dark:border-gray-600 rounded-lg focus:outline-none focus:ring-1 focus:ring-blue-500"
                            />
                          </div>
                        </div>

                        {filteredHistory.length === 0 ? (
                          <div className="p-4 text-center text-sm text-gray-500 dark:text-gray-400">
                            {searchHistory.length === 0 ? "No search history yet" : "No matching searches"}
                          </div>
                        ) : (
                          <>
                            <div className="max-h-64 overflow-y-auto">
                              {filteredHistory.map((item, index) => (
                                <button
                                  key={`${item.query}-${item.timestamp}`}
                                  type="button"
                                  onClick={() => selectHistoryItem(item)}
                                  onKeyDown={(e) => {
                                    if (e.key === 'Enter' || e.key === ' ') {
                                      selectHistoryItem(item);
                                    }
                                  }}
                                  className={`w-full flex items-center gap-3 px-4 py-2.5 text-left transition-colors group ${index === historySelectedIndex
                                    ? "bg-blue-50 dark:bg-blue-900/30"
                                    : "hover:bg-gray-50 dark:hover:bg-gray-700/50"
                                    }`}
                                >
                                  <Clock className="w-4 h-4 text-gray-400 flex-shrink-0" />
                                  <span className="font-mono text-sm text-gray-700 dark:text-gray-300 truncate flex-1">
                                    {item.query}
                                  </span>
                                  <span
                                    role="button"
                                    tabIndex={-1}
                                    onClick={(e) => { e.stopPropagation(); deleteHistoryItem(e, item.query); }}
                                    onKeyDown={(e) => { if (e.key === 'Enter' || e.key === ' ') { e.stopPropagation(); deleteHistoryItem(e, item.query); } }}
                                    className="p-1 text-gray-400 hover:text-red-500 opacity-0 group-hover:opacity-100 transition-opacity flex-shrink-0 cursor-pointer"
                                    title="Remove from history"
                                  >
                                    <X className="w-3.5 h-3.5" />
                                  </span>
                                </button>
                              ))}
                            </div>
                            <div className="p-2 border-t border-gray-100 dark:border-gray-700 flex items-center justify-between">
                              <span className="text-xs text-gray-400">
                                {filteredHistory.length} of {searchHistory.length} searches
                              </span>
                              <button
                                type="button"
                                onClick={clearAllHistory}
                                className="text-xs text-red-500 hover:text-red-600 dark:hover:text-red-400"
                              >
                                Clear all
                              </button>
                            </div>
                          </>
                        )}
                      </div>
                    )}

                    <input
                      ref={inputRef}
                      type="text"
                      id="replace-search-pattern"
                      value={state.searchPattern}
                      onChange={(e) => handleInputChange(e.target.value)}
                      onFocus={() => {
                        updateSuggestions(state.searchPattern);
                        setShowSuggestions(true);
                        setShowHistory(false);
                      }}
                      onKeyDown={handleKeyDown}
                      onScroll={(e) => {
                        if (highlightRef.current) {
                          highlightRef.current.style.transform = `translateX(-${e.currentTarget.scrollLeft}px)`;
                        }
                      }}
                      className="w-full pl-9 pr-3 py-2 text-sm rounded-lg bg-transparent font-mono text-transparent caret-gray-900 dark:caret-white outline-none"
                      autoComplete="off"
                      aria-label="Search pattern"
                    />

                    <div className="absolute inset-y-0 left-9 right-3 overflow-hidden pointer-events-none rounded-lg">
                      <div
                        ref={highlightRef}
                        className="py-2 text-sm font-mono flex items-center whitespace-pre h-full"
                        style={{ width: 'max-content', minWidth: '100%' }}
                      >
                        {state.searchPattern ? <HighlightedQuery query={state.searchPattern} /> : (
                          <span className="text-gray-400">
                            <span className="sm:hidden">Search code...</span>
                            <span className="hidden sm:inline">Search code... (type repo:, lang:, file: for filters)</span>
                          </span>
                        )}
                      </div>
                    </div>

                    {showSuggestions && filteredSuggestions.length > 0 && (
                      <div
                        ref={suggestionsRef}
                        className="absolute top-full left-0 right-0 mt-1 bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-xl shadow-lg z-50 overflow-hidden"
                        role="listbox"
                      >
                        <div className="p-2 border-b border-gray-100 dark:border-gray-700">
                          <span className="text-xs text-gray-500 dark:text-gray-400 font-medium">
                            Narrow your search
                          </span>
                        </div>
                        <div className="max-h-64 overflow-y-auto">
                          {filteredSuggestions.map((suggestion, index) => (
                            <button
                              key={`${suggestion.type}-${suggestion.value}`}
                              type="button"
                              role="option"
                              aria-selected={index === selectedIndex}
                              data-replace-suggestion-index={index}
                              onClick={() => selectSuggestion(suggestion)}
                              className={`w-full flex items-center gap-3 px-4 py-2.5 text-left transition-colors ${index === selectedIndex
                                ? "bg-blue-50 dark:bg-blue-900/30"
                                : "hover:bg-gray-50 dark:hover:bg-gray-700/50"
                                }`}
                            >
                              {suggestion.icon}
                              <span className="font-mono text-sm text-cyan-600 dark:text-cyan-400">
                                {suggestion.value}
                              </span>
                              {suggestion.description && (
                                <span className="text-sm text-gray-500 dark:text-gray-400">
                                  {suggestion.description}
                                </span>
                              )}
                              {index === selectedIndex && (
                                <span className="ml-auto text-xs text-gray-400">Add</span>
                              )}
                            </button>
                          ))}
                        </div>
                        <div className="p-2 border-t border-gray-100 dark:border-gray-700 text-xs text-gray-400 flex items-center gap-4">
                          <span><kbd className="px-1.5 py-0.5 bg-gray-100 dark:bg-gray-700 rounded">↑↓</kbd> navigate</span>
                          <span><kbd className="px-1.5 py-0.5 bg-gray-100 dark:bg-gray-700 rounded">Tab</kbd> select</span>
                          <span><kbd className="px-1.5 py-0.5 bg-gray-100 dark:bg-gray-700 rounded">Esc</kbd> close</span>
                        </div>
                      </div>
                    )}
                  </div>
                </div>
                <input
                  type="text"
                  value={state.replaceWith}
                  onChange={(e) => setState(prev => ({ ...prev, replaceWith: e.target.value }))}
                  placeholder="Replace with..."
                  className="w-full px-3 py-2 text-sm border border-gray-200 dark:border-gray-700 rounded-lg bg-white dark:bg-gray-800 shadow-sm hover:shadow-md transition-shadow focus:outline-none focus:border-blue-500 dark:focus:border-blue-500 font-mono"
                  aria-label="Replace with"
                />
              </div>
              <button
                type="submit"
                disabled={state.loading || !state.searchPattern.trim()}
                className="hidden sm:flex items-center justify-center gap-2 px-4 py-2 text-sm font-medium border border-blue-500 dark:border-blue-400 text-blue-600 dark:text-blue-400 hover:bg-blue-50 dark:hover:bg-blue-900/30 rounded-lg transition-colors disabled:opacity-50"
              >
                <Eye className="w-4 h-4" />
                {state.loading ? "Loading..." : "Preview"}
              </button>
            </div>

            <div className="flex items-center justify-between gap-3 sm:gap-4 text-sm pl-1">
              <div className="flex flex-wrap items-center gap-3 sm:gap-4">
                <button
                  type="button"
                  onClick={() => setState(prev => ({ ...prev, showOptions: !prev.showOptions }))}
                  className="flex items-center gap-1 text-blue-600 dark:text-blue-400 hover:text-blue-800 dark:hover:text-blue-300 transition-colors"
                >
                  <span className="sm:hidden">{state.showOptions ? "Less" : "More"}</span>
                  <span className="hidden sm:inline">{state.showOptions ? "Hide options" : "More options"}</span>
                  {state.showOptions ? <ChevronUp className="w-4 h-4" /> : <ChevronDown className="w-4 h-4" />}
                </button>
              </div>
              <button
                type="submit"
                disabled={state.loading || !state.searchPattern.trim()}
                className="sm:hidden flex items-center gap-1.5 px-3 py-1.5 text-sm font-medium border border-blue-500 dark:border-blue-400 text-blue-600 dark:text-blue-400 hover:bg-blue-50 dark:hover:bg-blue-900/30 rounded-lg transition-colors disabled:opacity-50"
              >
                <Eye className="w-4 h-4" />
                Preview
              </button>
            </div>

            {state.showOptions && (
              <div className="mt-3 p-4 bg-gray-50 dark:bg-gray-800/50 rounded-lg border border-gray-100 dark:border-gray-700 grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
                <div>
                  <label htmlFor="replace-file-patterns" className="block text-xs font-medium text-gray-700 dark:text-gray-300 mb-1">
                    File patterns
                  </label>
                  <input
                    ref={filePatternsInputRef}
                    id="replace-file-patterns"
                    type="text"
                    value={state.filePatterns}
                    onChange={(e) => {
                      const val = e.target.value;
                      setState(prev => ({ ...prev, filePatterns: val }));
                      filePatternsRef.current = val;
                    }}
                    placeholder="*.go, *.ts"
                    className="w-full px-3 py-1.5 text-sm border border-gray-200 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 focus:outline-none focus:border-blue-500 dark:focus:border-blue-500"
                  />
                </div>
                <div>
                  <label htmlFor="replace-repos" className="block text-xs font-medium text-gray-700 dark:text-gray-300 mb-1">
                    Repositories
                  </label>
                  <input
                    ref={reposInputRef}
                    id="replace-repos"
                    type="text"
                    value={state.repos}
                    onChange={(e) => {
                      const val = e.target.value;
                      setState(prev => ({ ...prev, repos: val }));
                      reposRef.current = val;
                    }}
                    placeholder="org/repo1, org/repo2"
                    className="w-full px-3 py-1.5 text-sm border border-gray-200 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 focus:outline-none focus:border-blue-500 dark:focus:border-blue-500"
                  />
                </div>
                <div>
                  <label htmlFor="mr-title" className="block text-xs font-medium text-gray-700 dark:text-gray-300 mb-1">
                    MR/PR title
                  </label>
                  <input
                    id="mr-title"
                    type="text"
                    value={state.mrTitle}
                    onChange={(e) => setState(prev => ({ ...prev, mrTitle: e.target.value }))}
                    placeholder="Auto-generated if empty"
                    className="w-full px-3 py-1.5 text-sm border border-gray-200 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 focus:outline-none focus:border-blue-500 dark:focus:border-blue-500"
                  />
                </div>
              </div>
            )}

            {state.preview && state.preview.length > 0 && (
              <div className="mt-4 flex flex-col sm:flex-row sm:items-center gap-2 sm:gap-3">
                <button
                  type="button"
                  onClick={handleExecute}
                  disabled={state.executing || state.previewStale || state.previewTruncated}
                  className="flex items-center justify-center gap-2 px-3 sm:px-4 py-2 text-sm font-medium border border-green-500 dark:border-green-500 text-green-600 dark:text-green-400 hover:bg-green-50 dark:hover:bg-green-900/20 rounded-lg transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
                >
                  {state.executing ? (
                    <Loader2 className="w-4 h-4 animate-spin" />
                  ) : (
                    <Play className="w-4 h-4" />
                  )}
                  <span>{state.executing ? "Executing..." : `Execute (${state.preview.length} matches)`}</span>
                </button>
                {state.previewTruncated && (
                  <span className="text-xs sm:text-sm text-red-600 dark:text-red-400">
                    Cannot execute — results are truncated. Use filters to narrow your search.
                  </span>
                )}
                {state.previewStale && !state.previewTruncated && (
                  <span className="text-xs sm:text-sm text-amber-600 dark:text-amber-400">
                    Search/options changed — preview again
                  </span>
                )}
              </div>
            )}
          </form>

          {state.error && (
            <div className="mb-4 p-4 bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-lg text-red-700 dark:text-red-400">
              {state.error}
            </div>
          )}

          {state.loading && (
            <div className="text-center py-12">
              <Loader2 className="w-8 h-8 animate-spin text-blue-600 mx-auto" />
              <p className="mt-3 text-gray-500 dark:text-gray-400">Searching...</p>
            </div>
          )}

          {state.result && (
            <div className="mb-4 p-4 bg-green-50 dark:bg-green-900/20 border border-green-200 dark:border-green-800 rounded-lg text-green-700 dark:text-green-400">
              <p className="font-medium">{state.result.message}</p>
              <p className="text-sm">Job ID: {state.result.job_id}</p>
            </div>
          )}

          {!state.loading && state.preview && (
            <PreviewResults
              preview={state.preview}
              searchPattern={state.searchPattern}
              replaceWith={state.replaceWith}
              truncated={state.previewTruncated}
              totalCount={state.previewTotalCount}
              duration={state.previewDuration}
              stats={state.previewStats}
            />
          )}
        </div>
      </div>
    </div>
  );
}
