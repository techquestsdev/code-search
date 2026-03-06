"use client";

import { useState, useCallback, useEffect } from "react";
import { BrowseTab } from "@/components/BrowseTabBar";
import { windowStorage } from "@/lib/windowStorage";

const STORAGE_KEY = "browse-tabs";

interface BrowseTabsState {
  tabs: BrowseTab[];
  activeTabId: string | null;
}

function generateTabId(): string {
  return `tab_${Date.now()}_${Math.random().toString(36).substr(2, 9)}`;
}

function loadState(): BrowseTabsState {
  try {
    const stored = windowStorage.getItem(STORAGE_KEY);
    if (stored) {
      return JSON.parse(stored);
    }
  } catch (e) {
    console.error("Failed to load browse tabs:", e);
  }
  return { tabs: [], activeTabId: null };
}

function saveState(state: BrowseTabsState): void {
  try {
    windowStorage.setItem(STORAGE_KEY, JSON.stringify(state));
  } catch (e) {
    console.error("Failed to save browse tabs:", e);
  }
}

export function useBrowseTabs(
  currentRepoId: number,
  currentPath: string,
  currentRef?: string
) {
  // Tabs are persisted per-window using windowStorage
  // Each browser tab/window has its own isolated storage
  const [state, setState] = useState<BrowseTabsState>(() => loadState());

  // Save state when it changes
  useEffect(() => {
    saveState(state);
  }, [state]);

  // Sync current file with tabs - add if not present, make active
  useEffect(() => {
    if (!currentPath) return; // Don't add tabs for directory views

    setState((prev) => {
      // Check if this file is already in tabs
      const existingTab = prev.tabs.find(
        (t) => t.repoId === currentRepoId && t.filePath === currentPath
      );

      if (existingTab) {
        // Just make it active if not already
        if (prev.activeTabId === existingTab.id) {
          return prev;
        }
        return {
          ...prev,
          activeTabId: existingTab.id,
        };
      }

      // Add new tab
      const newTab: BrowseTab = {
        id: generateTabId(),
        repoId: currentRepoId,
        repoName: "", // Will be filled by the page
        filePath: currentPath,
        ref: currentRef,
      };

      return {
        tabs: [...prev.tabs, newTab],
        activeTabId: newTab.id,
      };
    });
  }, [currentRepoId, currentPath, currentRef]);

  const openTab = useCallback(
    (
      repoId: number,
      repoName: string,
      filePath: string,
      ref?: string,
      language?: string
    ) => {
      setState((prev) => {
        // Check if already open
        const existingTab = prev.tabs.find(
          (t) => t.repoId === repoId && t.filePath === filePath
        );

        if (existingTab) {
          return {
            ...prev,
            activeTabId: existingTab.id,
          };
        }

        // Create new tab
        const newTab: BrowseTab = {
          id: generateTabId(),
          repoId,
          repoName,
          filePath,
          ref,
          language,
        };

        return {
          tabs: [...prev.tabs, newTab],
          activeTabId: newTab.id,
        };
      });
    },
    []
  );

  const closeTab = useCallback((tabId: string) => {
    setState((prev) => {
      const tabIndex = prev.tabs.findIndex((t) => t.id === tabId);
      if (tabIndex === -1) return prev;

      const newTabs = prev.tabs.filter((t) => t.id !== tabId);

      // Determine new active tab
      let newActiveTabId: string | null = null;
      if (newTabs.length > 0) {
        if (prev.activeTabId === tabId) {
          // Activate adjacent tab
          newActiveTabId =
            newTabs[Math.min(tabIndex, newTabs.length - 1)]?.id || null;
        } else {
          newActiveTabId = prev.activeTabId;
        }
      }

      return {
        tabs: newTabs,
        activeTabId: newActiveTabId,
      };
    });
  }, []);

  const setActiveTab = useCallback((tabId: string) => {
    setState((prev) => ({
      ...prev,
      activeTabId: tabId,
    }));
  }, []);

  const updateTabRepoName = useCallback((tabId: string, repoName: string) => {
    setState((prev) => ({
      ...prev,
      tabs: prev.tabs.map((t) => (t.id === tabId ? { ...t, repoName } : t)),
    }));
  }, []);

  const clearTabs = useCallback(() => {
    setState({ tabs: [], activeTabId: null });
  }, []);

  // Replace all tabs at once (used when closing split pane)
  const replaceTabs = useCallback(
    (newTabs: BrowseTab[], newActiveTabId: string | null) => {
      setState({ tabs: newTabs, activeTabId: newActiveTabId });
    },
    []
  );

  const activeTab = state.tabs.find((t) => t.id === state.activeTabId) || null;

  return {
    tabs: state.tabs,
    activeTab,
    activeTabId: state.activeTabId,
    openTab,
    closeTab,
    setActiveTab,
    updateTabRepoName,
    clearTabs,
    replaceTabs,
  };
}
