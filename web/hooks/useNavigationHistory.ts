"use client";

import { useState, useCallback, useEffect, useRef } from "react";

export interface HistoryEntry {
  repoId: number;
  path: string;
  ref?: string;
  line?: number;
  timestamp: number;
}

interface UseNavigationHistoryOptions {
  maxHistory?: number;
}

interface NavigationHistoryState {
  entries: HistoryEntry[];
  currentIndex: number;
}

const HISTORY_KEY = "code-search-nav-history";
const MAX_HISTORY = 50;

// Load history from session storage
function loadHistory(): NavigationHistoryState {
  if (typeof window === "undefined") {
    return { entries: [], currentIndex: -1 };
  }
  try {
    const stored = sessionStorage.getItem(HISTORY_KEY);
    if (stored) {
      return JSON.parse(stored);
    }
  } catch {
    // Ignore storage errors
  }
  return { entries: [], currentIndex: -1 };
}

// Save history to session storage
function saveHistory(state: NavigationHistoryState) {
  if (typeof window === "undefined") return;
  try {
    sessionStorage.setItem(HISTORY_KEY, JSON.stringify(state));
  } catch {
    // Ignore storage errors
  }
}

export function useNavigationHistory(
  options: UseNavigationHistoryOptions = {}
) {
  const { maxHistory = MAX_HISTORY } = options;
  const [state, setState] = useState<NavigationHistoryState>(() =>
    loadHistory()
  );
  const isNavigatingRef = useRef(false);

  // Save to session storage whenever state changes
  useEffect(() => {
    saveHistory(state);
  }, [state]);

  // Add a new entry to history
  const push = useCallback(
    (entry: Omit<HistoryEntry, "timestamp">) => {
      // Skip if we're navigating back/forward
      if (isNavigatingRef.current) {
        isNavigatingRef.current = false;
        return;
      }

      setState((prev) => {
        // Check if this is the same as current entry (avoid duplicates)
        const currentEntry = prev.entries[prev.currentIndex];
        if (
          currentEntry &&
          currentEntry.repoId === entry.repoId &&
          currentEntry.path === entry.path &&
          currentEntry.ref === entry.ref
        ) {
          // Same location, just update line if different
          if (entry.line && entry.line !== currentEntry.line) {
            const newEntries = [...prev.entries];
            newEntries[prev.currentIndex] = {
              ...currentEntry,
              line: entry.line,
            };
            return { ...prev, entries: newEntries };
          }
          return prev;
        }

        // Remove entries after current index (we're creating a new branch)
        const newEntries = prev.entries.slice(0, prev.currentIndex + 1);

        // Add new entry
        newEntries.push({
          ...entry,
          timestamp: Date.now(),
        });

        // Trim to max history
        const trimmedEntries = newEntries.slice(-maxHistory);
        const newIndex = trimmedEntries.length - 1;

        return {
          entries: trimmedEntries,
          currentIndex: newIndex,
        };
      });
    },
    [maxHistory]
  );

  // Go back in history
  const goBack = useCallback((): HistoryEntry | null => {
    if (state.currentIndex > 0) {
      const targetEntry = state.entries[state.currentIndex - 1];
      isNavigatingRef.current = true;
      setState((prev) => ({ ...prev, currentIndex: prev.currentIndex - 1 }));
      return targetEntry;
    }
    return null;
  }, [state.currentIndex, state.entries]);

  // Go forward in history
  const goForward = useCallback((): HistoryEntry | null => {
    if (state.currentIndex < state.entries.length - 1) {
      const targetEntry = state.entries[state.currentIndex + 1];
      isNavigatingRef.current = true;
      setState((prev) => ({ ...prev, currentIndex: prev.currentIndex + 1 }));
      return targetEntry;
    }
    return null;
  }, [state.currentIndex, state.entries]);

  // Clear history
  const clear = useCallback(() => {
    setState({ entries: [], currentIndex: -1 });
  }, []);

  // Get current entry
  const currentEntry = state.entries[state.currentIndex] || null;

  // Check if can go back/forward
  const canGoBack = state.currentIndex > 0;
  const canGoForward = state.currentIndex < state.entries.length - 1;

  // Get recent history (for display)
  const recentHistory = state.entries
    .slice(Math.max(0, state.currentIndex - 10), state.currentIndex + 1)
    .reverse();

  return {
    push,
    goBack,
    goForward,
    clear,
    currentEntry,
    canGoBack,
    canGoForward,
    recentHistory,
    historyLength: state.entries.length,
    currentIndex: state.currentIndex,
  };
}
