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
  onToggleScope,
}: SearchInputProps) {
  return (
    <div className="flex items-center gap-2">
      {/* Search input */}
      <div className="relative flex-1">
        <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-gray-400" />
        <input
          ref={inputRef}
          type="text"
          value={query}
          onChange={onChange}
          onFocus={onFocus}
          onKeyDown={onKeyDown}
          placeholder={placeholder}
          aria-label={placeholder}
          className="w-full rounded-lg border border-gray-200 bg-white py-1.5 pl-9 pr-8 text-sm focus:border-transparent focus:outline-none focus:ring-2 focus:ring-blue-500 dark:border-gray-700 dark:bg-gray-800"
        />
        {query && (
          <button
            onClick={onClear}
            className="absolute right-2 top-1/2 -translate-y-1/2 p-0.5 text-gray-400 hover:text-gray-600 dark:hover:text-gray-300"
          >
            <X className="h-4 w-4" />
          </button>
        )}
        {loading && (
          <Loader2 className="absolute right-8 top-1/2 h-4 w-4 -translate-y-1/2 animate-spin text-gray-400" />
        )}
      </div>

      {/* Search scope toggle */}
      {repoId && repoName && (
        <button
          onClick={onToggleScope}
          className={`flex items-center gap-1.5 rounded-lg border px-2 py-1.5 text-xs font-medium transition-colors ${
            searchInRepo
              ? "border-blue-200 bg-blue-50 text-blue-700 dark:border-blue-800 dark:bg-blue-900/30 dark:text-blue-300"
              : "border-gray-200 bg-gray-50 text-gray-600 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-400"
          }`}
          title={
            searchInRepo ? "Searching in this repo" : "Searching all repos"
          }
        >
          {searchInRepo ? (
            <ToggleRight className="h-4 w-4" />
          ) : (
            <ToggleLeft className="h-4 w-4" />
          )}
          <span className="hidden sm:inline">
            {searchInRepo ? "This repo" : "All repos"}
          </span>
        </button>
      )}
    </div>
  );
}
