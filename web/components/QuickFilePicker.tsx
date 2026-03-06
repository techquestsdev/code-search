"use client";

import { useEffect, useRef, useCallback, useMemo, useReducer } from "react";
import { api } from "@/lib/api";
import {
  Search,
  X,
  Loader2,
} from "lucide-react";
import { QuickFilePickerList } from "./QuickFilePickerList";

interface QuickFilePickerProps {
  repoId: number;
  currentRef?: string;
  isOpen: boolean;
  onClose: () => void;
  onSelect: (path: string) => void;
}

export interface FileEntry {
  path: string;
  name: string;
  type: "file" | "dir";
  language?: string;
}

export function QuickFilePicker({
  repoId,
  currentRef,
  isOpen,
  onClose,
  onSelect,
}: QuickFilePickerProps) {
  const [pickerState, updatePicker] = useReducer(
    (state: { query: string; files: FileEntry[]; selectedIndex: number; loading: boolean }, update: Partial<{ query: string; files: FileEntry[]; selectedIndex: number; loading: boolean }>) => ({ ...state, ...update }),
    { query: "", files: [] as FileEntry[], selectedIndex: 0, loading: false }
  );

  const { query, files, selectedIndex, loading } = pickerState;

  const setQuery = useCallback((q: string | ((prev: string) => string)) => {
    const newQ = typeof q === 'function' ? q(query) : q;
    updatePicker({ query: newQ, selectedIndex: 0 });
  }, [query]);
  const setSelectedIndex = useCallback((i: number | ((prev: number) => number)) => {
    const newI = typeof i === 'function' ? i(selectedIndex) : i;
    updatePicker({ selectedIndex: newI });
  }, [selectedIndex]);

  const inputRef = useRef<HTMLInputElement>(null);
  const listRef = useRef<HTMLDivElement>(null);

  // Load all files in repo (recursive tree walk)
  useEffect(() => {
    if (!isOpen) return;

    const loadFiles = async () => {
      try {
        const allFiles: FileEntry[] = [];
        const queue: string[] = [""]; // Start from root

        // BFS to walk the tree
        while (queue.length > 0 && allFiles.length < 5000) {
          const currentPath = queue.shift()!;
          try {
            const tree = await api.getTree(repoId, currentPath || undefined, currentRef);
            for (const entry of tree.entries) {
              if (entry.type === "dir") {
                queue.push(entry.path);
              }
              allFiles.push({
                path: entry.path,
                name: entry.name,
                type: entry.type as "file" | "dir",
                language: entry.language,
              });
            }
          } catch {
            // Skip directories we can't read
          }
        }

        // Sort: files first, then directories, alphabetically
        allFiles.sort((a, b) => {
          if (a.type !== b.type) {
            return a.type === "file" ? -1 : 1;
          }
          return a.path.localeCompare(b.path);
        });

        updatePicker({ files: allFiles, loading: false });
      } catch (err) {
        console.error("Failed to load files:", err);
        updatePicker({ loading: false });
      }
    };

    updatePicker({ query: "", selectedIndex: 0, loading: true });
    setTimeout(() => inputRef.current?.focus(), 0);
    loadFiles();
  }, [repoId, currentRef, isOpen]);

  // Derived filtered files
  const filteredFiles = useMemo(() => {
    if (!query.trim()) {
      return files.slice(0, 100);
    }

    const lowerQuery = query.toLowerCase();
    const parts = lowerQuery.split(/[\s/]/);

    // Score and filter files
    return files
      .filter((file) => file.type === "file") // Only show files in search
      .map((file) => {
        const lowerPath = file.path.toLowerCase();
        const lowerName = file.name.toLowerCase();

        let score = 0;

        // Exact match on name
        if (lowerName === lowerQuery) {
          score += 100;
        }
        // Name starts with query
        else if (lowerName.startsWith(lowerQuery)) {
          score += 50;
        }
        // Name contains query
        else if (lowerName.includes(lowerQuery)) {
          score += 25;
        }
        // Path contains query
        else if (lowerPath.includes(lowerQuery)) {
          score += 10;
        }
        // Fuzzy match - all parts present
        else {
          const allPartsMatch = parts.every((part) => lowerPath.includes(part));
          if (allPartsMatch) {
            score += 5;
          }
        }

        // Bonus for shorter paths
        score += Math.max(0, 10 - file.path.split("/").length);

        return { file, score };
      })
      .filter((item) => item.score > 0)
      .sort((a, b) => b.score - a.score)
      .slice(0, 100)
      .map((item) => item.file);
  }, [query, files]);

  // Scroll selected item into view
  useEffect(() => {
    const selectedElement = listRef.current?.querySelector(
      `[data-index="${selectedIndex}"]`
    );
    selectedElement?.scrollIntoView({ block: "nearest" });
  }, [selectedIndex]);

  // Handle keyboard navigation
  const handleKeyDown = useCallback(
    (e: React.KeyboardEvent) => {
      switch (e.key) {
        case "ArrowDown":
          e.preventDefault();
          setSelectedIndex((i) => Math.min(i + 1, filteredFiles.length - 1));
          break;
        case "ArrowUp":
          e.preventDefault();
          setSelectedIndex((i) => Math.max(i - 1, 0));
          break;
        case "Enter":
          e.preventDefault();
          if (filteredFiles[selectedIndex]) {
            onSelect(filteredFiles[selectedIndex].path);
            onClose();
          }
          break;
        case "Escape":
          e.preventDefault();
          onClose();
          break;
      }
    },
    [filteredFiles, selectedIndex, onSelect, onClose, setSelectedIndex]
  );

  // Close on backdrop click
  const handleBackdropClick = useCallback(
    (e: React.MouseEvent) => {
      if (e.target === e.currentTarget) {
        onClose();
      }
    },
    [onClose]
  );

  if (!isOpen) return null;

  return (
    <div
      role="dialog"
      aria-modal="true"
      className="fixed inset-0 z-50 flex items-start justify-center pt-[15vh] bg-black/50 backdrop-blur-sm"
    >
      {/* Backdrop button for closing */}
      <button
        className="absolute inset-0 w-full h-full cursor-default"
        onClick={handleBackdropClick}
        onKeyDown={(e) => {
          if (e.key === 'Escape') onClose();
        }}
        aria-label="Close file picker"
        tabIndex={-1}
      />
      <div className="relative w-full max-w-xl bg-white dark:bg-gray-800 rounded-xl shadow-2xl border border-gray-200 dark:border-gray-700 overflow-hidden">
        {/* Search input */}
        <div className="flex items-center gap-3 px-4 py-3 border-b border-gray-200 dark:border-gray-700">
          <Search className="w-5 h-5 text-gray-400" />
          <input
            ref={inputRef}
            type="text"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            onKeyDown={handleKeyDown}
            placeholder="Search files by name..."
            className="flex-1 bg-transparent text-base outline-none placeholder:text-gray-400"
            autoComplete="off"
            autoCorrect="off"
            autoCapitalize="off"
            spellCheck={false}
          />
          {loading && <Loader2 className="w-4 h-4 animate-spin text-gray-400" />}
          <button
            onClick={onClose}
            className="p-1 text-gray-400 hover:text-gray-600 dark:hover:text-gray-300 transition-colors"
          >
            <X className="w-4 h-4" />
          </button>
        </div>

        {/* Results list */}
        <QuickFilePickerList
          filteredFiles={filteredFiles}
          loading={loading}
          selectedIndex={selectedIndex}
          query={query}
          onSelect={onSelect}
          onClose={onClose}
          listRef={listRef}
        />

        {/* Footer hints */}
        <div className="flex items-center gap-4 px-4 py-2 border-t border-gray-200 dark:border-gray-700 text-xs text-gray-400">
          <span>
            <kbd className="px-1.5 py-0.5 bg-gray-100 dark:bg-gray-700 rounded">↑↓</kbd>{" "}
            navigate
          </span>
          <span>
            <kbd className="px-1.5 py-0.5 bg-gray-100 dark:bg-gray-700 rounded">Enter</kbd>{" "}
            open
          </span>
          <span>
            <kbd className="px-1.5 py-0.5 bg-gray-100 dark:bg-gray-700 rounded">Esc</kbd>{" "}
            close
          </span>
          <span className="ml-auto">
            {filteredFiles.length} {filteredFiles.length === 1 ? "file" : "files"}
          </span>
        </div>
      </div>
    </div>
  );
}

// Highlight matching text in results
export function highlightMatch(text: string, query: string): React.ReactNode {
  if (!query.trim()) return text;

  const lowerText = text.toLowerCase();
  const lowerQuery = query.toLowerCase();
  const index = lowerText.indexOf(lowerQuery);

  if (index === -1) return text;

  return (
    <>
      {text.slice(0, index)}
      <mark className="bg-yellow-200 dark:bg-yellow-500/40 text-inherit rounded">
        {text.slice(index, index + query.length)}
      </mark>
      {text.slice(index + query.length)}
    </>
  );
}
