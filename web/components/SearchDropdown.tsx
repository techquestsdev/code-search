"use client";

import React, { useState, useEffect, useRef, useCallback } from "react";
import { useRouter } from "next/navigation";
import { api, SearchResult, SearchResponse, buildBrowseUrl } from "@/lib/api";
import { useActiveContext } from "@/hooks/useContexts";
import { Context } from "@/lib/contexts";
import { ExternalLink, FileCode, FolderGit2, Loader2 } from "lucide-react";
import Link from "next/link";
import { SearchInput } from "./SearchInput";

interface SearchDropdownProps {
  repoId?: number;
  repoName?: string;
  className?: string;
  placeholder?: string;
}

interface SearchResultItemProps {
  group: {
    repo: string;
    file: string;
    language?: string;
    matches: SearchResult[];
  };
  index: number;
  selectedIndex: number;
  onSelect: (group: SearchResultItemProps["group"]) => void;
  highlightContent: (
    content: string,
    start: number,
    end: number
  ) => React.ReactNode;
}

function SearchResultItem({
  group,
  index,
  selectedIndex,
  onSelect,
  highlightContent,
}: SearchResultItemProps) {
  const isSelected = index === selectedIndex;

  return (
    <button
      role="option"
      aria-selected={isSelected}
      data-result-index={index}
      onClick={() => onSelect(group)}
      className={`flex w-full flex-col gap-1 border-b border-gray-50 px-3 py-2 text-left transition-colors last:border-0 dark:border-gray-700/50 ${
        isSelected
          ? "bg-blue-50 dark:bg-blue-900/20"
          : "hover:bg-gray-50 dark:hover:bg-gray-700/30"
      }`}
    >
      <div className="flex items-center gap-2">
        <FileCode className="h-3.5 w-3.5 flex-shrink-0 text-gray-400" />
        <span className="flex-1 truncate text-sm font-medium text-gray-700 dark:text-gray-200">
          {group.file.split("/").pop()}
        </span>
        {group.language && (
          <span className="rounded bg-gray-100 px-1.5 py-0.5 text-[10px] font-bold uppercase tracking-wider text-gray-500 dark:bg-gray-700 dark:text-gray-400">
            {group.language}
          </span>
        )}
      </div>
      <div className="ml-5 flex items-center gap-1.5 truncate text-[10px] text-gray-400">
        <FolderGit2 className="h-3 w-3 flex-shrink-0" />
        <span>{group.repo}</span>
        <span>•</span>
        <span className="truncate">{group.file}</span>
      </div>
      {group.matches[0] && (
        <div className="ml-5 mt-1 truncate rounded border border-gray-100 bg-gray-50 p-1 font-mono text-xs text-gray-600 dark:border-gray-800 dark:bg-gray-900/50 dark:text-gray-400">
          {highlightContent(
            group.matches[0].content,
            group.matches[0].match_start,
            group.matches[0].match_end
          )}
        </div>
      )}
    </button>
  );
}

interface SearchDropdownResultsProps {
  results: SearchResponse;
  groupedList: {
    repo: string;
    file: string;
    language?: string;
    matches: SearchResult[];
  }[];
  selectedIndex: number;
  searchInRepo: boolean;
  repoName?: string;
  query: string;
  activeContext: Context | null;
  onSelect: (group: SearchResultItemProps["group"]) => void;
  highlightContent: (
    content: string,
    start: number,
    end: number
  ) => React.ReactNode;
}

function SearchDropdownResults({
  results,
  groupedList,
  selectedIndex,
  searchInRepo,
  repoName,
  query,
  activeContext,
  onSelect,
  highlightContent,
}: SearchDropdownResultsProps) {
  const fullSearchUrl = `/?q=${encodeURIComponent(searchInRepo && repoName ? `repo:${repoName} ${query}` : query)}`;

  return (
    <>
      <div className="flex items-center justify-between border-b border-gray-100 p-2 dark:border-gray-700">
        <div className="flex items-center gap-2">
          <span className="text-xs text-gray-500 dark:text-gray-400">
            {results.total_count} results in {results.duration}
          </span>
          {activeContext && !searchInRepo && (
            <span
              className="flex items-center gap-1 rounded-full px-1.5 py-0.5 text-xs"
              style={{
                backgroundColor: `${activeContext.color}20`,
                color: activeContext.color,
              }}
            >
              <div
                className="h-1.5 w-1.5 rounded-full"
                style={{ backgroundColor: activeContext.color }}
              />
              {activeContext.name}
            </span>
          )}
        </div>
        <Link
          href={fullSearchUrl}
          className="flex items-center gap-1 text-xs text-blue-600 hover:underline dark:text-blue-400"
        >
          View all
          <ExternalLink className="h-3 w-3" />
        </Link>
      </div>
      <div className="max-h-80 overflow-y-auto" role="listbox">
        {groupedList.slice(0, 10).map((group, index) => (
          <SearchResultItem
            key={`${group.repo}:${group.file}`}
            group={group}
            index={index}
            selectedIndex={selectedIndex}
            onSelect={onSelect}
            highlightContent={highlightContent}
          />
        ))}
      </div>
      {groupedList.length > 10 && (
        <div className="border-t border-gray-100 p-2 text-center dark:border-gray-700">
          <Link
            href={fullSearchUrl}
            className="text-xs text-blue-600 hover:underline dark:text-blue-400"
          >
            Show {groupedList.length - 10} more results...
          </Link>
        </div>
      )}
    </>
  );
}

function SearchDropdownFooter() {
  return (
    <div className="flex items-center gap-4 border-t border-gray-100 p-2 text-xs text-gray-400 dark:border-gray-700">
      <span>
        <kbd className="rounded bg-gray-100 px-1.5 py-0.5 dark:bg-gray-700">
          ↑↓
        </kbd>{" "}
        navigate
      </span>
      <span>
        <kbd className="rounded bg-gray-100 px-1.5 py-0.5 dark:bg-gray-700">
          Enter
        </kbd>{" "}
        open
      </span>
      <span>
        <kbd className="rounded bg-gray-100 px-1.5 py-0.5 dark:bg-gray-700">
          Esc
        </kbd>{" "}
        close
      </span>
    </div>
  );
}

export function SearchDropdown({
  repoId,
  repoName,
  className = "",
  placeholder,
}: SearchDropdownProps) {
  const router = useRouter();
  const { activeContext, getRepoFilter } = useActiveContext();

  const [state, setState] = useState({
    query: "",
    searchInRepo: !!repoId,
    results: null as SearchResponse | null,
    loading: false,
    showDropdown: false,
    selectedIndex: 0,
  });

  const { query, searchInRepo, results, loading, showDropdown, selectedIndex } =
    state;

  const setSearchInRepo = (val: boolean) =>
    setState((prev) => ({ ...prev, searchInRepo: val }));
  const setShowDropdown = (val: boolean) =>
    setState((prev) => ({ ...prev, showDropdown: val }));

  const inputRef = useRef<HTMLInputElement>(null);
  const dropdownRef = useRef<HTMLDivElement>(null);
  const searchTimeoutRef = useRef<NodeJS.Timeout | null>(null);

  // Debounced search
  const performSearch = useCallback(
    async (searchQuery: string) => {
      if (!searchQuery.trim()) {
        setState((prev) => ({ ...prev, results: null }));
        return;
      }

      setState((prev) => ({ ...prev, loading: true }));
      try {
        // Build query with repo filter
        let finalQuery = searchQuery;

        // If searching in current repo, add repo filter
        if (state.searchInRepo && repoName) {
          finalQuery = `repo:${repoName} ${searchQuery}`;
        }
        // If we have an active context (and not searching in specific repo), add context filter
        else if (activeContext && !state.searchInRepo) {
          const repoFilter = getRepoFilter();
          if (repoFilter) {
            finalQuery = `repo:${repoFilter} ${searchQuery}`;
          }
        }

        const response = await api.search({
          query: finalQuery,
          limit: 20,
          context_lines: 1,
        });
        setState((prev) => ({
          ...prev,
          results: response,
          selectedIndex: 0,
          loading: false,
        }));
      } catch (error) {
        console.error("Search failed:", error);
        setState((prev) => ({ ...prev, results: null, loading: false }));
      }
    },
    [state.searchInRepo, repoName, activeContext, getRepoFilter]
  );

  // Handle input change with debounce
  const handleInputChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const value = e.target.value;
    setState((prev) => ({ ...prev, query: value, showDropdown: true }));

    // Clear previous timeout
    if (searchTimeoutRef.current) {
      clearTimeout(searchTimeoutRef.current);
    }

    // Debounce search by 300ms
    searchTimeoutRef.current = setTimeout(() => {
      performSearch(value);
    }, 300);
  };

  // Group results by repo and file
  const groupedResults = results?.results.reduce(
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

  const groupedList = groupedResults ? Object.values(groupedResults) : [];

  // Handle keyboard navigation
  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (!showDropdown || groupedList.length === 0) {
      if (e.key === "Enter" && query.trim()) {
        // Navigate to full search page
        e.preventDefault();
        const searchQuery =
          searchInRepo && repoName ? `repo:${repoName} ${query}` : query;
        router.push(`/?q=${encodeURIComponent(searchQuery)}`);
        setShowDropdown(false);
      }
      return;
    }

    switch (e.key) {
      case "ArrowDown":
        e.preventDefault();
        setState((prev) => ({
          ...prev,
          selectedIndex: Math.min(
            prev.selectedIndex + 1,
            groupedList.length - 1
          ),
        }));
        break;
      case "ArrowUp":
        e.preventDefault();
        setState((prev) => ({
          ...prev,
          selectedIndex: Math.max(prev.selectedIndex - 1, 0),
        }));
        break;
      case "Enter":
        e.preventDefault();
        if (groupedList[selectedIndex]) {
          const group = groupedList[selectedIndex];
          navigateToResult(group);
        }
        break;
      case "Escape":
        e.preventDefault();
        setShowDropdown(false);
        inputRef.current?.blur();
        break;
    }
  };

  // Navigate to a result
  const navigateToResult = async (group: {
    repo: string;
    file: string;
    language?: string;
    matches: SearchResult[];
  }) => {
    try {
      const lookupResult = await api.lookupRepoByName(group.repo);
      if (lookupResult) {
        const firstLine = group.matches[0]?.line || 1;
        router.push(
          buildBrowseUrl(lookupResult.id, group.file, { line: firstLine })
        );
        setState((prev) => ({ ...prev, showDropdown: false, query: "" }));
      }
    } catch (err) {
      console.error("Failed to navigate:", err);
    }
  };

  // Close dropdown on click outside
  useEffect(() => {
    if (!showDropdown) return;
    const handleClickOutside = (e: MouseEvent) => {
      if (
        dropdownRef.current &&
        !dropdownRef.current.contains(e.target as Node) &&
        inputRef.current &&
        !inputRef.current.contains(e.target as Node)
      ) {
        setShowDropdown(false);
      }
    };
    document.addEventListener("mousedown", handleClickOutside);
    return () => document.removeEventListener("mousedown", handleClickOutside);
  }, [showDropdown]);

  // Scroll selected into view
  useEffect(() => {
    if (showDropdown && groupedList.length > 0) {
      const element = document.querySelector(
        `[data-result-index="${selectedIndex}"]`
      );
      element?.scrollIntoView({ block: "nearest", behavior: "smooth" });
    }
  }, [selectedIndex, showDropdown, groupedList.length]);

  // Highlight match in content
  const highlightContent = (content: string, start: number, end: number) => {
    if (start === undefined || end === undefined) return content;
    const maxLen = 80;
    let displayContent = content;
    let displayStart = start;
    let displayEnd = end;

    // Truncate if too long
    if (content.length > maxLen) {
      const matchCenter = Math.floor((start + end) / 2);
      const displayOffset = Math.max(0, matchCenter - maxLen / 2);
      displayContent =
        (displayOffset > 0 ? "..." : "") +
        content.slice(displayOffset, displayOffset + maxLen) +
        (displayOffset + maxLen < content.length ? "..." : "");
      displayStart = start - displayOffset + (displayOffset > 0 ? 3 : 0);
      displayEnd = end - displayOffset + (displayOffset > 0 ? 3 : 0);
    }

    return (
      <>
        {displayContent.slice(0, displayStart)}
        <mark className="rounded bg-yellow-300 px-0.5 text-yellow-900 dark:bg-yellow-500/40 dark:text-yellow-100">
          {displayContent.slice(displayStart, displayEnd)}
        </mark>
        {displayContent.slice(displayEnd)}
      </>
    );
  };

  // Dynamic placeholder based on search scope
  const getPlaceholder = () => {
    if (searchInRepo && repoName) {
      return `Search in ${repoName.split("/").pop()}...`;
    }
    if (activeContext) {
      return `Search in ${activeContext.name} (${activeContext.repos.length} repos)...`;
    }
    return "Search all repositories...";
  };

  const defaultPlaceholder = getPlaceholder();

  return (
    <div className={`relative ${className}`}>
      <SearchInput
        query={query}
        loading={loading}
        placeholder={placeholder || defaultPlaceholder}
        inputRef={inputRef}
        onChange={handleInputChange}
        onFocus={() => query && setShowDropdown(true)}
        onKeyDown={handleKeyDown}
        onClear={() => {
          setState((prev) => ({
            ...prev,
            query: "",
            results: null,
            showDropdown: false,
          }));
          inputRef.current?.focus();
        }}
        repoId={repoId}
        repoName={repoName}
        searchInRepo={searchInRepo}
        onToggleScope={() => setSearchInRepo(!searchInRepo)}
      />

      {/* Dropdown results */}
      {showDropdown && (query || loading) && (
        <div
          ref={dropdownRef}
          className="absolute left-0 right-0 top-full z-50 mt-1 max-h-[60vh] overflow-hidden rounded-xl border border-gray-200 bg-white shadow-xl dark:border-gray-700 dark:bg-gray-800"
        >
          {loading && !results && (
            <div className="p-4 text-center text-sm text-gray-500 dark:text-gray-400">
              <Loader2 className="mx-auto mb-2 h-5 w-5 animate-spin" />
              Searching...
            </div>
          )}

          {!loading && results && results.results.length === 0 && (
            <div className="p-4 text-center text-sm text-gray-500 dark:text-gray-400">
              No results found
            </div>
          )}

          {results && groupedList.length > 0 && (
            <SearchDropdownResults
              results={results}
              groupedList={groupedList}
              selectedIndex={selectedIndex}
              searchInRepo={searchInRepo}
              repoName={repoName}
              query={query}
              activeContext={activeContext}
              onSelect={navigateToResult}
              highlightContent={highlightContent}
            />
          )}

          <SearchDropdownFooter />
        </div>
      )}
    </div>
  );
}

export default SearchDropdown;
