// Context (Workspace) management
// - Contexts list stored in localStorage (shared across all windows)
// - Active context ID stored per-window using windowStorage

import { windowStorage } from "./windowStorage";

export interface ContextRepo {
  id: number;
  name: string;
  addedAt: string;
  addedByQuery?: string; // The search query used when this repo was added
  addedByRegex?: boolean; // Whether the query was a regex
}

export interface Context {
  id: string;
  name: string;
  description?: string;
  color: string;
  icon?: string;
  repos: ContextRepo[];
  repoFilter?: string; // Search/regex pattern used to select repos
  isRegexFilter?: boolean; // Whether repoFilter is a regex
  createdAt: string;
  updatedAt: string;
}

export interface ContextState {
  contexts: Context[];
  activeContextId: string | null;
}

const CONTEXTS_STORAGE_KEY = "code-search-contexts";
const ACTIVE_CONTEXT_KEY = "active-context";
const DEFAULT_COLORS = [
  "#3B82F6", // blue
  "#10B981", // green
  "#F59E0B", // amber
  "#EF4444", // red
  "#8B5CF6", // purple
  "#EC4899", // pink
  "#06B6D4", // cyan
  "#F97316", // orange
];

// Generate a unique ID
function generateId(): string {
  return `ctx_${Date.now()}_${Math.random().toString(36).substr(2, 9)}`;
}

// Get all contexts from localStorage and active context from windowStorage (per-window)
export function getContexts(): ContextState {
  if (typeof window === "undefined") {
    return { contexts: [], activeContextId: null };
  }

  let contexts: Context[] = [];
  let activeContextId: string | null = null;

  try {
    // Load contexts from localStorage (shared across all windows)
    const storedContexts = localStorage.getItem(CONTEXTS_STORAGE_KEY);
    if (storedContexts) {
      const parsed = JSON.parse(storedContexts);
      // Handle both old format (with activeContextId) and new format (contexts only)
      contexts = Array.isArray(parsed) ? parsed : parsed.contexts || [];
    }
  } catch (e) {
    console.error("Failed to load contexts:", e);
  }

  try {
    // Load active context from windowStorage (per-window, persists across refresh)
    activeContextId = windowStorage.getItem(ACTIVE_CONTEXT_KEY);
  } catch (e) {
    console.error("Failed to load active context:", e);
  }

  return { contexts, activeContextId };
}

// Save contexts to localStorage and active context to windowStorage
export function saveContexts(state: ContextState): void {
  if (typeof window === "undefined") return;

  try {
    // Save contexts to localStorage (shared across windows)
    localStorage.setItem(CONTEXTS_STORAGE_KEY, JSON.stringify(state.contexts));
  } catch (e) {
    console.error("Failed to save contexts:", e);
  }

  try {
    // Save active context to windowStorage (per-window, persists across refresh)
    if (state.activeContextId) {
      windowStorage.setItem(ACTIVE_CONTEXT_KEY, state.activeContextId);
    } else {
      windowStorage.removeItem(ACTIVE_CONTEXT_KEY);
    }
  } catch (e) {
    console.error("Failed to save active context:", e);
  }
}

// Create a new context
export function createContext(
  name: string,
  description?: string,
  color?: string
): Context {
  const now = new Date().toISOString();
  return {
    id: generateId(),
    name,
    description,
    color:
      color ||
      DEFAULT_COLORS[Math.floor(Math.random() * DEFAULT_COLORS.length)],
    repos: [],
    createdAt: now,
    updatedAt: now,
  };
}

// Add a repo to a context
export function addRepoToContext(
  context: Context,
  repoId: number,
  repoName: string,
  query?: string,
  isRegex?: boolean
): Context {
  if (context.repos.some((r) => r.id === repoId)) {
    return context; // Already exists
  }

  return {
    ...context,
    repos: [
      ...context.repos,
      {
        id: repoId,
        name: repoName,
        addedAt: new Date().toISOString(),
        addedByQuery: query || undefined,
        addedByRegex: isRegex === true ? true : undefined, // Only store if explicitly true
      },
    ],
    updatedAt: new Date().toISOString(),
  };
}

// Remove a repo from a context
export function removeRepoFromContext(
  context: Context,
  repoId: number
): Context {
  return {
    ...context,
    repos: context.repos.filter((r) => r.id !== repoId),
    updatedAt: new Date().toISOString(),
  };
}

// Get the active context
export function getActiveContext(state: ContextState): Context | null {
  if (!state.activeContextId) return null;
  return state.contexts.find((c) => c.id === state.activeContextId) || null;
}

// Build a repo filter for search queries
// Returns a regex OR pattern for use with repo: filter, or null if no context
// Format: (repo1|repo2|repo3) - zoekt's repo: filter expects a regex pattern
export function buildRepoFilter(context: Context | null): string | null {
  if (!context || context.repos.length === 0) return null;

  // Escape special regex characters in repo names, then join with OR
  const escapedNames = context.repos.map((r) =>
    r.name.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")
  );

  // Return as regex OR pattern: (name1|name2|name3)
  return `(${escapedNames.join("|")})`;
}

// Check if a repo is in the active context
export function isRepoInContext(
  context: Context | null,
  repoId: number
): boolean {
  if (!context) return true; // No context = all repos allowed
  return context.repos.some((r) => r.id === repoId);
}

// Get suggested color for a new context
export function getNextColor(existingContexts: Context[]): string {
  const usedColors = new Set(existingContexts.map((c) => c.color));
  const available = DEFAULT_COLORS.find((c) => !usedColors.has(c));
  return (
    available || DEFAULT_COLORS[existingContexts.length % DEFAULT_COLORS.length]
  );
}

export const CONTEXT_COLORS = DEFAULT_COLORS;
