import React from "react";
import { Search, X, Loader2, ToggleRight, ToggleLeft } from "lucide-react";

interface SearchInputProps {
  query: string;
  loading: boolean;
  placeholder: string;
  inputRef: React.RefObject<HTMLInputElement | null>;
  onChange: (e: React.ChangeEvent<HTMLInputElement>) => void;
  onFocus: () => void;
  onKeyDown: (e: React.KeyboardEvent) => void;
  onClear: () => void;
  repoId?: number;
  repoName?: string;
  searchInRepo: boolean;
  onToggleScope: () => void;
}

export function SearchInput({
  query,
  loading,
  placeholder,
  inputRef,
  onChange,
  onFocus,
  onKeyDown,
  onClear,
  repoId,
  repoName,
  searchInRepo,
  onToggleScope
}: SearchInputProps) {
  return (
    <div className="flex items-center gap-2">
      {/* Search input */}
      <div className="relative flex-1">
        <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-gray-400" />
        <input
          ref={inputRef}
          type="text"
          value={query}
          onChange={onChange}
          onFocus={onFocus}
          onKeyDown={onKeyDown}
          placeholder={placeholder}
          aria-label={placeholder}
          className="w-full pl-9 pr-8 py-1.5 text-sm border border-gray-200 dark:border-gray-700 rounded-lg bg-white dark:bg-gray-800 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent"
        />
        {query && (
          <button
            onClick={onClear}
            className="absolute right-2 top-1/2 -translate-y-1/2 p-0.5 text-gray-400 hover:text-gray-600 dark:hover:text-gray-300"
          >
            <X className="w-4 h-4" />
          </button>
        )}
        {loading && (
          <Loader2 className="absolute right-8 top-1/2 -translate-y-1/2 w-4 h-4 text-gray-400 animate-spin" />
        )}
      </div>

      {/* Search scope toggle */}
      {repoId && repoName && (
        <button
          onClick={onToggleScope}
          className={`flex items-center gap-1.5 px-2 py-1.5 text-xs font-medium rounded-lg border transition-colors ${searchInRepo
            ? "bg-blue-50 dark:bg-blue-900/30 border-blue-200 dark:border-blue-800 text-blue-700 dark:text-blue-300"
            : "bg-gray-50 dark:bg-gray-800 border-gray-200 dark:border-gray-700 text-gray-600 dark:text-gray-400"
            }`}
          title={searchInRepo ? "Searching in this repo" : "Searching all repos"}
        >
          {searchInRepo ? (
            <ToggleRight className="w-4 h-4" />
          ) : (
            <ToggleLeft className="w-4 h-4" />
          )}
          <span className="hidden sm:inline">
            {searchInRepo ? "This repo" : "All repos"}
          </span>
        </button>
      )}
    </div>
  );
}
