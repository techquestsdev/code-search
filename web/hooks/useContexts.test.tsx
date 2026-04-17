import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { renderHook, act, waitFor } from "@testing-library/react";
import React from "react";

// Mock localStorage
const mockLocalStorage: Record<string, string> = {};
vi.stubGlobal("localStorage", {
  getItem: vi.fn((key: string) => mockLocalStorage[key] || null),
  setItem: vi.fn((key: string, value: string) => {
    mockLocalStorage[key] = value;
  }),
  removeItem: vi.fn((key: string) => {
    delete mockLocalStorage[key];
  }),
  clear: vi.fn(() => {
    Object.keys(mockLocalStorage).forEach(
      (key) => delete mockLocalStorage[key]
    );
  }),
});

// Mock windowStorage
vi.mock("@/lib/windowStorage", () => ({
  windowStorage: {
    getItem: vi.fn(() => null),
    setItem: vi.fn(),
    removeItem: vi.fn(),
  },
}));

// Import after mocking
import { ContextProvider, useContexts, useActiveContext } from "./useContexts";
import { windowStorage } from "@/lib/windowStorage";

// Wrapper component for the hook
const wrapper = ({ children }: { children: React.ReactNode }) => (
  <ContextProvider>{children}</ContextProvider>
);

describe("useContexts", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    Object.keys(mockLocalStorage).forEach(
      (key) => delete mockLocalStorage[key]
    );
    vi.mocked(windowStorage.getItem).mockReturnValue(null);
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it("should throw error when used outside provider", () => {
    // Suppress console.error for this test
    const consoleSpy = vi.spyOn(console, "error").mockImplementation(() => {});

    expect(() => {
      renderHook(() => useContexts());
    }).toThrow("useContexts must be used within a ContextProvider");

    consoleSpy.mockRestore();
  });

  it("should initialize with empty contexts", async () => {
    const { result } = renderHook(() => useContexts(), { wrapper });

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    expect(result.current.contexts).toEqual([]);
    expect(result.current.activeContext).toBeNull();
  });

  it("should create a new context", async () => {
    const { result } = renderHook(() => useContexts(), { wrapper });

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    act(() => {
      result.current.createContext("Test Context", "A test context", "#3B82F6");
    });

    expect(result.current.contexts).toHaveLength(1);
    expect(result.current.contexts[0].name).toBe("Test Context");
    expect(result.current.contexts[0].description).toBe("A test context");
    expect(result.current.contexts[0].color).toBe("#3B82F6");
  });

  it("should update a context", async () => {
    const { result } = renderHook(() => useContexts(), { wrapper });

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    let contextId: string;
    act(() => {
      const ctx = result.current.createContext("Original Name");
      contextId = ctx.id;
    });

    act(() => {
      result.current.updateContext(contextId!, { name: "Updated Name" });
    });

    expect(result.current.contexts[0].name).toBe("Updated Name");
  });

  it("should delete a context", async () => {
    const { result } = renderHook(() => useContexts(), { wrapper });

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    let contextId: string;
    act(() => {
      const ctx = result.current.createContext("To Delete");
      contextId = ctx.id;
    });

    expect(result.current.contexts).toHaveLength(1);

    act(() => {
      result.current.deleteContext(contextId!);
    });

    expect(result.current.contexts).toHaveLength(0);
  });

  it("should set active context", async () => {
    const { result } = renderHook(() => useContexts(), { wrapper });

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    let contextId = "";
    act(() => {
      const ctx = result.current.createContext("Active Context");
      contextId = ctx.id;
    });
    act(() => {
      result.current.setActiveContext(contextId!);
    });

    expect(result.current.activeContext).not.toBeNull();
    expect(result.current.activeContext?.id).toBe(contextId);
  });

  it("should add repo to context", async () => {
    const { result } = renderHook(() => useContexts(), { wrapper });

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    let contextId: string;
    act(() => {
      const ctx = result.current.createContext("With Repos");
      contextId = ctx.id;
    });

    act(() => {
      result.current.addRepo(contextId!, 1, "test/repo");
    });

    expect(result.current.contexts[0].repos).toHaveLength(1);
    expect(result.current.contexts[0].repos[0].id).toBe(1);
    expect(result.current.contexts[0].repos[0].name).toBe("test/repo");
  });

  it("should remove repo from context", async () => {
    const { result } = renderHook(() => useContexts(), { wrapper });

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    let contextId: string;
    act(() => {
      const ctx = result.current.createContext("With Repos");
      contextId = ctx.id;
    });

    act(() => {
      result.current.addRepo(contextId!, 1, "test/repo");
      result.current.addRepo(contextId!, 2, "test/repo2");
    });

    expect(result.current.contexts[0].repos).toHaveLength(2);

    act(() => {
      result.current.removeRepo(contextId!, 1);
    });

    expect(result.current.contexts[0].repos).toHaveLength(1);
    expect(result.current.contexts[0].repos[0].id).toBe(2);
  });

  it("should clear active context when deleted", async () => {
    const { result } = renderHook(() => useContexts(), { wrapper });

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    let contextId: string;
    act(() => {
      const ctx = result.current.createContext("Active To Delete");
      contextId = ctx.id;
      result.current.setActiveContext(ctx.id);
    });

    expect(result.current.activeContext).not.toBeNull();

    act(() => {
      result.current.deleteContext(contextId!);
    });

    expect(result.current.activeContext).toBeNull();
  });

  it("should provide getNextColor", async () => {
    const { result } = renderHook(() => useContexts(), { wrapper });

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    const color = result.current.getNextColor();
    expect(color).toBeDefined();
    expect(color).toMatch(/^#[0-9A-F]{6}$/i);
  });
});

describe("useActiveContext", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    Object.keys(mockLocalStorage).forEach(
      (key) => delete mockLocalStorage[key]
    );
    vi.mocked(windowStorage.getItem).mockReturnValue(null);
  });

  it("should return active context utilities", async () => {
    const { result } = renderHook(() => useActiveContext(), { wrapper });

    await waitFor(() => {
      expect(result.current.activeContext).toBeNull();
    });

    expect(typeof result.current.isRepoInActiveContext).toBe("function");
    expect(typeof result.current.getRepoFilter).toBe("function");
  });

  it("should check if repo is in active context", async () => {
    const { result } = renderHook(() => useContexts(), { wrapper });

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    // Create and activate a context with a repo
    let _contextId: string;
    act(() => {
      const ctx = result.current.createContext("Test Context");
      _contextId = ctx.id;
      result.current.addRepo(ctx.id, 1, "test/repo");
      result.current.setActiveContext(ctx.id);
    });

    // Now check if the repo is in active context
    expect(result.current.isRepoInActiveContext(1)).toBe(true);
    expect(result.current.isRepoInActiveContext(999)).toBe(false);
  });

  it("should return null filter when no active context", async () => {
    const { result } = renderHook(() => useActiveContext(), { wrapper });

    await waitFor(() => {
      expect(result.current.activeContext).toBeNull();
    });

    const filter = result.current.getRepoFilter();
    expect(filter).toBeNull();
  });
});
