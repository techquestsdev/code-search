"use client";

import React, { createContext, useContext, useState, useEffect, useCallback, ReactNode } from "react";
import {
  Context,
  ContextState,
  getContexts,
  saveContexts,
  createContext as createNewContext,
  addRepoToContext,
  removeRepoFromContext,
  getActiveContext,
  buildRepoFilter,
  isRepoInContext,
  getNextColor,
} from "@/lib/contexts";

interface ContextProviderState {
  // State
  contexts: Context[];
  activeContext: Context | null;
  isLoading: boolean;

  // Context CRUD
  createContext: (name: string, description?: string, color?: string) => Context;
  updateContext: (id: string, updates: Partial<Pick<Context, "name" | "description" | "color" | "repoFilter" | "isRegexFilter">>) => void;
  deleteContext: (id: string) => void;

  // Active context
  setActiveContext: (id: string | null) => void;

  // Repo management
  addRepo: (contextId: string, repoId: number, repoName: string, query?: string, isRegex?: boolean) => void;
  removeRepo: (contextId: string, repoId: number) => void;

  // Utilities
  getRepoFilter: () => string | null;
  isRepoInActiveContext: (repoId: number) => boolean;
  getNextColor: () => string;
}

const ContextProviderContext = createContext<ContextProviderState | null>(null);

export function ContextProvider({ children }: { children: ReactNode }) {
  const [state, setState] = useState<ContextState>({ contexts: [], activeContextId: null });
  const [isLoading, setIsLoading] = useState(true);

  // Load from localStorage on mount
  useEffect(() => {
    const loaded = getContexts();
    setState(loaded);
    setIsLoading(false);
  }, []);

  // Save to localStorage when state changes
  useEffect(() => {
    if (!isLoading) {
      saveContexts(state);
    }
  }, [state, isLoading]);

  // Get active context
  const activeContext = getActiveContext(state);

  // Create a new context
  const handleCreateContext = useCallback((name: string, description?: string, color?: string) => {
    const newContext = createNewContext(name, description, color);
    setState((prev) => ({
      ...prev,
      contexts: [...prev.contexts, newContext],
    }));
    return newContext;
  }, []);

  // Update a context
  const handleUpdateContext = useCallback(
    (id: string, updates: Partial<Pick<Context, "name" | "description" | "color" | "repoFilter" | "isRegexFilter">>) => {
      setState((prev) => ({
        ...prev,
        contexts: prev.contexts.map((ctx) =>
          ctx.id === id
            ? { ...ctx, ...updates, updatedAt: new Date().toISOString() }
            : ctx
        ),
      }));
    },
    []
  );

  // Delete a context
  const handleDeleteContext = useCallback((id: string) => {
    setState((prev) => ({
      contexts: prev.contexts.filter((ctx) => ctx.id !== id),
      activeContextId: prev.activeContextId === id ? null : prev.activeContextId,
    }));
  }, []);

  // Set active context
  const handleSetActiveContext = useCallback((id: string | null) => {
    setState((prev) => ({
      ...prev,
      activeContextId: id,
    }));
  }, []);

  // Add repo to context
  const handleAddRepo = useCallback((contextId: string, repoId: number, repoName: string, query?: string, isRegex?: boolean) => {
    setState((prev) => ({
      ...prev,
      contexts: prev.contexts.map((ctx) =>
        ctx.id === contextId ? addRepoToContext(ctx, repoId, repoName, query, isRegex) : ctx
      ),
    }));
  }, []);

  // Remove repo from context
  const handleRemoveRepo = useCallback((contextId: string, repoId: number) => {
    setState((prev) => ({
      ...prev,
      contexts: prev.contexts.map((ctx) =>
        ctx.id === contextId ? removeRepoFromContext(ctx, repoId) : ctx
      ),
    }));
  }, []);

  // Get repo filter for search
  const getRepoFilter = useCallback(() => {
    return buildRepoFilter(activeContext);
  }, [activeContext]);

  // Check if repo is in active context
  const isRepoInActiveContext = useCallback(
    (repoId: number) => {
      return isRepoInContext(activeContext, repoId);
    },
    [activeContext]
  );

  // Get next available color
  const handleGetNextColor = useCallback(() => {
    return getNextColor(state.contexts);
  }, [state.contexts]);

  const value: ContextProviderState = {
    contexts: state.contexts,
    activeContext,
    isLoading,
    createContext: handleCreateContext,
    updateContext: handleUpdateContext,
    deleteContext: handleDeleteContext,
    setActiveContext: handleSetActiveContext,
    addRepo: handleAddRepo,
    removeRepo: handleRemoveRepo,
    getRepoFilter,
    isRepoInActiveContext,
    getNextColor: handleGetNextColor,
  };

  return (
    <ContextProviderContext.Provider value={value}>
      {children}
    </ContextProviderContext.Provider>
  );
}

// Hook to use context
export function useContexts() {
  const context = useContext(ContextProviderContext);
  if (!context) {
    throw new Error("useContexts must be used within a ContextProvider");
  }
  return context;
}

// Hook to get just the active context (for components that only need to read)
export function useActiveContext() {
  const { activeContext, isRepoInActiveContext, getRepoFilter } = useContexts();
  return { activeContext, isRepoInActiveContext, getRepoFilter };
}
