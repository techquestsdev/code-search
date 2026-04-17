"use client";

import React, {
  useState,
  useEffect,
  useRef,
  useCallback,
  useReducer,
} from "react";
import { useRouter, useSearchParams } from "next/navigation";
import {
  api,
  SearchResult,
  SearchResponse,
  SearchSuggestionsResponse,
  buildBrowseUrl,
} from "@/lib/api";
import { useActiveContext } from "@/hooks/useContexts";
import { ContextSwitcher } from "./ContextSwitcher";
import {
  Search,
  ChevronDown,
  ChevronUp,
  FileCode,
  FolderGit2,
  ChevronRight,
  Filter,
  GitBranch,
  Code,
  ExternalLink,
  History,
  X,
  Clock,
  AlertTriangle,
  Eye,
  Tag,
  FileText,
  Braces,
  Hash,
  ToggleLeft,
  Loader2,
} from "lucide-react";
import Link from "next/link";

interface Suggestion {
  type: "filter" | "repo" | "language";
  value: string;
  description?: string;
  icon?: React.ReactNode;
}

interface SearchHistoryItem {
  query: string;
  timestamp: number;
  isRegex?: boolean;
  caseSensitive?: boolean;
}

// Default filter keywords - only operators that work with zoekt-git-index.
// Ensures UI shows all operators even before the backend is restarted.
const DEFAULT_FILTERS: {
  keyword: string;
  description: string;
  example: string;
}[] = [
  {
    keyword: "repo:",
    description: "Filter by repository",
    example: "repo:org/repo",
  },
  {
    keyword: "file:",
    description: "Filter by file path pattern",
    example: "file:*.go",
  },
  {
    keyword: "lang:",
    description: "Filter by language name",
    example: "lang:typescript",
  },
  {
    keyword: "case:yes",
    description: "Case sensitive search",
    example: "case:yes func",
  },
  {
    keyword: "case:no",
    description: "Case insensitive search (default)",
    example: "case:no FOO",
  },
  {
    keyword: "sym:",
    description: "Search for symbols/definitions",
    example: "sym:main",
  },
  {
    keyword: "content:",
    description: "Search content only (not file names)",
    example: "content:FOO",
  },
  {
    keyword: "branch:",
    description: "Filter by branch/tag",
    example: "branch:main",
  },
  {
    keyword: "type:",
    description: "Result type: filematch, filename, or repo",
    example: "type:filename main",
  },
  {
    keyword: "regex:",
    description: "Treat pattern as regex",
    example: "regex:func\\s+main",
  },
  {
    keyword: "-repo:",
    description: "Exclude repository",
    example: "-repo:test",
  },
  {
    keyword: "-file:",
    description: "Exclude file pattern",
    example: "-file:*_test.go",
  },
  {
    keyword: "-lang:",
    description: "Exclude language",
    example: "-lang:markdown",
  },
  {
    keyword: "-content:",
    description: "Exclude content pattern",
    example: "-content:TODO",
  },
];

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
    localStorage.setItem(
      SEARCH_HISTORY_KEY,
      JSON.stringify(history.slice(0, MAX_HISTORY_ITEMS))
    );
  } catch {
    // Ignore storage errors
  }
}

function addToSearchHistory(
  query: string,
  isRegex?: boolean,
  caseSensitive?: boolean
) {
  if (!query.trim()) return;
  const history = getSearchHistory();
  // Remove duplicates
  const filtered = history.filter((h) => h.query !== query);
  // Add new item at the beginning
  const newHistory = [
    { query, timestamp: Date.now(), isRegex, caseSensitive },
    ...filtered,
  ];
  saveSearchHistory(newHistory);
}

interface FormState {
  query: string;
  showOptions: boolean;
  filePatterns: string;
  repos: string;
  limit: number;
}

function formReducer(state: FormState, update: Partial<FormState>): FormState {
  return { ...state, ...update };
}

export function SearchForm({
  onResults,
  onLoading,
}: {
  onResults: (results: SearchResponse | null) => void;
  onLoading: (loading: boolean) => void;
}) {
  const router = useRouter();
  const searchParams = useSearchParams();
  const { activeContext, getRepoFilter } = useActiveContext();

  const [formState, updateForm] = useReducer(formReducer, {
    query: "",
    showOptions: false,
    filePatterns: "",
    repos: "",
    limit: 0,
  });
  const { query, showOptions, filePatterns, repos, limit } = formState;
  const [initialSearchDone, setInitialSearchDone] = useState(false);
  const isManualSearch = useRef(false);

  // Context Manager state
  const [_showContextManager, _setShowContextManager] = useState(false);

  // Refs to track current values for form submission (avoids stale state issues)
  const filePatternsRef = useRef(filePatterns);
  const reposRef = useRef(repos);
  const limitRef = useRef(limit);
  const filePatternsInputRef = useRef<HTMLInputElement>(null);
  const reposInputRef = useRef<HTMLInputElement>(null);
  const limitInputRef = useRef<HTMLInputElement>(null);

  // Autocomplete state
  const [showSuggestions, setShowSuggestions] = useState(false);
  const [suggestions, setSuggestions] =
    useState<SearchSuggestionsResponse | null>(null);
  const [filteredSuggestions, setFilteredSuggestions] = useState<Suggestion[]>(
    []
  );
  const [selectedIndex, setSelectedIndex] = useState(0);
  const inputRef = useRef<HTMLInputElement>(null);
  const highlightRef = useRef<HTMLDivElement>(null);
  const suggestionsRef = useRef<HTMLDivElement>(null);

  // Search history state
  const [showHistory, setShowHistory] = useState(false);
  const [searchHistory, setSearchHistory] = useState<SearchHistoryItem[]>([]);
  const [historyFilter, setHistoryFilter] = useState("");
  const [historySelectedIndex, setHistorySelectedIndex] = useState(0);
  const historyRef = useRef<HTMLDivElement>(null);
  const historyButtonRef = useRef<HTMLButtonElement>(null);

  // Streaming configuration
  const [enableStreaming, setEnableStreaming] = useState(false);

  // AbortController for cancelling streaming searches
  const abortControllerRef = useRef<AbortController | null>(null);

  // Load search history and settings on mount
  useEffect(() => {
    setSearchHistory(getSearchHistory());
    // Load streaming setting
    api
      .getUISettings()
      .then((settings) => {
        setEnableStreaming(settings.enable_streaming);
      })
      .catch(() => {
        // Default to false if settings fail to load
        setEnableStreaming(false);
      });

    // Auto-focus the search input on mount
    inputRef.current?.focus();
  }, []);

  // Filter history based on search
  const filteredHistory = searchHistory.filter((h) =>
    h.query.toLowerCase().includes(historyFilter.toLowerCase())
  );

  // Initialize from URL params on mount (skip if manual search triggered the URL change)
  useEffect(() => {
    // Skip if this URL change was triggered by a manual search
    if (isManualSearch.current) {
      isManualSearch.current = false;
      return;
    }

    const q = searchParams.get("q") || "";
    const files = searchParams.get("files") || "";
    const reposParam = searchParams.get("repos") || "";
    const limitParam = searchParams.get("limit");

    if (q) {
      const limitVal = limitParam ? Number(limitParam) : limitRef.current;
      updateForm({
        query: q,
        filePatterns: files,
        repos: reposParam,
        limit: limitVal,
        showOptions: !!(files || reposParam || limitParam),
      });
      filePatternsRef.current = files;
      reposRef.current = reposParam;
      if (limitParam) {
        limitRef.current = limitVal;
      }
    }
  }, [searchParams]);

  // Auto-search when URL has query param (on initial load)
  useEffect(() => {
    const q = searchParams.get("q");
    if (q && !initialSearchDone) {
      setInitialSearchDone(true);
      const files = searchParams.get("files") || "";
      const reposParam = searchParams.get("repos") || "";
      const limitParam = searchParams.get("limit");

      onLoading(true);

      // Build query with context/repo filter
      let finalQuery = q;

      if (activeContext && activeContext.repos.length > 0) {
        if (reposParam) {
          // If user specified repos, filter context repos by that pattern
          const userPatterns = reposParam
            .split(",")
            .map((r) => r.trim().toLowerCase());
          const filtered = activeContext.repos.filter((repo) =>
            userPatterns.some((pattern) =>
              repo.name.toLowerCase().includes(pattern)
            )
          );
          if (filtered.length > 0) {
            // Build regex OR pattern from filtered repos
            const repoPattern = filtered
              .map((r) => r.name.replace(/[.*+?^${}()|[\]\\]/g, "\\$&"))
              .join("|");
            finalQuery = `repo:(${repoPattern}) ${q}`;
          } else {
            // No match - use all context repos
            const repoPattern = activeContext.repos
              .map((r) => r.name.replace(/[.*+?^${}()|[\]\\]/g, "\\$&"))
              .join("|");
            finalQuery = `repo:(${repoPattern}) ${q}`;
          }
        } else {
          // No user filter - use all context repos
          const repoPattern = activeContext.repos
            .map((r) => r.name.replace(/[.*+?^${}()|[\]\\]/g, "\\$&"))
            .join("|");
          finalQuery = `repo:(${repoPattern}) ${q}`;
        }
      } else if (reposParam) {
        // No context but user specified repos - build regex OR pattern for multiple repos
        const repoList = reposParam
          .split(",")
          .map((r) => r.trim())
          .filter(Boolean);
        if (repoList.length > 1) {
          const repoPattern = repoList
            .map((r) => r.replace(/[.*+?^${}()|[\]\\]/g, "\\$&"))
            .join("|");
          finalQuery = `repo:(${repoPattern}) ${q}`;
        } else if (repoList.length === 1) {
          finalQuery = `repo:${repoList[0]} ${q}`;
        }
      }

      // Build search request
      const searchRequest = {
        query: finalQuery,
        file_patterns: files
          ? files.split(",").map((p) => p.trim())
          : undefined,
        repos: undefined,
        limit: limitParam ? Number(limitParam) : undefined,
        context_lines: 3,
      };

      // Cancel any existing streaming search
      if (abortControllerRef.current) {
        abortControllerRef.current.abort();
      }
      const abortController = new AbortController();
      abortControllerRef.current = abortController;

      (async () => {
        try {
          if (enableStreaming) {
            // Stream results progressively
            const results: SearchResult[] = [];
            let totalCount = 0;
            let truncated = false;
            let duration = "";
            let stats = { files_searched: 0, repos_searched: 0 };
            const startTime = Date.now();
            let lastUpdateTime = 0;

            // Helper to yield control to browser and allow paint
            const yieldToBrowser = () =>
              new Promise<void>((resolve) => {
                requestAnimationFrame(() => resolve());
              });

            for await (const event of api.searchStream(
              searchRequest,
              abortController.signal
            )) {
              // Check if aborted
              if (abortController.signal.aborted) break;

              if (event.type === "result" && event.result) {
                results.push(event.result);
                const now = Date.now();
                // Update UI less frequently to reduce blocking - every 100ms
                if (results.length === 1 || now - lastUpdateTime > 100) {
                  lastUpdateTime = now;
                  onResults({
                    results: [...results],
                    total_count: totalCount || results.length,
                    truncated,
                    duration: `${Date.now() - startTime}ms`,
                    stats,
                  });
                  // Yield control to allow browser to paint
                  await yieldToBrowser();
                }
              } else if (event.type === "done") {
                totalCount = event.total_count || results.length;
                truncated = event.truncated || totalCount > results.length;
                duration = event.duration || `${Date.now() - startTime}ms`;
                stats = event.stats || stats;
              }
            }
            // Final update with all results (if not aborted)
            if (!abortController.signal.aborted) {
              onResults({
                results,
                total_count: totalCount,
                truncated,
                duration,
                stats,
              });
            }
          } else {
            // Use batch search (non-streaming)
            const response = await api.search(searchRequest);
            onResults(response);
          }
        } catch (err) {
          // Don't report errors from aborted requests
          if (err instanceof Error && err.name === "AbortError") return;
          onResults(null);
        } finally {
          onLoading(false);
        }
      })();
    }
  }, [
    searchParams,
    initialSearchDone,
    onLoading,
    onResults,
    activeContext,
    getRepoFilter,
    enableStreaming,
  ]);

  // Fetch suggestions on mount
  useEffect(() => {
    api.getSearchSuggestions().then(setSuggestions).catch(console.error);
  }, []);

  // Filter suggestions based on current input
  const updateSuggestions = useCallback(
    (value: string) => {
      if (!suggestions) return;

      const parts = value.split(/\s+/);
      const lastPart = parts[parts.length - 1] || "";
      const newSuggestions: Suggestion[] = [];

      // Helper to get icon for filter keyword
      const getFilterIcon = (keyword: string) => {
        const kw = keyword.replace(/^-/, "");
        if (kw.startsWith("repo:")) {
          return (
            <FolderGit2
              className={`h-4 w-4 ${keyword.startsWith("-") ? "text-red-500" : "text-blue-500"}`}
            />
          );
        }
        if (kw.startsWith("file:") || kw.startsWith("path:")) {
          return (
            <FileText
              className={`h-4 w-4 ${keyword.startsWith("-") ? "text-red-500" : "text-orange-500"}`}
            />
          );
        }
        if (kw.startsWith("lang:")) {
          return (
            <Code
              className={`h-4 w-4 ${keyword.startsWith("-") ? "text-red-500" : "text-green-500"}`}
            />
          );
        }
        if (kw.startsWith("sym:") || kw.startsWith("symbol:")) {
          return <Braces className="h-4 w-4 text-purple-500" />;
        }
        if (kw.startsWith("branch:") || kw.startsWith("rev:")) {
          return <GitBranch className="h-4 w-4 text-cyan-500" />;
        }
        if (kw.startsWith("case:")) {
          return <ToggleLeft className="h-4 w-4 text-amber-500" />;
        }
        if (kw.startsWith("content:")) {
          return (
            <Hash
              className={`h-4 w-4 ${keyword.startsWith("-") ? "text-red-500" : "text-pink-500"}`}
            />
          );
        }
        if (kw.startsWith("type:")) {
          return <Tag className="h-4 w-4 text-teal-500" />;
        }
        if (kw.startsWith("regex:")) {
          return <Code className="h-4 w-4 text-purple-500" />;
        }
        return <Filter className="h-4 w-4 text-gray-500" />;
      };

      // Check if we're typing a filter keyword
      if (lastPart.startsWith("repo:")) {
        const search = lastPart.slice(5).toLowerCase();
        suggestions.repos
          .filter((r) => r.display_name.toLowerCase().includes(search))
          .slice(0, 8)
          .forEach((r) =>
            newSuggestions.push({
              type: "repo",
              value: `repo:${r.name}`,
              description: r.display_name,
              icon: <FolderGit2 className="h-4 w-4 text-blue-500" />,
            })
          );
      } else if (lastPart.startsWith("lang:")) {
        const search = lastPart.slice(5).toLowerCase();
        suggestions.languages
          .filter((l) => l.name.toLowerCase().includes(search))
          .slice(0, 8)
          .forEach((l) =>
            newSuggestions.push({
              type: "language",
              value: `lang:${l.name}`,
              icon: <Code className="h-4 w-4 text-green-500" />,
            })
          );
      } else if (
        lastPart.startsWith("file:") ||
        lastPart.startsWith("path:") ||
        lastPart.startsWith("-file:")
      ) {
        // File pattern filter - no suggestions, user types their pattern
      } else if (
        lastPart.startsWith("sym:") ||
        lastPart.startsWith("symbol:")
      ) {
        // Symbol filter - no suggestions
      } else if (
        lastPart.startsWith("branch:") ||
        lastPart.startsWith("rev:")
      ) {
        // Branch filter - no suggestions
      } else if (lastPart.startsWith("case:")) {
        newSuggestions.push(
          {
            type: "filter",
            value: "case:yes",
            description: "Case sensitive",
            icon: <ToggleLeft className="h-4 w-4 text-amber-500" />,
          },
          {
            type: "filter",
            value: "case:no",
            description: "Case insensitive",
            icon: <ToggleLeft className="h-4 w-4 text-amber-500" />,
          }
        );
      } else if (lastPart.startsWith("type:")) {
        newSuggestions.push(
          {
            type: "filter",
            value: "type:filematch",
            description: "File matches (default)",
            icon: <Tag className="h-4 w-4 text-teal-500" />,
          },
          {
            type: "filter",
            value: "type:filename",
            description: "Matching file names",
            icon: <Tag className="h-4 w-4 text-teal-500" />,
          },
          {
            type: "filter",
            value: "type:repo",
            description: "Matching repositories",
            icon: <Tag className="h-4 w-4 text-teal-500" />,
          }
        );
      } else if (
        lastPart.startsWith("regex:") ||
        lastPart.startsWith("content:")
      ) {
        // Freeform patterns, no value suggestions
      } else if (lastPart.startsWith("-repo:")) {
        const search = lastPart.slice(6).toLowerCase();
        suggestions.repos
          .filter((r) => r.display_name.toLowerCase().includes(search))
          .slice(0, 8)
          .forEach((r) =>
            newSuggestions.push({
              type: "repo",
              value: `-repo:${r.name}`,
              description: `Exclude ${r.display_name}`,
              icon: <FolderGit2 className="h-4 w-4 text-red-500" />,
            })
          );
      } else if (lastPart.startsWith("-lang:")) {
        const search = lastPart.slice(6).toLowerCase();
        suggestions.languages
          .filter((l) => l.name.toLowerCase().includes(search))
          .slice(0, 8)
          .forEach((l) =>
            newSuggestions.push({
              type: "language",
              value: `-lang:${l.name}`,
              description: `Exclude ${l.name}`,
              icon: <Code className="h-4 w-4 text-red-500" />,
            })
          );
      } else if (lastPart === "" || !lastPart.includes(":")) {
        // Merge backend filters with defaults (backend takes precedence)
        const backendKeywords = new Set(
          suggestions.filters.map((f) => f.keyword)
        );
        const mergedFilters = [
          ...suggestions.filters,
          ...DEFAULT_FILTERS.filter((f) => !backendKeywords.has(f.keyword)),
        ];
        const search = lastPart.toLowerCase();
        mergedFilters
          .filter(
            (f) =>
              f.keyword.toLowerCase().startsWith(search) ||
              (search.length > 0 && f.keyword.toLowerCase().includes(search))
          )
          .forEach((f) =>
            newSuggestions.push({
              type: "filter",
              value: f.keyword,
              description: f.description,
              icon: getFilterIcon(f.keyword),
            })
          );
      }

      setFilteredSuggestions(newSuggestions);
      setSelectedIndex(0);
    },
    [suggestions]
  );

  // Scroll selected item into view
  useEffect(() => {
    if (showSuggestions && filteredSuggestions.length > 0) {
      const selectedElement = document.querySelector(
        `[data-suggestion-index="${selectedIndex}"]`
      );
      selectedElement?.scrollIntoView({ block: "nearest", behavior: "smooth" });
    }
  }, [selectedIndex, showSuggestions, filteredSuggestions.length]);

  // Handle input change
  const handleInputChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const value = e.target.value;
    updateForm({ query: value });
    updateSuggestions(value);
    setShowSuggestions(true);

    // Sync highlight layer position after state update using transform
    requestAnimationFrame(() => {
      if (inputRef.current && highlightRef.current) {
        highlightRef.current.style.transform = `translateX(-${inputRef.current.scrollLeft}px)`;
      }
    });
  };

  // Handle suggestion selection
  const selectSuggestion = (suggestion: Suggestion) => {
    const parts = query.split(/\s+/);
    parts[parts.length - 1] = suggestion.value;

    // Add space after the suggestion to continue typing
    const newQuery =
      parts.join(" ") +
      (suggestion.type === "filter" && !suggestion.value.endsWith(":")
        ? " "
        : "");
    updateForm({ query: newQuery });
    setShowSuggestions(false);
    inputRef.current?.focus();

    // Scroll to end and sync highlight layer position after state update
    requestAnimationFrame(() => {
      if (inputRef.current) {
        // Move cursor to end
        inputRef.current.setSelectionRange(newQuery.length, newQuery.length);
        // Scroll input to show the end
        inputRef.current.scrollLeft = inputRef.current.scrollWidth;
        // Sync highlight layer
        if (highlightRef.current) {
          highlightRef.current.style.transform = `translateX(-${inputRef.current.scrollLeft}px)`;
        }
      }
    });
  };

  // Handle keyboard navigation
  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (!showSuggestions || filteredSuggestions.length === 0) {
      // Allow Enter to submit form when no suggestions
      if (e.key === "Enter") return;
    }

    switch (e.key) {
      case "ArrowDown":
        e.preventDefault();
        setSelectedIndex((i) =>
          Math.min(i + 1, filteredSuggestions.length - 1)
        );
        break;
      case "ArrowUp":
        e.preventDefault();
        setSelectedIndex((i) => Math.max(i - 1, 0));
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
        // Otherwise let form submit
        break;
      case "Escape":
        e.preventDefault();
        if (showSuggestions) {
          setShowSuggestions(false);
        } else {
          // Blur the input if suggestions are already closed
          inputRef.current?.blur();
        }
        break;
    }
  };

  // Close suggestions on click outside
  useEffect(() => {
    const handleClickOutside = (e: MouseEvent) => {
      if (
        suggestionsRef.current &&
        !suggestionsRef.current.contains(e.target as Node) &&
        inputRef.current &&
        !inputRef.current.contains(e.target as Node)
      ) {
        setShowSuggestions(false);
      }
      // Close history dropdown on click outside
      if (
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
  }, []);

  // Select a history item
  const selectHistoryItem = (item: SearchHistoryItem) => {
    updateForm({ query: item.query });
    setShowHistory(false);
    setHistoryFilter("");
    inputRef.current?.focus();
  };

  // Delete a history item
  const deleteHistoryItem = (
    e: React.SyntheticEvent,
    queryToDelete: string
  ) => {
    e.stopPropagation();
    const newHistory = searchHistory.filter((h) => h.query !== queryToDelete);
    setSearchHistory(newHistory);
    saveSearchHistory(newHistory);
  };

  // Clear all history
  const clearAllHistory = () => {
    setSearchHistory([]);
    saveSearchHistory([]);
    setShowHistory(false);
  };

  // Handle history keyboard navigation
  const handleHistoryKeyDown = (e: React.KeyboardEvent) => {
    switch (e.key) {
      case "ArrowDown":
        e.preventDefault();
        setHistorySelectedIndex((i) =>
          Math.min(i + 1, filteredHistory.length - 1)
        );
        break;
      case "ArrowUp":
        e.preventDefault();
        setHistorySelectedIndex((i) => Math.max(i - 1, 0));
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

  const handleSearch = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!query.trim()) return;

    setShowSuggestions(false);
    setShowHistory(false);
    onLoading(true);

    // Save to search history
    addToSearchHistory(query);
    setSearchHistory(getSearchHistory());

    // Read values directly from DOM inputs to ensure we have the absolute latest values
    const currentFilePatterns =
      filePatternsInputRef.current?.value ?? filePatternsRef.current;
    const currentRepos = reposInputRef.current?.value ?? reposRef.current;
    const currentLimit = limitInputRef.current?.value
      ? Number(limitInputRef.current.value)
      : limitRef.current;

    // Mark as manual search to prevent URL useEffect from resetting state
    isManualSearch.current = true;

    // Update URL with search params (include all options)
    const params = new URLSearchParams();
    params.set("q", query);
    if (currentFilePatterns) params.set("files", currentFilePatterns);
    if (currentRepos) params.set("repos", currentRepos);
    if (currentLimit > 0) params.set("limit", String(currentLimit));
    router.push(`?${params.toString()}`, { scroll: false });

    try {
      // Build final query with context/repo filter
      let finalQuery = query;
      let effectiveRepos: string[] | undefined = undefined;

      if (activeContext && activeContext.repos.length > 0) {
        if (currentRepos) {
          // If user specified repos, filter context repos by that pattern
          const userPatterns = currentRepos
            .split(",")
            .map((r) => r.trim().toLowerCase());
          const filtered = activeContext.repos.filter((repo) =>
            userPatterns.some((pattern) =>
              repo.name.toLowerCase().includes(pattern)
            )
          );
          if (filtered.length > 0) {
            // Build regex OR pattern from filtered repos
            const repoPattern = filtered
              .map((r) => r.name.replace(/[.*+?^${}()|[\]\\]/g, "\\$&"))
              .join("|");
            finalQuery = `repo:(${repoPattern}) ${query}`;
          } else {
            // No match - use all context repos
            const repoPattern = activeContext.repos
              .map((r) => r.name.replace(/[.*+?^${}()|[\]\\]/g, "\\$&"))
              .join("|");
            finalQuery = `repo:(${repoPattern}) ${query}`;
          }
        } else {
          // No user filter - use all context repos
          const repoPattern = activeContext.repos
            .map((r) => r.name.replace(/[.*+?^${}()|[\]\\]/g, "\\$&"))
            .join("|");
          finalQuery = `repo:(${repoPattern}) ${query}`;
        }
      } else if (currentRepos) {
        // No context but user specified repos - build regex OR pattern for multiple repos
        const repoList = currentRepos
          .split(",")
          .map((r) => r.trim())
          .filter(Boolean);
        if (repoList.length > 1) {
          // Multiple repos - build OR pattern
          const repoPattern = repoList
            .map((r) => r.replace(/[.*+?^${}()|[\]\\]/g, "\\$&"))
            .join("|");
          finalQuery = `repo:(${repoPattern}) ${query}`;
        } else if (repoList.length === 1) {
          // Single repo - can use simple pattern
          finalQuery = `repo:${repoList[0]} ${query}`;
        }
      }

      // Build search request
      const searchRequest = {
        query: finalQuery,
        file_patterns: currentFilePatterns
          ? currentFilePatterns.split(",").map((p) => p.trim())
          : undefined,
        repos: effectiveRepos,
        limit: currentLimit > 0 ? currentLimit : undefined,
        context_lines: 3,
      };

      // Cancel any existing streaming search
      if (abortControllerRef.current) {
        abortControllerRef.current.abort();
      }
      const abortController = new AbortController();
      abortControllerRef.current = abortController;

      if (enableStreaming) {
        // Accumulate results as they stream in
        const results: SearchResult[] = [];
        let totalCount = 0;
        let truncated = false;
        let duration = "";
        let stats = { files_searched: 0, repos_searched: 0 };
        const startTime = Date.now();
        let lastUpdateTime = 0;

        // Helper to yield control to browser and allow paint
        const yieldToBrowser = () =>
          new Promise<void>((resolve) => {
            requestAnimationFrame(() => resolve());
          });

        for await (const event of api.searchStream(
          searchRequest,
          abortController.signal
        )) {
          // Check if aborted
          if (abortController.signal.aborted) break;

          if (event.type === "result" && event.result) {
            results.push(event.result);
            const now = Date.now();
            // Update UI less frequently to reduce blocking - every 100ms
            if (results.length === 1 || now - lastUpdateTime > 100) {
              lastUpdateTime = now;
              onResults({
                results: [...results],
                total_count: totalCount || results.length,
                truncated,
                duration: `${Date.now() - startTime}ms`,
                stats,
              });
              // Yield control to allow browser to paint
              await yieldToBrowser();
            }
          } else if (event.type === "done") {
            totalCount = event.total_count || results.length;
            truncated = event.truncated || totalCount > results.length;
            duration = event.duration || `${Date.now() - startTime}ms`;
            stats = event.stats || stats;
          } else if (event.type === "error") {
            console.error("Stream error:", event.error);
          }
        }

        // Final update with complete stats (if not aborted)
        if (!abortController.signal.aborted) {
          onResults({
            results,
            total_count: totalCount,
            truncated,
            duration,
            stats,
          });
        }
      } else {
        // Use batch search (non-streaming)
        const response = await api.search(searchRequest);
        onResults(response);
      }
    } catch (error) {
      // Don't report errors from aborted requests
      if (error instanceof Error && error.name === "AbortError") return;
      console.error("Search failed:", error);
      onResults(null);
    } finally {
      onLoading(false);
    }
  };

  return (
    <form onSubmit={handleSearch} className="mb-6">
      {/* Context Switcher */}
      <div className="mb-3">
        <ContextSwitcher />
      </div>

      <div className="mb-2 flex gap-2">
        <div className="relative flex-1 rounded-lg border border-gray-200 bg-white shadow-sm transition-shadow focus-within:border-transparent focus-within:ring-2 focus-within:ring-blue-500 hover:shadow-md dark:border-gray-700 dark:bg-gray-800">
          {/* History button (replaces search icon) */}
          <button
            ref={historyButtonRef}
            type="button"
            onClick={() => {
              setShowHistory(!showHistory);
              setShowSuggestions(false);
              setHistoryFilter("");
              setHistorySelectedIndex(0);
            }}
            className={`absolute left-2 top-1/2 z-10 -translate-y-1/2 rounded p-1 transition-colors ${
              showHistory
                ? "bg-blue-50 text-blue-500 dark:bg-blue-900/30"
                : "text-gray-400 hover:bg-gray-100 hover:text-gray-600 dark:hover:bg-gray-700 dark:hover:text-gray-300"
            }`}
            title="Search history"
          >
            <History className="h-4 w-4" />
          </button>

          {/* Search history dropdown */}
          {showHistory && (
            <div
              ref={historyRef}
              className="absolute left-0 right-0 top-full z-50 mt-1 overflow-hidden rounded-xl border border-gray-200 bg-white shadow-lg dark:border-gray-700 dark:bg-gray-800"
            >
              {/* Search filter */}
              <div className="border-b border-gray-100 p-2 dark:border-gray-700">
                <div className="relative">
                  <Search className="absolute left-2.5 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-gray-400" />
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
                    className="w-full rounded-lg border border-gray-200 bg-gray-50 py-1.5 pl-8 pr-3 text-sm focus:outline-none focus:ring-1 focus:ring-blue-500 dark:border-gray-600 dark:bg-gray-700/50"
                  />{" "}
                </div>
              </div>

              {filteredHistory.length === 0 ? (
                <div className="p-4 text-center text-sm text-gray-500 dark:text-gray-400">
                  {searchHistory.length === 0
                    ? "No search history yet"
                    : "No matching searches"}
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
                          if (e.key === "Enter" || e.key === " ") {
                            selectHistoryItem(item);
                          }
                        }}
                        className={`group flex w-full items-center gap-3 px-4 py-2.5 text-left transition-colors ${
                          index === historySelectedIndex
                            ? "bg-blue-50 dark:bg-blue-900/30"
                            : "hover:bg-gray-50 dark:hover:bg-gray-700/50"
                        }`}
                      >
                        <Clock className="h-4 w-4 flex-shrink-0 text-gray-400" />
                        <span className="flex-1 truncate font-mono text-sm text-gray-700 dark:text-gray-300">
                          {item.query}
                        </span>
                        {(item.isRegex || item.caseSensitive) && (
                          <span className="flex flex-shrink-0 gap-1">
                            {item.isRegex && (
                              <span className="rounded bg-purple-100 px-1.5 py-0.5 text-xs text-purple-600 dark:bg-purple-900/30 dark:text-purple-400">
                                regex
                              </span>
                            )}
                            {item.caseSensitive && (
                              <span className="rounded bg-orange-100 px-1.5 py-0.5 text-xs text-orange-600 dark:bg-orange-900/30 dark:text-orange-400">
                                case
                              </span>
                            )}
                          </span>
                        )}
                        <span
                          role="button"
                          tabIndex={-1}
                          onClick={(e) => {
                            e.stopPropagation();
                            deleteHistoryItem(e, item.query);
                          }}
                          onKeyDown={(e) => {
                            if (e.key === "Enter" || e.key === " ") {
                              e.stopPropagation();
                              deleteHistoryItem(e, item.query);
                            }
                          }}
                          className="flex-shrink-0 cursor-pointer p-1 text-gray-400 opacity-0 transition-opacity hover:text-red-500 group-hover:opacity-100"
                          title="Remove from history"
                        >
                          <X className="h-3.5 w-3.5" />
                        </span>
                      </button>
                    ))}
                  </div>
                  <div className="flex items-center justify-between border-t border-gray-100 p-2 dark:border-gray-700">
                    <span className="text-xs text-gray-400">
                      {filteredHistory.length} of {searchHistory.length}{" "}
                      searches
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
            value={query}
            onChange={handleInputChange}
            onFocus={() => {
              updateSuggestions(query);
              setShowSuggestions(true);
              setShowHistory(false);
            }}
            onKeyDown={handleKeyDown}
            onScroll={(e) => {
              // Sync highlight layer position using transform
              if (highlightRef.current) {
                highlightRef.current.style.transform = `translateX(-${e.currentTarget.scrollLeft}px)`;
              }
            }}
            className="w-full rounded-lg bg-transparent py-2 pl-9 pr-3 font-mono text-sm text-transparent caret-gray-900 outline-none dark:caret-white"
            autoComplete="off"
          />

          {/* Syntax highlighted overlay layer - clips content to prevent overlap with history button */}
          <div className="pointer-events-none absolute inset-y-0 left-9 right-3 overflow-hidden rounded-lg">
            <div
              ref={highlightRef}
              className="flex h-full items-center whitespace-pre py-2 font-mono text-sm"
              style={{ width: "max-content", minWidth: "100%" }}
            >
              {query ? (
                <HighlightedQuery query={query} />
              ) : (
                <span className="text-gray-400">
                  <span className="sm:hidden">Search code...</span>
                  <span className="hidden sm:inline">
                    Search code... (type repo:, lang:, file: for filters)
                  </span>
                </span>
              )}
            </div>
          </div>

          {/* Suggestions dropdown */}
          {showSuggestions && filteredSuggestions.length > 0 && (
            <div
              ref={suggestionsRef}
              className="absolute left-0 right-0 top-full z-50 mt-1 overflow-hidden rounded-xl border border-gray-200 bg-white shadow-lg dark:border-gray-700 dark:bg-gray-800"
            >
              <div className="border-b border-gray-100 p-2 dark:border-gray-700">
                <span className="text-xs font-medium text-gray-500 dark:text-gray-400">
                  Narrow your search
                </span>
              </div>
              <div className="max-h-64 overflow-y-auto">
                {filteredSuggestions.map((suggestion, index) => (
                  <button
                    key={`${suggestion.type}-${suggestion.value}`}
                    type="button"
                    data-suggestion-index={index}
                    onClick={() => selectSuggestion(suggestion)}
                    className={`flex w-full items-center gap-3 px-4 py-2.5 text-left transition-colors ${
                      index === selectedIndex
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
              <div className="flex items-center gap-4 border-t border-gray-100 p-2 text-xs text-gray-400 dark:border-gray-700">
                <span>
                  <kbd className="rounded bg-gray-100 px-1.5 py-0.5 dark:bg-gray-700">
                    ↑↓
                  </kbd>{" "}
                  navigate
                </span>
                <span>
                  <kbd className="rounded bg-gray-100 px-1.5 py-0.5 dark:bg-gray-700">
                    Tab
                  </kbd>{" "}
                  select
                </span>
                <span>
                  <kbd className="rounded bg-gray-100 px-1.5 py-0.5 dark:bg-gray-700">
                    Esc
                  </kbd>{" "}
                  close
                </span>
              </div>
            </div>
          )}
        </div>
        <button
          type="submit"
          className="hidden items-center gap-2 rounded-lg border border-blue-500 px-4 py-2 text-sm font-medium text-blue-600 transition-colors hover:bg-blue-50 dark:border-blue-400 dark:text-blue-400 dark:hover:bg-blue-900/30 sm:flex"
        >
          <Search className="h-4 w-4" />
          Search
        </button>
      </div>

      <div className="flex items-center justify-between gap-3 pl-1 text-sm sm:gap-4">
        <div className="flex flex-wrap items-center gap-3 sm:gap-4">
          <button
            type="button"
            onClick={() => updateForm({ showOptions: !showOptions })}
            className="flex items-center gap-1 text-blue-600 transition-colors hover:text-blue-800 dark:text-blue-400 dark:hover:text-blue-300"
          >
            <span className="sm:hidden">{showOptions ? "Less" : "More"}</span>
            <span className="hidden sm:inline">
              {showOptions ? "Hide options" : "More options"}
            </span>
            {showOptions ? (
              <ChevronUp className="h-4 w-4" />
            ) : (
              <ChevronDown className="h-4 w-4" />
            )}
          </button>
        </div>
        <button
          type="submit"
          className="flex items-center gap-1.5 rounded-lg border border-blue-500 px-3 py-1.5 text-sm font-medium text-blue-600 transition-colors hover:bg-blue-50 dark:border-blue-400 dark:text-blue-400 dark:hover:bg-blue-900/30 sm:hidden"
        >
          <Search className="h-4 w-4" />
          Search
        </button>
      </div>

      {showOptions && (
        <div className="mt-3 grid grid-cols-1 gap-4 rounded-lg border border-gray-100 bg-gray-50 p-4 dark:border-gray-700 dark:bg-gray-800/50 sm:grid-cols-2 lg:grid-cols-3">
          <div>
            <label
              htmlFor="file-patterns"
              className="mb-1 block text-xs font-medium text-gray-700 dark:text-gray-300"
            >
              File patterns
            </label>
            <input
              ref={filePatternsInputRef}
              id="file-patterns"
              type="text"
              value={filePatterns}
              onChange={(e) => {
                const val = e.target.value;
                updateForm({ filePatterns: val });
                filePatternsRef.current = val;
              }}
              placeholder="*.go, *.ts"
              className="w-full rounded-lg border border-gray-200 bg-white px-3 py-1.5 text-sm focus:border-blue-500 focus:outline-none dark:border-gray-600 dark:bg-gray-700 dark:focus:border-blue-500"
            />
          </div>
          <div>
            <label
              htmlFor="repos"
              className="mb-1 block text-xs font-medium text-gray-700 dark:text-gray-300"
            >
              Repositories
            </label>
            <input
              ref={reposInputRef}
              id="repos"
              type="text"
              value={repos}
              onChange={(e) => {
                const val = e.target.value;
                updateForm({ repos: val });
                reposRef.current = val;
              }}
              placeholder="org/repo1, org/repo2"
              className="w-full rounded-lg border border-gray-200 bg-white px-3 py-1.5 text-sm focus:border-blue-500 focus:outline-none dark:border-gray-600 dark:bg-gray-700 dark:focus:border-blue-500"
            />
          </div>
          <div>
            <label
              htmlFor="search-limit"
              className="mb-1 block text-xs font-medium text-gray-700 dark:text-gray-300"
            >
              Limit (0 = unlimited)
            </label>
            <input
              ref={limitInputRef}
              id="search-limit"
              type="number"
              value={limit || ""}
              onChange={(e) => {
                const val = e.target.value ? Number(e.target.value) : 0;
                updateForm({ limit: val });
                limitRef.current = val;
              }}
              min={0}
              max={10000}
              placeholder="Unlimited"
              className="w-full rounded-lg border border-gray-200 bg-white px-3 py-1.5 text-sm focus:border-blue-500 focus:outline-none dark:border-gray-600 dark:bg-gray-700 dark:focus:border-blue-500"
            />
          </div>
        </div>
      )}
    </form>
  );
}

export function SearchResults({
  response,
  isStreaming = false,
}: {
  response: SearchResponse | null;
  isStreaming?: boolean;
}) {
  const [hideFileNavigator, setHideFileNavigator] = useState(false);

  // Load UI settings to check if file navigator should be hidden
  useEffect(() => {
    api
      .getUISettings()
      .then((settings) => {
        setHideFileNavigator(settings.hide_file_navigator);
      })
      .catch(() => {});
  }, []);

  if (!response) return null;

  const { results, total_count, truncated, duration } = response;

  // Group results by repo and file
  const groupedResults = results.reduce(
    (acc, result) => {
      const key = `${result.repo}:${result.file}`;
      if (!acc[key]) {
        acc[key] = {
          repo: result.repo,
          file: result.file,
          language: result.language,
          matches: [],
        };
      }
      acc[key].matches.push(result);
      return acc;
    },
    {} as Record<
      string,
      { repo: string; file: string; language: string; matches: SearchResult[] }
    >
  );

  const groupedList = Object.values(groupedResults);

  // Count unique repos with matches
  const uniqueReposWithMatches = new Set(groupedList.map((g) => g.repo)).size;

  return (
    <div>
      <div className="mb-5 flex flex-col gap-2 px-1 sm:flex-row sm:items-center sm:justify-between">
        <div className="flex items-center gap-2 text-sm text-gray-600 dark:text-gray-400">
          {isStreaming && (
            <Loader2 className="h-4 w-4 animate-spin text-blue-600" />
          )}
          <span className="font-semibold text-gray-900 dark:text-white">
            {total_count?.toLocaleString()}
          </span>
          <span>results{isStreaming ? " (streaming)" : ""} in</span>
          <span className="font-mono text-blue-600 dark:text-blue-400">
            {duration}
          </span>
        </div>
        <div className="flex items-center gap-4 text-sm text-gray-500 dark:text-gray-400">
          <span className="flex items-center gap-1.5">
            <FileCode className="h-4 w-4" />
            {groupedList.length} files
          </span>
          <span className="flex items-center gap-1.5">
            <FolderGit2 className="h-4 w-4" />
            {uniqueReposWithMatches} repos
          </span>
        </div>
      </div>

      {/* Truncation warning */}
      {truncated && (
        <div className="mb-4 flex items-center gap-2 rounded-lg border border-amber-200 bg-amber-50 p-3 text-sm text-amber-700 dark:border-amber-800 dark:bg-amber-900/20 dark:text-amber-400">
          <AlertTriangle className="h-4 w-4 flex-shrink-0" />
          <span>
            Results truncated. Showing {results.length.toLocaleString()} of{" "}
            {total_count.toLocaleString()} matches. Use filters or set a limit
            to narrow your search.
          </span>
        </div>
      )}

      {results.length === 0 ? (
        <div className="rounded-xl bg-gray-50 py-12 text-center dark:bg-gray-800/50">
          <p className="text-gray-500 dark:text-gray-400">No results found</p>
        </div>
      ) : (
        <div className="space-y-3">
          {groupedList.map((group, index) => (
            <FileResultCard
              key={`${group.repo}:${group.file}-${index}`}
              group={group}
              hideFileNavigator={hideFileNavigator}
            />
          ))}
        </div>
      )}
    </div>
  );
}

interface FileGroup {
  repo: string;
  file: string;
  language: string;
  matches: SearchResult[];
}

function FileResultCard({
  group,
  hideFileNavigator,
}: {
  group: FileGroup;
  hideFileNavigator?: boolean;
}) {
  const [collapsed, setCollapsed] = useState(false);
  const [repoId, setRepoId] = useState<number | null>(null);

  // Sort matches by line number
  const sortedMatches = [...group.matches].sort((a, b) => a.line - b.line);

  // Try to lookup repo ID for browse link (do this once when group changes)
  useEffect(() => {
    api
      .lookupRepoByName(group.repo)
      .then((repo) => setRepoId(repo.id))
      .catch(() => setRepoId(null)); // Repo not found in DB
  }, [group.repo]);

  // Build the URL to open the file in the repo at the specific line
  const getFileUrl = (line: number) => {
    const repo = group.repo;
    const file = group.file;

    // Check for common git hosts and build appropriate URLs
    if (repo.includes("github.com")) {
      return `https://${repo}/blob/HEAD/${file}#L${line}`;
    } else if (repo.includes("gitlab")) {
      return `https://${repo}/-/blob/HEAD/${file}#L${line}`;
    } else if (repo.includes("bitbucket")) {
      return `https://${repo}/src/HEAD/${file}#lines-${line}`;
    }
    // Default: assume GitLab-style URL
    return `https://${repo}/-/blob/HEAD/${file}#L${line}`;
  };

  // Build internal browse URL
  const getBrowseUrl = (line?: number) => {
    if (!repoId) return null;
    return buildBrowseUrl(repoId, group.file, { line });
  };

  // Highlight multiple match ranges in the content
  const highlightContent = (
    content: string,
    ranges: { start: number; end: number }[]
  ) => {
    if (!ranges.length) return content;

    // Sort ranges by start position and merge overlapping ones
    const sorted = [...ranges].sort((a, b) => a.start - b.start);
    const merged: { start: number; end: number }[] = [sorted[0]];
    for (let i = 1; i < sorted.length; i++) {
      const last = merged[merged.length - 1];
      if (sorted[i].start <= last.end) {
        last.end = Math.max(last.end, sorted[i].end);
      } else {
        merged.push(sorted[i]);
      }
    }

    const parts: React.ReactNode[] = [];
    let cursor = 0;
    for (let i = 0; i < merged.length; i++) {
      const { start, end } = merged[i];
      if (cursor < start) {
        parts.push(content.slice(cursor, start));
      }
      parts.push(
        <mark
          key={i}
          className="rounded bg-yellow-300 px-0.5 text-yellow-900 dark:bg-yellow-500/40 dark:text-yellow-100"
        >
          {content.slice(start, end)}
        </mark>
      );
      cursor = end;
    }
    if (cursor < content.length) {
      parts.push(content.slice(cursor));
    }
    return <>{parts}</>;
  };

  const firstLine = sortedMatches[0]?.line || 1;
  const browseUrl = getBrowseUrl(firstLine);

  return (
    <div className="overflow-hidden rounded-xl border border-gray-200 bg-white shadow-sm transition-shadow hover:shadow-md dark:border-gray-700 dark:bg-gray-800">
      {/* Header */}
      <div className="flex flex-wrap items-center gap-2 bg-gray-50 px-3 py-3 dark:bg-gray-800/80 sm:gap-3 sm:px-4">
        <button
          type="button"
          onClick={() => setCollapsed(!collapsed)}
          className="text-gray-400 hover:text-gray-600 dark:hover:text-gray-300"
          title={collapsed ? "Expand" : "Collapse"}
        >
          {collapsed ? (
            <ChevronRight className="h-4 w-4" />
          ) : (
            <ChevronDown className="h-4 w-4" />
          )}
        </button>
        <FolderGit2 className="hidden h-4 w-4 text-blue-500 sm:block" />
        {browseUrl && !hideFileNavigator ? (
          <Link
            href={browseUrl}
            className="max-w-[120px] truncate text-xs font-medium text-blue-600 hover:underline dark:text-blue-400 sm:max-w-none sm:text-sm"
            onClick={(e) => e.stopPropagation()}
          >
            {group.repo}
          </Link>
        ) : (
          <a
            href={getFileUrl(firstLine)}
            target="_blank"
            rel="noopener noreferrer"
            className="max-w-[120px] truncate text-xs font-medium text-blue-600 hover:underline dark:text-blue-400 sm:max-w-none sm:text-sm"
            onClick={(e) => e.stopPropagation()}
          >
            {group.repo}
          </a>
        )}
        <span className="hidden text-gray-300 dark:text-gray-600 sm:inline">
          /
        </span>
        <FileCode className="hidden h-4 w-4 text-gray-400 sm:block" />
        {browseUrl && !hideFileNavigator ? (
          <Link
            href={browseUrl}
            className="flex-1 truncate text-xs text-gray-700 hover:text-blue-600 hover:underline dark:text-gray-300 dark:hover:text-blue-400 sm:text-sm"
            onClick={(e) => e.stopPropagation()}
          >
            {group.file}
          </Link>
        ) : (
          <a
            href={getFileUrl(firstLine)}
            target="_blank"
            rel="noopener noreferrer"
            className="flex-1 truncate text-xs text-gray-700 hover:text-blue-600 hover:underline dark:text-gray-300 dark:hover:text-blue-400 sm:text-sm"
            onClick={(e) => e.stopPropagation()}
          >
            {group.file}
          </a>
        )}
        <span className="whitespace-nowrap rounded-full bg-blue-100 px-2 py-0.5 text-xs font-medium text-blue-700 dark:bg-blue-900/30 dark:text-blue-300">
          {sortedMatches.length}{" "}
          {sortedMatches.length === 1 ? "match" : "matches"}
        </span>
        {group.language && (
          <span className="hidden rounded-full bg-gray-200 px-2 py-0.5 text-xs font-medium dark:bg-gray-700 sm:inline">
            {group.language}
          </span>
        )}
        {/* Browse button */}
        {browseUrl && !hideFileNavigator && (
          <Link
            href={browseUrl}
            className="hidden text-gray-400 hover:text-blue-600 dark:hover:text-blue-400 sm:block"
            onClick={(e) => e.stopPropagation()}
            title="Open in browser"
          >
            <Eye className="h-4 w-4" />
          </Link>
        )}
        {/* External link */}
        <a
          href={getFileUrl(firstLine)}
          target="_blank"
          rel="noopener noreferrer"
          className="hidden text-gray-400 hover:text-blue-600 dark:hover:text-blue-400 sm:block"
          onClick={(e) => e.stopPropagation()}
          title="Open in code host"
        >
          <ExternalLink className="h-4 w-4" />
        </a>
      </div>

      {/* Code content with line numbers */}
      {!collapsed && (
        <div className="overflow-x-auto border-t border-gray-100 dark:border-gray-700/50">
          <table className="w-full font-mono text-sm">
            <tbody>
              {(() => {
                // Group consecutive same-line matches into single entries
                const lineGroups: { line: number; matches: SearchResult[] }[] =
                  [];
                for (const match of sortedMatches) {
                  const last = lineGroups[lineGroups.length - 1];
                  if (last && last.line === match.line) {
                    last.matches.push(match);
                  } else {
                    lineGroups.push({ line: match.line, matches: [match] });
                  }
                }

                return lineGroups.map((lineGroup, groupIndex) => {
                  // Use the first match for context lines (all same-line matches share context)
                  const result = lineGroup.matches[0];
                  const beforeLines = result.context?.before || [];
                  const afterLines = result.context?.after || [];
                  const matchLine = result.line;
                  const beforeStartLine = matchLine - beforeLines.length;
                  const afterStartLine = matchLine + 1;

                  // Check next group to cap after-context before the next match line
                  const nextGroup =
                    groupIndex < lineGroups.length - 1
                      ? lineGroups[groupIndex + 1]
                      : null;
                  const afterLinesToShow = nextGroup
                    ? Math.min(
                        afterLines.length,
                        Math.max(0, nextGroup.line - matchLine - 1)
                      )
                    : afterLines.length;
                  const filteredAfterLines = afterLines.slice(
                    0,
                    afterLinesToShow
                  );

                  // Check previous group to avoid duplicating context lines
                  const prevGroup =
                    groupIndex > 0 ? lineGroups[groupIndex - 1] : null;
                  const prevResult = prevGroup ? prevGroup.matches[0] : null;
                  // Calculate how many after-context lines the previous group actually displayed
                  const prevAfterLength =
                    prevResult?.context?.after?.length || 0;
                  const prevAfterShown = prevGroup
                    ? Math.min(
                        prevAfterLength,
                        Math.max(0, matchLine - prevGroup.line - 1)
                      )
                    : 0;
                  const prevDisplayedEndLine = prevGroup
                    ? prevGroup.line + prevAfterShown
                    : 0;

                  // Calculate how many before lines to skip (already shown by previous group)
                  const beforeLinesToSkip = prevGroup
                    ? Math.max(0, prevDisplayedEndLine - beforeStartLine + 1)
                    : 0;
                  const filteredBeforeLines =
                    beforeLines.slice(beforeLinesToSkip);
                  const filteredBeforeStartLine =
                    beforeStartLine + beforeLinesToSkip;

                  const needsSeparator =
                    prevResult &&
                    filteredBeforeStartLine > prevDisplayedEndLine + 1;

                  const getLineLink = (line: number) => {
                    if (repoId && !hideFileNavigator) {
                      return buildBrowseUrl(repoId, group.file, { line });
                    }
                    return getFileUrl(line);
                  };

                  // Collect all highlight ranges for this line
                  const ranges = lineGroup.matches.map((m) => ({
                    start: m.match_start,
                    end: m.match_end,
                  }));

                  return (
                    <React.Fragment key={`match-${groupIndex}`}>
                      {needsSeparator && (
                        <tr>
                          <td colSpan={2} className="py-1">
                            <div className="border-t border-dashed border-gray-200 dark:border-gray-700" />
                          </td>
                        </tr>
                      )}

                      {filteredBeforeLines.map((line, i) => (
                        <tr
                          key={`before-${groupIndex}-${i}`}
                          className="text-gray-400 dark:text-gray-500"
                        >
                          <td className="w-12 select-none border-r border-gray-100 py-0.5 pl-4 pr-3 text-right text-xs dark:border-gray-700">
                            {filteredBeforeStartLine + i}
                          </td>
                          <td className="whitespace-pre py-0.5 pl-3 pr-4">
                            {line}
                          </td>
                        </tr>
                      ))}

                      <tr className="bg-yellow-50 dark:bg-yellow-900/20">
                        <td className="w-12 select-none border-r border-yellow-200 py-1 pl-4 pr-3 text-right text-xs font-medium text-yellow-700 dark:border-yellow-800 dark:text-yellow-400">
                          {repoId ? (
                            <Link
                              href={getLineLink(matchLine)}
                              className="hover:underline"
                            >
                              {matchLine}
                            </Link>
                          ) : (
                            <a
                              href={getLineLink(matchLine)}
                              target="_blank"
                              rel="noopener noreferrer"
                              className="hover:underline"
                            >
                              {matchLine}
                            </a>
                          )}
                        </td>
                        <td className="whitespace-pre py-1 pl-3 pr-4 text-gray-800 dark:text-gray-200">
                          {highlightContent(result.content, ranges)}
                        </td>
                      </tr>

                      {filteredAfterLines.map((line, i) => (
                        <tr
                          key={`after-${groupIndex}-${i}`}
                          className="text-gray-400 dark:text-gray-500"
                        >
                          <td className="w-12 select-none border-r border-gray-100 py-0.5 pl-4 pr-3 text-right text-xs dark:border-gray-700">
                            {afterStartLine + i}
                          </td>
                          <td className="whitespace-pre py-0.5 pl-3 pr-4">
                            {line}
                          </td>
                        </tr>
                      ))}
                    </React.Fragment>
                  );
                });
              })()}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}

// Component to render syntax-highlighted query
function HighlightedQuery({ query }: { query: string }) {
  // Parse the query and highlight filter prefixes and regex patterns
  const parts: React.ReactNode[] = [];

  // Using inline styles with exact Tailwind color values to ensure they work
  // cyan-500: #06b6d4, orange-500: #f97316, red-500: #ef4444, purple-500: #a855f7
  const CYAN_500 = "#06b6d4";
  const ORANGE_500 = "#f97316";
  const RED_500 = "#ef4444";
  const PURPLE_500 = "#a855f7";

  // Combined regex to match:
  // 1. Filter patterns like repo:value, lang:value, etc.
  // 2. Regex patterns like /pattern/
  const tokenRegex = new RegExp(
    "(/[^/\\s]+/)|((repo:|file:|lang:|content:|case:|sym:|symbol:|branch:|rev:|path:|type:|regex:|-repo:|-file:|-lang:|-content:|-path:|-sym:|-branch:)([^\\s]*))",
    "g"
  );

  let lastIndex = 0;
  let match;

  while ((match = tokenRegex.exec(query)) !== null) {
    // Add text before match
    if (match.index > lastIndex) {
      parts.push(
        <span
          key={`text-${lastIndex}`}
          className="text-gray-900 dark:text-white"
        >
          {query.slice(lastIndex, match.index)}
        </span>
      );
    }

    if (match[1]) {
      // This is a regex pattern /pattern/
      parts.push(
        <span key={`regex-${match.index}`} style={{ color: PURPLE_500 }}>
          {match[1]}
        </span>
      );
    } else if (match[2]) {
      // This is a filter pattern
      const prefix = match[3];
      const value = match[4];
      const isNegation = prefix.startsWith("-");

      // Determine the color - matching "How to search" section
      let color = CYAN_500; // default for most filters
      if (isNegation) {
        color = RED_500; // negation
      } else if (prefix === "case:") {
        color = ORANGE_500; // case: is orange
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

  // Add remaining text
  if (lastIndex < query.length) {
    parts.push(
      <span key={`text-${lastIndex}`} className="text-gray-900 dark:text-white">
        {query.slice(lastIndex)}
      </span>
    );
  }

  // If no parts (no filters found), show entire query
  if (parts.length === 0 && query) {
    return <span className="text-gray-900 dark:text-white">{query}</span>;
  }

  return <>{parts}</>;
}
