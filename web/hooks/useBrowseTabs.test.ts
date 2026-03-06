import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { renderHook, act } from "@testing-library/react";

// Mock windowStorage before importing useBrowseTabs
vi.mock("@/lib/windowStorage", () => ({
  windowStorage: {
    getItem: vi.fn(() => null),
    setItem: vi.fn(),
    removeItem: vi.fn(),
  },
}));

// Import after mocking
import { useBrowseTabs } from "./useBrowseTabs";
import { windowStorage } from "@/lib/windowStorage";

describe("useBrowseTabs", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    // Reset mock to return empty state
    vi.mocked(windowStorage.getItem).mockReturnValue(null);
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it("should initialize with empty tabs", () => {
    const { result } = renderHook(() => useBrowseTabs(1, "", "main"));

    expect(result.current.tabs).toEqual([]);
    expect(result.current.activeTabId).toBeNull();
  });

  it("should add a tab when a file path is provided", () => {
    const { result } = renderHook(() =>
      useBrowseTabs(1, "src/main.go", "main")
    );

    expect(result.current.tabs).toHaveLength(1);
    expect(result.current.tabs[0].filePath).toBe("src/main.go");
    expect(result.current.tabs[0].repoId).toBe(1);
    expect(result.current.activeTabId).toBe(result.current.tabs[0].id);
  });

  it("should not add duplicate tabs for same file", () => {
    const { result, rerender } = renderHook(
      ({ repoId, path, ref }) => useBrowseTabs(repoId, path, ref),
      { initialProps: { repoId: 1, path: "src/main.go", ref: "main" } }
    );

    expect(result.current.tabs).toHaveLength(1);
    const firstTabId = result.current.tabs[0].id;

    // Re-render with same props
    rerender({ repoId: 1, path: "src/main.go", ref: "main" });

    expect(result.current.tabs).toHaveLength(1);
    expect(result.current.tabs[0].id).toBe(firstTabId);
  });

  it("should open new tabs with openTab", () => {
    const { result } = renderHook(() => useBrowseTabs(1, "", "main"));

    act(() => {
      result.current.openTab(
        1,
        "test/repo",
        "src/file.ts",
        "main",
        "TypeScript"
      );
    });

    expect(result.current.tabs).toHaveLength(1);
    expect(result.current.tabs[0].filePath).toBe("src/file.ts");
    expect(result.current.tabs[0].repoName).toBe("test/repo");
    expect(result.current.tabs[0].language).toBe("TypeScript");
  });

  it("should close tabs with closeTab", () => {
    const { result } = renderHook(() =>
      useBrowseTabs(1, "src/main.go", "main")
    );

    expect(result.current.tabs).toHaveLength(1);
    const tabId = result.current.tabs[0].id;

    act(() => {
      result.current.closeTab(tabId);
    });

    expect(result.current.tabs).toHaveLength(0);
    expect(result.current.activeTabId).toBeNull();
  });

  it("should clear all tabs with clearTabs", () => {
    const { result } = renderHook(() => useBrowseTabs(1, "", "main"));

    // Open multiple tabs
    act(() => {
      result.current.openTab(1, "repo", "file1.ts", "main");
      result.current.openTab(1, "repo", "file2.ts", "main");
      result.current.openTab(1, "repo", "file3.ts", "main");
    });

    expect(result.current.tabs).toHaveLength(3);

    act(() => {
      result.current.clearTabs();
    });

    expect(result.current.tabs).toHaveLength(0);
    expect(result.current.activeTabId).toBeNull();
  });

  it("should set active tab with setActiveTab", () => {
    const { result } = renderHook(() => useBrowseTabs(1, "", "main"));

    act(() => {
      result.current.openTab(1, "repo", "file1.ts", "main");
      result.current.openTab(1, "repo", "file2.ts", "main");
    });

    const firstTabId = result.current.tabs[0].id;

    act(() => {
      result.current.setActiveTab(firstTabId);
    });

    expect(result.current.activeTabId).toBe(firstTabId);
  });

  it("should load state from storage on init", () => {
    const savedState = {
      tabs: [{ id: "tab_1", repoId: 1, repoName: "test", filePath: "file.ts" }],
      activeTabId: "tab_1",
    };
    vi.mocked(windowStorage.getItem).mockReturnValue(
      JSON.stringify(savedState)
    );

    const { result } = renderHook(() => useBrowseTabs(1, "", "main"));

    expect(result.current.tabs).toHaveLength(1);
    expect(result.current.tabs[0].id).toBe("tab_1");
    expect(result.current.activeTabId).toBe("tab_1");
  });

  it("should save state to storage on change", () => {
    const { result: _result } = renderHook(() =>
      useBrowseTabs(1, "file.ts", "main")
    );

    // State should be saved after tab is added
    expect(windowStorage.setItem).toHaveBeenCalled();
    const savedArg = vi.mocked(windowStorage.setItem).mock.calls[0];
    expect(savedArg[0]).toBe("browse-tabs");
  });

  it("should not add tab for empty path (directory view)", () => {
    const { result } = renderHook(() => useBrowseTabs(1, "", "main"));

    expect(result.current.tabs).toHaveLength(0);
  });

  it("should return activeTab property", () => {
    const { result } = renderHook(() => useBrowseTabs(1, "file.ts", "main"));

    expect(result.current.activeTab).toBeDefined();
    expect(result.current.activeTab?.filePath).toBe("file.ts");
  });
});
