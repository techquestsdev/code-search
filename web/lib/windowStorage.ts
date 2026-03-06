"use client";

// Window-specific storage that persists within a browser tab but is isolated between tabs
// Uses localStorage with a unique window ID to achieve both persistence and isolation

const WINDOW_ID_KEY = "code-search-window-id";
const WINDOW_ID_TIMESTAMP_KEY = "code-search-window-id-ts";

// Generate a unique window ID
function generateWindowId(): string {
  return `win_${Date.now()}_${Math.random().toString(36).substring(2, 11)}`;
}

// Get or create a unique ID for this browser window/tab
// This ID persists across page refreshes but is unique per tab
function getWindowId(): string {
  if (typeof window === "undefined") {
    return "ssr";
  }

  // Check if we already have a window ID in memory (fastest)
  const memoryId = (window as typeof window & { __codeSearchWindowId?: string })
    .__codeSearchWindowId;
  if (memoryId) {
    return memoryId;
  }

  // Check sessionStorage for existing window ID
  let windowId = sessionStorage.getItem(WINDOW_ID_KEY);
  const storedTimestamp = sessionStorage.getItem(WINDOW_ID_TIMESTAMP_KEY);

  // Detect if this is a cloned sessionStorage from opening a new tab:
  // When a new tab is opened via link, sessionStorage is cloned but performance.navigation
  // or the page load timing can help us detect it
  const isNewTab = !windowId || detectNewTab(storedTimestamp);

  if (isNewTab) {
    // Generate new window ID for this tab
    windowId = generateWindowId();
    sessionStorage.setItem(WINDOW_ID_KEY, windowId);
    sessionStorage.setItem(WINDOW_ID_TIMESTAMP_KEY, Date.now().toString());
  }

  // Store in memory for fast access (windowId is guaranteed non-null here)
  (
    window as typeof window & { __codeSearchWindowId?: string }
  ).__codeSearchWindowId = windowId!;

  return windowId!;
}

// Detect if this is a new tab (vs a refresh of existing tab)
function detectNewTab(storedTimestamp: string | null): boolean {
  if (!storedTimestamp) return true;

  // Use performance API to check navigation type
  const navEntries = performance.getEntriesByType(
    "navigation"
  ) as PerformanceNavigationTiming[];
  if (navEntries.length > 0) {
    const navType = navEntries[0].type;
    // "navigate" means new navigation (could be new tab or typed URL)
    // "reload" means page refresh
    // "back_forward" means browser back/forward
    if (navType === "reload" || navType === "back_forward") {
      return false; // Same tab, just refreshed or navigated
    }
  }

  // Check if the stored timestamp is very recent (within 1 second)
  // If we have a timestamp and it's recent, this is likely a cloned sessionStorage
  const ts = parseInt(storedTimestamp, 10);
  const timeSinceStored = Date.now() - ts;

  // If timestamp is older than 2 seconds, it's likely a refresh of the same tab
  // If it's very fresh (< 2s), it could be a new tab that cloned sessionStorage
  if (timeSinceStored > 2000) {
    return false; // Old enough to be a refresh
  }

  // Fresh timestamp - check if we have any history for this page
  // A new tab typically has navigationStart very close to now
  const perfNow = performance.now();
  if (perfNow < 1000) {
    // Page just loaded and timestamp is fresh - likely a new tab
    return true;
  }

  return false;
}

// Get a storage key namespaced to this window
function getWindowKey(key: string): string {
  return `${getWindowId()}:${key}`;
}

// Window-scoped storage API
export const windowStorage = {
  getItem(key: string): string | null {
    if (typeof window === "undefined") return null;
    return localStorage.getItem(getWindowKey(key));
  },

  setItem(key: string, value: string): void {
    if (typeof window === "undefined") return;
    localStorage.setItem(getWindowKey(key), value);
  },

  removeItem(key: string): void {
    if (typeof window === "undefined") return;
    localStorage.removeItem(getWindowKey(key));
  },

  // Get the current window ID (useful for debugging)
  getWindowId,

  // Clear all data for this window
  clearWindow(): void {
    if (typeof window === "undefined") return;
    const windowId = getWindowId();
    const keysToRemove: string[] = [];

    for (let i = 0; i < localStorage.length; i++) {
      const key = localStorage.key(i);
      if (key?.startsWith(`${windowId}:`)) {
        keysToRemove.push(key);
      }
    }

    keysToRemove.forEach((key) => localStorage.removeItem(key));
  },

  // Clean up old window data (call periodically)
  cleanupOldWindows(maxAgeMs: number = 7 * 24 * 60 * 60 * 1000): void {
    if (typeof window === "undefined") return;

    const now = Date.now();
    const keysToRemove: string[] = [];

    for (let i = 0; i < localStorage.length; i++) {
      const key = localStorage.key(i);
      if (key?.startsWith("win_")) {
        // Extract timestamp from window ID
        const match = key.match(/^win_(\d+)_/);
        if (match) {
          const timestamp = parseInt(match[1], 10);
          if (now - timestamp > maxAgeMs) {
            keysToRemove.push(key);
          }
        }
      }
    }

    keysToRemove.forEach((key) => localStorage.removeItem(key));
  },
};
