"use client";

import { useState, useEffect, useCallback, useMemo, useRef, useReducer } from "react";
import { useParams, useRouter, useSearchParams } from "next/navigation";
import {
  api,
  Repository,
  TreeEntry,
  BlobResponse,
  RefsResponse,
  ActivePane,
  PaneFile,
  PaneTab,
  buildBrowseUrl,
  isImageFile,
  isPdfFile,
  isVideoFile,
  isAudioFile,
  getRawFileUrl,
} from "@/lib/api";
import dynamic from "next/dynamic";
import { SymbolSidebar } from "@/components/SymbolSidebar";
import { ReferencePanel } from "@/components/ReferencePanel";
import { QuickFilePicker } from "@/components/QuickFilePicker";
import { SearchDropdown } from "@/components/SearchDropdown";
import { LazyFileTree, getFileIcon } from "@/components/LazyFileTree";
import { BrowseTabBar, BrowseTab } from "@/components/BrowseTabBar";
import { useBrowseTabs } from "@/hooks/useBrowseTabs";
import { useActiveContext } from "@/hooks/useContexts";
import { windowStorage } from "@/lib/windowStorage";
import {
  ChevronRight,
  ChevronLeft,
  Folder,
  GitBranch,
  Tag,
  Loader2,
  AlertCircle,
  ArrowLeft,
  Home,
  FileCode,
  ExternalLink,
  Copy,
  Check,
  ListTree,
  PanelRightClose,
  PanelLeftClose,
  PanelLeftOpen,
  Keyboard,
  Search,
  X,
  FolderTree,
  EyeOff,
  FileImage,
  FileVideo,
  FileAudio,
  FileText,
} from "lucide-react";
import Link from "next/link";
import { useNavigationHistory } from "@/hooks/useNavigationHistory";
import Image from "next/image";

const CodeViewer = dynamic(() => import("@/components/CodeViewer"), {
  ssr: false,
  loading: () => (
    <div className="flex-1 flex items-center justify-center h-full">
      <Loader2 className="w-6 h-6 animate-spin text-gray-400" />
    </div>
  ),
});

// Breakpoint for responsive sidebar behavior
const MOBILE_BREAKPOINT = 768;
const TABLET_BREAKPOINT = 1024;

  // Split pane direction type
  type SplitDirection = "vertical" | "horizontal";

  // Split pane state
  interface SplitPaneState {

  enabled: boolean;
  direction: SplitDirection;
  ratio: number; // 0-1, percentage of first pane
  activePane: ActivePane; // Which pane is currently selected
  primaryTabs: PaneTab[]; // Tabs in primary pane
  primaryActiveTabId: string | null;
  secondaryTabs: PaneTab[]; // Tabs in secondary pane
  secondaryActiveTabId: string | null;
}

// Helper to generate tab ID
function generateTabId(file: PaneFile): string {
  return `${file.repoId}:${file.path}:${file.ref || ""}`;
}

function mergeReducer<T>(state: T, update: Partial<T>): T {
  return { ...state, ...update };
}

export default function BrowseClient() {
  const params = useParams();
  const router = useRouter();
  const searchParams = useSearchParams();

  // Extract params - decode URL-encoded path segments
  const repoId = Number(params.id);
  const pathSegments = params.path as string[] | undefined;
  // Decode each segment to handle special characters like + (encoded as %2B)
  const currentPath = pathSegments?.map(segment => decodeURIComponent(segment)).join("/") || "";
  const ref = searchParams.get("ref") || "";

  // Parse line number from ?line= query param
  const lineFromQuery = useMemo(() => {
    const lineParam = searchParams.get("line");
    if (lineParam) {
      return Number(lineParam);
    }
    return undefined;
  }, [searchParams]);

  // Parse line number from #L{line} hash (GitHub style) - needs useEffect for client-side
  const [lineFromHash, setLineFromHash] = useState<number | undefined>(undefined);
  useEffect(() => {
    const hash = window.location.hash;
    const match = hash.match(/^#L(\d+)$/);
    if (match) {
      setLineFromHash(Number(match[1]));
    } else {
      setLineFromHash(undefined);
    }
  }, [currentPath]); // Re-check when path changes

  const highlightLine = lineFromQuery ?? lineFromHash;

  // State for access control
  const [settings, setSettings] = useState({ browseDisabled: false, settingsLoaded: false });
  const { browseDisabled, settingsLoaded } = settings;

  // State - browse data (grouped via useReducer with merge pattern)
  const [browseData, updateBrowse] = useReducer(mergeReducer, {
    repo: null as Repository | null,
    entries: [] as TreeEntry[],
    blob: null as BlobResponse | null,
    refs: null as RefsResponse | null,
    displayRepo: null as Repository | null,
    displayRefs: null as RefsResponse | null,
    loading: true,
    error: null as string | null,
    isFile: false,
  });
  const { repo, entries, blob, refs, displayRepo, displayRefs, loading, error, isFile } = browseData;
  const [scrollToLine, setScrollToLine] = useState<number | undefined>(highlightLine);
  // Scroll state for split panes (use object with counter to force re-render on same line click)
  const [primaryScrollToLine, setPrimaryScrollToLine] = useState<{ line: number; key: number } | null>(null);
  const [secondaryScrollToLine, setSecondaryScrollToLine] = useState<{ line: number; key: number } | null>(null);
  const scrollKeyRef = useRef(0);
  // Pending scroll line for after navigation (applied when content loads)
  const pendingScrollRef = useRef<number | null>(null);
  const [referenceSearch, setReferenceSearch] = useState<{
    symbolName: string;
    language?: string;
    line?: number;
    col?: number;
    filePath?: string;
    repoId?: number;
  } | null>(null);
  // UI state (grouped via useReducer with merge pattern)
  const [uiState, updateUI] = useReducer(mergeReducer, {
    showFileTree: true,
    showSymbols: true,
    isMobile: false,
    _isTablet: false,
    copied: false,
    showQuickPicker: false,
    showShortcutsHelp: false,
  });
  const { showFileTree, showSymbols, isMobile, _isTablet, copied, showQuickPicker, showShortcutsHelp } = uiState;
  const [fileTreeWidth, setFileTreeWidth] = useState(280); // Default width in pixels
  const [symbolsWidth, setSymbolsWidth] = useState(224); // Default 56 * 4 = 224px (w-56)
  const [referencePanelHeight, setReferencePanelHeight] = useState(256); // Default h-64 = 256px
  const isResizing = useRef(false);
  const isResizingSymbols = useRef(false);
  const isResizingReferences = useRef(false);
  const fileTreeRef = useRef<HTMLDivElement>(null);
  const symbolsRef = useRef<HTMLDivElement>(null);

  // Split pane state - persisted per-window using windowStorage
  // Each browser tab/window has its own isolated split state
  const [splitPane, setSplitPane] = useState<SplitPaneState>(() => {
    // Initialize from windowStorage on mount
    if (typeof window !== "undefined") {
      const saved = windowStorage.getItem("split-state");
      if (saved) {
        try {
          const parsed = JSON.parse(saved);
          // Validate that it has the expected structure
          if (parsed && Array.isArray(parsed.primaryTabs) && Array.isArray(parsed.secondaryTabs)) {
            return parsed;
          }
        } catch {
          // Ignore invalid JSON in localStorage
        }
      }
    }
    return {
      enabled: false,
      direction: "vertical" as SplitDirection,
      ratio: 0.5,
      activePane: "primary" as ActivePane,
      primaryTabs: [],
      primaryActiveTabId: null,
      secondaryTabs: [],
      secondaryActiveTabId: null,
    };
  });

  // Persist split state to windowStorage
  useEffect(() => {
    windowStorage.setItem("split-state", JSON.stringify(splitPane));
  }, [splitPane]);

  // Get active file for each pane
  const primaryActiveFile = useMemo(() => {
    if (!splitPane.enabled) return null;
    const tab = splitPane.primaryTabs.find(t => t.id === splitPane.primaryActiveTabId);
    return tab?.file || null;
  }, [splitPane.enabled, splitPane.primaryTabs, splitPane.primaryActiveTabId]);

  const secondaryActiveFile = useMemo(() => {
    if (!splitPane.enabled) return null;
    const tab = splitPane.secondaryTabs.find(t => t.id === splitPane.secondaryActiveTabId);
    return tab?.file || null;
  }, [splitPane.enabled, splitPane.secondaryTabs, splitPane.secondaryActiveTabId]);

  // Compute the active file for the symbols sidebar based on split pane state
  const symbolsFile = useMemo(() => {
    if (!splitPane.enabled) {
      // No split - use URL path
      return { repoId, path: currentPath, ref: ref || undefined };
    }
    // Split enabled - use active pane's file
    if (splitPane.activePane === "primary" && primaryActiveFile) {
      return primaryActiveFile;
    }
    if (splitPane.activePane === "secondary" && secondaryActiveFile) {
      return secondaryActiveFile;
    }
    // Fallback to URL path
    return { repoId, path: currentPath, ref: ref || undefined };
  }, [splitPane.enabled, splitPane.activePane, primaryActiveFile, secondaryActiveFile, repoId, currentPath, ref]);

  // Blobs for split panes (when files differ from URL)
  const [splitBlobs, setSplitBlobs] = useState({
    primaryBlob: null as BlobResponse | null,
    primaryBlobLoading: false,
    secondaryBlob: null as BlobResponse | null,
    secondaryBlobLoading: false,
  });
  const { primaryBlob, primaryBlobLoading, secondaryBlob, secondaryBlobLoading } = splitBlobs;
  const splitResizing = useRef(false);
  const splitContainerRef = useRef<HTMLDivElement>(null);

  // Get the blob for the symbols sidebar (after primaryBlob/secondaryBlob are declared)
  const symbolsBlob = useMemo(() => {
    if (!splitPane.enabled) return blob;
    if (splitPane.activePane === "primary" && primaryBlob) return primaryBlob;
    if (splitPane.activePane === "secondary" && secondaryBlob) return secondaryBlob;
    return blob;
  }, [splitPane.enabled, splitPane.activePane, primaryBlob, secondaryBlob, blob]);

  // Context for filtering
  const { activeContext } = useActiveContext();

  // Navigation history for back/forward
  const navHistory = useNavigationHistory();

  // Check if browse API is disabled
  useEffect(() => {
    api.getUISettings()
      .then(uiSettings => {
        setSettings({ browseDisabled: uiSettings.disable_browse_api, settingsLoaded: true });
      })
      .catch(() => {
        // If settings fail to load, allow access (API might just be down)
        setSettings(prev => ({ ...prev, settingsLoaded: true }));
      });
  }, []);

  // Tab management for multi-file browsing
  const {
    tabs,
    activeTab: _activeTab,
    activeTabId,
    openTab: _openTab,
    closeTab,
    setActiveTab: _setActiveTab,
    updateTabRepoName,
    replaceTabs,
  } = useBrowseTabs(repoId, currentPath, ref || undefined);

  // Handle responsive sidebar defaults
  useEffect(() => {
    const handleResize = () => {
      const width = window.innerWidth;
      const mobile = width < MOBILE_BREAKPOINT;
      const tablet = width >= MOBILE_BREAKPOINT && width < TABLET_BREAKPOINT;

      if (mobile) {
        updateUI({ isMobile: mobile, _isTablet: tablet, showFileTree: false, showSymbols: false });
      } else if (tablet) {
        updateUI({ isMobile: mobile, _isTablet: tablet, showFileTree: false, showSymbols: false });
      } else {
        updateUI({ isMobile: mobile, _isTablet: tablet });
      }
    };

    // Initial check
    handleResize();

    window.addEventListener("resize", handleResize);
    return () => window.removeEventListener("resize", handleResize);
  }, []);

  // Handle sidebar resize with mouse drag - event listeners attached on mousedown
  const startResizing = useCallback(() => {
    isResizing.current = true;
    document.body.style.cursor = 'col-resize';
    document.body.style.userSelect = 'none';

    const handleMouseMove = (e: MouseEvent) => {
      const newWidth = e.clientX;
      const clampedWidth = Math.max(200, Math.min(500, newWidth));
      setFileTreeWidth(clampedWidth);
    };

    const handleMouseUp = () => {
      isResizing.current = false;
      document.body.style.cursor = '';
      document.body.style.userSelect = '';
      document.removeEventListener('mousemove', handleMouseMove);
      document.removeEventListener('mouseup', handleMouseUp);
    };

    document.addEventListener('mousemove', handleMouseMove);
    document.addEventListener('mouseup', handleMouseUp);
  }, []);

  // Split pane resize handling
  useEffect(() => {
    const handleSplitMouseMove = (e: MouseEvent) => {
      if (!splitResizing.current || !splitContainerRef.current) return;

      const rect = splitContainerRef.current.getBoundingClientRect();
      let ratio: number;

      if (splitPane.direction === "vertical") {
        ratio = (e.clientX - rect.left) / rect.width;
      } else {
        ratio = (e.clientY - rect.top) / rect.height;
      }

      setSplitPane((prev) => ({
        ...prev,
        ratio: Math.max(0.2, Math.min(0.8, ratio)),
      }));
    };

    const handleSplitMouseUp = () => {
      splitResizing.current = false;
      document.body.style.cursor = '';
      document.body.style.userSelect = '';
    };

    if (splitPane.enabled) {
      document.addEventListener('mousemove', handleSplitMouseMove);
      document.addEventListener('mouseup', handleSplitMouseUp);
    }

    return () => {
      document.removeEventListener('mousemove', handleSplitMouseMove);
      document.removeEventListener('mouseup', handleSplitMouseUp);
    };
  }, [splitPane.enabled, splitPane.direction]);

  const startSplitResizing = useCallback(() => {
    splitResizing.current = true;
    document.body.style.cursor = splitPane.direction === 'vertical' ? 'col-resize' : 'row-resize';
    document.body.style.userSelect = 'none';
  }, [splitPane.direction]);

  // Handle symbols sidebar resize with mouse drag
  useEffect(() => {
    const handleMouseMove = (e: MouseEvent) => {
      if (!isResizingSymbols.current) return;

      // Calculate new width from right edge of window
      const newWidth = window.innerWidth - e.clientX;
      // Clamp between min and max width
      const clampedWidth = Math.max(150, Math.min(400, newWidth));
      setSymbolsWidth(clampedWidth);
    };

    const handleMouseUp = () => {
      isResizingSymbols.current = false;
      document.body.style.cursor = '';
      document.body.style.userSelect = '';
    };

    document.addEventListener('mousemove', handleMouseMove);
    document.addEventListener('mouseup', handleMouseUp);

    return () => {
      document.removeEventListener('mousemove', handleMouseMove);
      document.removeEventListener('mouseup', handleMouseUp);
    };
  }, []);

  const startResizingSymbols = useCallback(() => {
    isResizingSymbols.current = true;
    document.body.style.cursor = 'col-resize';
    document.body.style.userSelect = 'none';
  }, []);

  // Handle references panel resize with mouse drag
  useEffect(() => {
    const handleMouseMove = (e: MouseEvent) => {
      if (!isResizingReferences.current) return;

      // Calculate new height from bottom edge of window
      const newHeight = window.innerHeight - e.clientY;
      // Clamp between min and max height
      const clampedHeight = Math.max(150, Math.min(500, newHeight));
      setReferencePanelHeight(clampedHeight);
    };

    const handleMouseUp = () => {
      isResizingReferences.current = false;
      document.body.style.cursor = '';
      document.body.style.userSelect = '';
    };

    document.addEventListener('mousemove', handleMouseMove);
    document.addEventListener('mouseup', handleMouseUp);

    return () => {
      document.removeEventListener('mousemove', handleMouseMove);
      document.removeEventListener('mouseup', handleMouseUp);
    };
  }, []);

  const startResizingReferences = useCallback(() => {
    isResizingReferences.current = true;
    document.body.style.cursor = 'row-resize';
    document.body.style.userSelect = 'none';
  }, []);

  // Load primary blob when split is active and primaryActiveFile is set
  useEffect(() => {
    if (!splitPane.enabled || !primaryActiveFile) {
      setSplitBlobs(prev => ({ ...prev, primaryBlob: null }));
      return;
    }

    const loadPrimaryBlob = async () => {
      setSplitBlobs(prev => ({ ...prev, primaryBlobLoading: true }));
      try {
        const blobData = await api.getBlob(
          primaryActiveFile.repoId,
          primaryActiveFile.path,
          primaryActiveFile.ref
        );
        setSplitBlobs(prev => ({ ...prev, primaryBlob: blobData, primaryBlobLoading: false }));
      } catch (err) {
        console.error("Failed to load primary file:", err);
        setSplitBlobs(prev => ({ ...prev, primaryBlob: null, primaryBlobLoading: false }));
      }
    };

    loadPrimaryBlob();
  }, [splitPane.enabled, primaryActiveFile]);

  // Load secondary blob when split is active
  useEffect(() => {
    if (!splitPane.enabled || !secondaryActiveFile) {
      setSplitBlobs(prev => ({ ...prev, secondaryBlob: null }));
      return;
    }

    const loadSecondaryBlob = async () => {
      setSplitBlobs(prev => ({ ...prev, secondaryBlobLoading: true }));
      try {
        const blobData = await api.getBlob(
          secondaryActiveFile.repoId,
          secondaryActiveFile.path,
          secondaryActiveFile.ref
        );
        setSplitBlobs(prev => ({ ...prev, secondaryBlob: blobData, secondaryBlobLoading: false }));
      } catch (err) {
        console.error("Failed to load secondary file:", err);
        setSplitBlobs(prev => ({ ...prev, secondaryBlob: null, secondaryBlobLoading: false }));
      }
    };

    loadSecondaryBlob();
  }, [splitPane.enabled, secondaryActiveFile]);

  // Split handlers
  const handleSplitVertical = useCallback(() => {
    if (isFile && blob) {
      const currentFile: PaneFile = { repoId, path: currentPath, ref: ref || undefined };
      const currentTabId = generateTabId(currentFile);
      const currentTab: PaneTab = { id: currentTabId, file: currentFile };

      // Convert existing tabs to pane tabs for the primary pane
      const primaryPaneTabs: PaneTab[] = tabs.map(tab => ({
        id: tab.id,
        file: { repoId: tab.repoId, path: tab.filePath, ref: tab.ref }
      }));

      // Check if current file is already in tabs
      const existingTab = primaryPaneTabs.find(t =>
        t.file.repoId === repoId && t.file.path === currentPath
      );

      // Keep existing tabs in primary, add current file if not already there
      const finalPrimaryTabs = existingTab ? primaryPaneTabs : [...primaryPaneTabs, currentTab];
      const primaryActiveId = existingTab ? existingTab.id : currentTabId;

      // Secondary pane starts with only the current file
      setSplitPane({
        enabled: true,
        direction: "vertical",
        ratio: 0.5,
        activePane: "secondary",
        primaryTabs: finalPrimaryTabs,
        primaryActiveTabId: primaryActiveId,
        secondaryTabs: [currentTab],
        secondaryActiveTabId: currentTabId,
      });
    }
  }, [isFile, blob, repoId, currentPath, ref, tabs]);

  const handleSplitHorizontal = useCallback(() => {
    if (isFile && blob) {
      const currentFile: PaneFile = { repoId, path: currentPath, ref: ref || undefined };
      const currentTabId = generateTabId(currentFile);
      const currentTab: PaneTab = { id: currentTabId, file: currentFile };

      // Convert existing tabs to pane tabs for the primary pane
      const primaryPaneTabs: PaneTab[] = tabs.map(tab => ({
        id: tab.id,
        file: { repoId: tab.repoId, path: tab.filePath, ref: tab.ref }
      }));

      // Check if current file is already in tabs
      const existingTab = primaryPaneTabs.find(t =>
        t.file.repoId === repoId && t.file.path === currentPath
      );

      // Keep existing tabs in primary, add current file if not already there
      const finalPrimaryTabs = existingTab ? primaryPaneTabs : [...primaryPaneTabs, currentTab];
      const primaryActiveId = existingTab ? existingTab.id : currentTabId;

      // Secondary pane starts with only the current file
      setSplitPane({
        enabled: true,
        direction: "horizontal",
        ratio: 0.5,
        activePane: "secondary",
        primaryTabs: finalPrimaryTabs,
        primaryActiveTabId: primaryActiveId,
        secondaryTabs: [currentTab],
        secondaryActiveTabId: currentTabId,
      });
    }
  }, [isFile, blob, repoId, currentPath, ref, tabs]);

  // Close split and keep the tabs from the pane that's NOT being closed
  const handleCloseSplit = useCallback((closingPane: "primary" | "secondary") => {
    // Get the tabs from the pane that will remain
    const remainingPane = closingPane === "primary" ? "secondary" : "primary";
    const remainingTabs = remainingPane === "primary" ? splitPane.primaryTabs : splitPane.secondaryTabs;
    const remainingActiveTabId = remainingPane === "primary" ? splitPane.primaryActiveTabId : splitPane.secondaryActiveTabId;

    // Find the active file in the remaining pane
    const activeTab = remainingTabs.find(t => t.id === remainingActiveTabId);

    // Convert split pane tabs to regular tabs
    const newTabs: BrowseTab[] = remainingTabs.map(paneTab => ({
      id: paneTab.id,
      repoId: paneTab.file.repoId,
      repoName: repo?.name || "",
      filePath: paneTab.file.path,
      ref: paneTab.file.ref,
    }));

    // Reset split state
    setSplitPane({
      enabled: false,
      direction: "vertical",
      ratio: 0.5,
      activePane: "primary",
      primaryTabs: [],
      primaryActiveTabId: null,
      secondaryTabs: [],
      secondaryActiveTabId: null,
    });
    setSplitBlobs({ primaryBlob: null, primaryBlobLoading: false, secondaryBlob: null, secondaryBlobLoading: false });

    // Update the main tabs with the remaining pane's tabs
    replaceTabs(newTabs, remainingActiveTabId);

    // Navigate to the active file from the remaining pane
    if (activeTab) {
      router.push(buildBrowseUrl(activeTab.file.repoId, activeTab.file.path, { ref: activeTab.file.ref }));
    } else if (remainingTabs.length > 0) {
      // No active tab, navigate to the first tab
      const firstTab = remainingTabs[0];
      router.push(buildBrowseUrl(firstTab.file.repoId, firstTab.file.path, { ref: firstTab.file.ref }));
    }
  }, [splitPane, router, repo?.name, replaceTabs]);

  // Set active pane when clicking on it and update URL
  const handleSetActivePane = useCallback((pane: ActivePane) => {
    setSplitPane((prev) => {
      return { ...prev, activePane: pane };
    });
  }, []);

  // Update URL when active pane or active tab changes
  useEffect(() => {
    if (typeof window === 'undefined') return;

    const tabs = splitPane.activePane === "primary" ? splitPane.primaryTabs : splitPane.secondaryTabs;
    const activeTabId = splitPane.activePane === "primary" ? splitPane.primaryActiveTabId : splitPane.secondaryActiveTabId;
    const activeTab = tabs.find(t => t.id === activeTabId);

    if (activeTab) {
      const file = activeTab.file;
      const newUrl = buildBrowseUrl(file.repoId, file.path) + window.location.hash;
      // Only update if URL is different to avoid unnecessary history entries
      if (window.location.pathname + window.location.hash !== newUrl) {
        window.history.replaceState(null, '', newUrl);
      }
    }
  }, [splitPane.activePane, splitPane.primaryActiveTabId, splitPane.secondaryActiveTabId, splitPane.primaryTabs, splitPane.secondaryTabs]);

  // Open file in a specific pane's tabs
  const openFileInPane = useCallback((pane: ActivePane, file: PaneFile) => {
    const tabId = generateTabId(file);
    setSplitPane((prev) => {
      const tabsKey = pane === "primary" ? "primaryTabs" : "secondaryTabs";
      const activeTabKey = pane === "primary" ? "primaryActiveTabId" : "secondaryActiveTabId";
      const existingTabs = prev[tabsKey];
      const existingTab = existingTabs.find(t => t.id === tabId);

      if (existingTab) {
        // Tab already exists, just make it active
        return { ...prev, [activeTabKey]: tabId, activePane: pane };
      } else {
        // Add new tab
        const newTab: PaneTab = { id: tabId, file };
        return {
          ...prev,
          [tabsKey]: [...existingTabs, newTab],
          [activeTabKey]: tabId,
          activePane: pane,
        };
      }
    });
  }, []);

  // Close tab in a specific pane
  const closeTabInPane = useCallback((pane: ActivePane, tabId: string) => {
    setSplitPane((prev) => {
      const tabsKey = pane === "primary" ? "primaryTabs" : "secondaryTabs";
      const activeTabKey = pane === "primary" ? "primaryActiveTabId" : "secondaryActiveTabId";
      const tabs = prev[tabsKey];
      const newTabs = tabs.filter(t => t.id !== tabId);

      // If closing the active tab, select another one
      let newActiveTabId = prev[activeTabKey];
      if (newActiveTabId === tabId) {
        const closedIndex = tabs.findIndex(t => t.id === tabId);
        const newActiveTab = newTabs[Math.min(closedIndex, newTabs.length - 1)];
        newActiveTabId = newActiveTab?.id || null;
      }

      return {
        ...prev,
        [tabsKey]: newTabs,
        [activeTabKey]: newActiveTabId,
      };
    });
  }, []);

  // Select tab in a specific pane
  const selectTabInPane = useCallback((pane: ActivePane, tabId: string) => {
    setSplitPane((prev) => {
      const activeTabKey = pane === "primary" ? "primaryActiveTabId" : "secondaryActiveTabId";
      return { ...prev, [activeTabKey]: tabId, activePane: pane };
    });
  }, []);

  // Sync scrollToLine with URL parameter when navigating
  useEffect(() => {
    if (highlightLine !== undefined) {
      setScrollToLine(highlightLine);
      // Also set primary scroll if split pane is enabled
      if (splitPane.enabled) {
        scrollKeyRef.current += 1;
        setPrimaryScrollToLine({ line: highlightLine, key: scrollKeyRef.current });
      }
    }
  }, [highlightLine, splitPane.enabled]);

  // Update URL when active file changes in split pane mode (without ?line=)
  useEffect(() => {
    if (!splitPane.enabled) return;

    const activeFile = splitPane.activePane === "primary" ? primaryActiveFile : secondaryActiveFile;
    if (!activeFile) return;

    // Only update if the path is different from current URL
    if (activeFile.repoId === repoId && activeFile.path === currentPath) return;

    // Build the URL without ?line= - we don't want line in URL during browsing
    const url = buildBrowseUrl(activeFile.repoId, activeFile.path, { ref: activeFile.ref });

    // Use replaceState to update URL without adding history entry
    window.history.replaceState(null, '', url);
  }, [splitPane.enabled, splitPane.activePane, primaryActiveFile, secondaryActiveFile, repoId, currentPath]);

  // Compute the display path based on active pane when in split mode
  const displayPath = useMemo(() => {
    if (!splitPane.enabled) {
      return currentPath;
    }
    // Use active pane's file path
    if (splitPane.activePane === "primary" && primaryActiveFile) {
      return primaryActiveFile.path;
    }
    if (splitPane.activePane === "secondary" && secondaryActiveFile) {
      return secondaryActiveFile.path;
    }
    return currentPath;
  }, [splitPane.enabled, splitPane.activePane, primaryActiveFile, secondaryActiveFile, currentPath]);

  // Compute the display repo ID for tree and breadcrumbs when in split mode
  const displayRepoId = useMemo(() => {
    if (!splitPane.enabled) {
      return repoId;
    }
    if (splitPane.activePane === "primary" && primaryActiveFile) {
      return primaryActiveFile.repoId;
    }
    if (splitPane.activePane === "secondary" && secondaryActiveFile) {
      return secondaryActiveFile.repoId;
    }
    return repoId;
  }, [splitPane.enabled, splitPane.activePane, primaryActiveFile, secondaryActiveFile, repoId]);

  // Compute the display ref for the active pane
  const displayRef = useMemo(() => {
    if (!splitPane.enabled) {
      return ref;
    }
    if (splitPane.activePane === "primary" && primaryActiveFile) {
      return primaryActiveFile.ref || "";
    }
    if (splitPane.activePane === "secondary" && secondaryActiveFile) {
      return secondaryActiveFile.ref || "";
    }
    return ref;
  }, [splitPane.enabled, splitPane.activePane, primaryActiveFile, secondaryActiveFile, ref]);

  // Current ref to display (use displayRef if available, fallback to default branch)
  const currentDisplayRef = displayRef || displayRepo?.default_branch || displayRefs?.default_branch || "HEAD";

  // Breadcrumb path parts
  const pathParts = useMemo(() => {
    if (!displayPath) return [];
    return displayPath.split("/").filter(Boolean);
  }, [displayPath]);

  // Load repository info
  useEffect(() => {
    const loadRepo = async () => {
      try {
        const repoData = await api.getRepo(repoId);
        updateBrowse({ repo: repoData });
        // Update tab with repo name
        if (activeTabId) {
          updateTabRepoName(activeTabId, repoData.name);
        }
      } catch (err) {
        updateBrowse({ error: err instanceof Error ? err.message : "Failed to load repository" });
      }
    };
    loadRepo();
  }, [repoId, activeTabId, updateTabRepoName]);

  // Load refs (branches and tags)
  useEffect(() => {
    const loadRefs = async () => {
      try {
        const refsData = await api.getRefs(repoId);
        updateBrowse({ refs: refsData });
      } catch (err) {
        // Non-fatal - refs dropdown just won't work
        console.error("Failed to load refs:", err);
      }
    };
    loadRefs();
  }, [repoId]);

  // Load display repo and refs when displayRepoId changes (for split pane with different repos)
  useEffect(() => {
    if (displayRepoId === repoId) {
      // Same repo as URL, use existing data
      updateBrowse({ displayRepo: repo, displayRefs: refs });
      return;
    }

    // Different repo in active pane, load its data
    const loadDisplayRepoData = async () => {
      try {
        const [repoData, refsData] = await Promise.all([
          api.getRepo(displayRepoId),
          api.getRefs(displayRepoId).catch(() => null),
        ]);
        updateBrowse({ displayRepo: repoData, displayRefs: refsData });
      } catch (err) {
        console.error("Failed to load display repo:", err);
        // Fall back to URL repo
        updateBrowse({ displayRepo: repo, displayRefs: refs });
      }
    };
    loadDisplayRepoData();
  }, [displayRepoId, repoId, repo, refs]);

  // Load tree or blob content
  const loadContent = useCallback(async () => {
    updateBrowse({ loading: true, error: null, blob: null, isFile: false });

    try {
      // First try to load as directory
      const treeData = await api.getTree(
        repoId,
        currentPath || undefined,
        ref || undefined
      );
      updateBrowse({ entries: treeData.entries, isFile: false, loading: false });
    } catch {
      // If tree fails, try to load as file
      try {
        const blobData = await api.getBlob(
          repoId,
          currentPath,
          ref || undefined
        );
        updateBrowse({ blob: blobData, isFile: true, entries: [], loading: false });
      } catch (blobErr) {
        updateBrowse({
          error: blobErr instanceof Error
            ? blobErr.message
            : "Failed to load content",
          loading: false,
        });
      }
    }
  }, [repoId, currentPath, ref]);

  useEffect(() => {
    if (repoId) {
      loadContent();
    }
  }, [loadContent, repoId]);

  // When navigating from external sources (e.g., search results) with split view enabled,
  // open the file in the currently active pane (or primary if no pane is active)
  useEffect(() => {
    if (!splitPane.enabled || !isFile || !currentPath) return;

    const urlFile: PaneFile = { repoId, path: currentPath, ref: ref || undefined };
    const urlTabId = generateTabId(urlFile);

    // Check if this file is already the active tab in either pane
    if (splitPane.primaryActiveTabId === urlTabId || splitPane.secondaryActiveTabId === urlTabId) return;

    // Check if this file already exists in either pane
    const existsInPrimary = splitPane.primaryTabs.some(t => t.id === urlTabId);
    const existsInSecondary = splitPane.secondaryTabs.some(t => t.id === urlTabId);

    // Determine which pane to use - respect the current active pane
    const targetPane = splitPane.activePane || "primary";

    setSplitPane((prev) => {
      if (targetPane === "secondary") {
        // Open in secondary pane
        if (existsInSecondary) {
          return { ...prev, secondaryActiveTabId: urlTabId };
        } else if (existsInPrimary) {
          // File exists in primary, switch to primary
          return { ...prev, primaryActiveTabId: urlTabId, activePane: "primary" };
        } else {
          // Add new tab to secondary pane
          const newTab: PaneTab = { id: urlTabId, file: urlFile };
          return {
            ...prev,
            secondaryTabs: [...prev.secondaryTabs, newTab],
            secondaryActiveTabId: urlTabId,
          };
        }
      } else {
        // Open in primary pane (default)
        if (existsInPrimary) {
          return { ...prev, primaryActiveTabId: urlTabId };
        } else if (existsInSecondary) {
          // File exists in secondary, switch to secondary
          return { ...prev, secondaryActiveTabId: urlTabId, activePane: "secondary" };
        } else {
          // Add new tab to primary pane
          const newTab: PaneTab = { id: urlTabId, file: urlFile };
          return {
            ...prev,
            primaryTabs: [...prev.primaryTabs, newTab],
            primaryActiveTabId: urlTabId,
          };
        }
      }
    });
    // eslint-disable-next-line react-hooks/exhaustive-deps -- splitPane state excluded to prevent loops
  }, [splitPane.enabled, isFile, repoId, currentPath, ref]);

  // Track navigation history when path changes
  useEffect(() => {
    if (repoId && !loading) {
      navHistory.push({
        repoId,
        path: currentPath,
        ref: ref || undefined,
        line: highlightLine,
      });
    }
  }, [repoId, currentPath, ref, highlightLine, loading, navHistory]);

  // Handle history navigation
  const handleHistoryBack = useCallback(() => {
    const entry = navHistory.goBack();
    if (entry) {
      router.push(buildBrowseUrl(entry.repoId, entry.path || "", { ref: entry.ref, line: entry.line }));
    }
  }, [navHistory, router]);

  const handleHistoryForward = useCallback(() => {
    const entry = navHistory.goForward();
    if (entry) {
      router.push(buildBrowseUrl(entry.repoId, entry.path || "", { ref: entry.ref, line: entry.line }));
    }
  }, [navHistory, router]);

  // Navigate to a path
  const navigateTo = useCallback(
    (path: string, _asFile: boolean = false) => {
      const url = buildBrowseUrl(repoId, path, { ref: ref || undefined });
      router.push(url);
    },
    [repoId, ref, router]
  );

  // Handle tree selection - if split is active, open in the active pane
  const handleTreeSelect = useCallback(
    (path: string, isFolder: boolean) => {
      if (isFolder) {
        // In split mode, don't navigate for folders (tree handles expand/collapse)
        if (splitPane.enabled) return;
        // Without split, navigate to folder
        navigateTo(path, false);
      } else if (splitPane.enabled) {
        // If split is active, open file in the active pane as a tab
        const newFile: PaneFile = { repoId, path, ref: ref || undefined };
        openFileInPane(splitPane.activePane, newFile);
      } else {
        // Normal navigation
        navigateTo(path, true);
      }
    },
    [navigateTo, splitPane.enabled, splitPane.activePane, repoId, ref, openFileInPane]
  );

  // Handle ref change
  const handleRefChange = useCallback(
    (newRef: string) => {
      router.push(buildBrowseUrl(repoId, currentPath, { ref: newRef }));
    },
    [repoId, currentPath, router]
  );

  // Copy file path
  const copyPath = useCallback(() => {
    navigator.clipboard.writeText(currentPath);
    updateUI({ copied: true });
    setTimeout(() => updateUI({ copied: false }), 2000);
  }, [currentPath]);

  // Handle find references - show panel with references from SCIP or Zoekt
  const handleFindReferences = useCallback((symbolName: string, line?: number, col?: number, fileRepoId?: number, filePath?: string, language?: string) => {
    console.log("handleFindReferences called:", { symbolName, line, col, fileRepoId, filePath, language });
    setReferenceSearch({
      symbolName,
      language: language || blob?.language,
      // Pass position info for SCIP lookup (API accepts 1-indexed lines)
      line: line,
      col: col,
      filePath: filePath || currentPath,
      repoId: fileRepoId ?? repoId,
    });
  }, [blob?.language, currentPath, repoId]);

  // Handle go to definition - try SCIP first, then fall back to Zoekt symbol search
  const handleGoToDefinition = useCallback(async (symbolName: string, line?: number, col?: number, fileRepoId?: number, filePath?: string, language?: string) => {
    const targetRepoId = fileRepoId || repoId;
    const targetFilePath = filePath || currentPath;
    const targetLanguage = language || blob?.language;

    try {
      // First try SCIP-based precise navigation
      if (line !== undefined && targetFilePath) {
        try {
          const scipResult = await api.getSCIPDefinition(
            targetRepoId,
            targetFilePath,
            line, // API accepts 1-indexed lines
            col ?? 0
          );

          if (scipResult.found && scipResult.definition) {
            const def = scipResult.definition;
            // Navigate within same repo
            router.push(
              buildBrowseUrl(targetRepoId, def.filePath, { line: def.startLine + 1 })
            );
            return;
          }

          // If external symbol, show info but can't navigate
          if (scipResult.external) {
            console.log("External symbol:", scipResult.symbol);
            // Fall through to regular search
          }
        } catch (scipErr) {
          // SCIP not available or error - fall through to regular search
          console.debug("SCIP lookup failed, falling back to symbol search:", scipErr);

        }
      }

      // Fall back to Zoekt-based symbol search
      const definitions = await api.findSymbols({
        name: symbolName,
        language: targetLanguage,
        limit: 1,
      });

      if (definitions.length > 0) {
        const def = definitions[0];
        // Look up repo ID and navigate
        const lookupResult = await api.lookupRepoByName(def.repo);
        if (lookupResult) {
          router.push(buildBrowseUrl(lookupResult.id, def.file, { line: def.line }));
        }
      } else {
        // No definition found - show references panel instead
        setReferenceSearch({ symbolName, language: blob?.language });
      }
    } catch (err) {
      console.error("Failed to go to definition:", err);
      // Fall back to showing references
      setReferenceSearch({ symbolName, language: blob?.language });
    }
  }, [router, repoId, currentPath, blob?.language]);

  // Handle reference navigation (navigate to a different file/repo)
  const handleReferenceNavigate = useCallback(async (repoName: string, file: string, line: number) => {
    // Try to look up the repo by name to get its ID
    try {
      const lookupResult = await api.lookupRepoByName(repoName);
      if (lookupResult) {
        const targetRepoId = lookupResult.id;

        if (splitPane.enabled) {
          // Open in active pane when split mode is enabled
          const newFile: PaneFile = { repoId: targetRepoId, path: file, ref: ref || undefined };
          openFileInPane(splitPane.activePane, newFile);
          // Set scroll to line for the target pane
          scrollKeyRef.current += 1;
          const scrollData = { line, key: scrollKeyRef.current };
          if (splitPane.activePane === "primary") {
            setPrimaryScrollToLine(scrollData);
          } else {
            setSecondaryScrollToLine(scrollData);
          }
        } else {
          // Check if it's the same file in the same repo
          if (targetRepoId === repoId && file === currentPath) {
            // Same file - just scroll to the line (no URL change needed)
            setScrollToLine(line);
          } else {
            // Different file or repo - use ?line= query param for reliable scroll
            // Query params are parsed via searchParams and synced to scrollToLine via useEffect
            router.push(buildBrowseUrl(targetRepoId, file, { line }));
          }
        }
      }
    } catch (err) {
      console.error("Failed to navigate to reference:", err);
    }
  }, [router, repoId, currentPath, splitPane.enabled, splitPane.activePane, ref, openFileInPane]);

  // Apply pending scroll when content (blob) loads
  useEffect(() => {
    if (blob && pendingScrollRef.current !== null) {
      const line = pendingScrollRef.current;
      pendingScrollRef.current = null;
      setScrollToLine(line);
    }
  }, [blob]);

  // Handle quick file picker selection
  const handleQuickPickerSelect = useCallback((path: string) => {
    if (splitPane.enabled) {
      // Open in active pane when split mode is enabled
      const newFile: PaneFile = { repoId, path, ref: ref || undefined };
      openFileInPane(splitPane.activePane, newFile);
    } else {
      navigateTo(path, true);
    }
  }, [navigateTo, splitPane.enabled, splitPane.activePane, repoId, ref, openFileInPane]);

  // Handle line number click - copy shareable link to clipboard and update URL
  const handleLineClick = useCallback((line: number, fileRepoId?: number, filePath?: string) => {
    const targetRepoId = fileRepoId ?? repoId;
    const targetPath = filePath ?? currentPath;
    const baseUrl = typeof window !== 'undefined' ? window.location.origin : '';
    const url = `${baseUrl}${buildBrowseUrl(targetRepoId, targetPath)}#L${line}`;

    // Update the URL to match the clicked file and line
    if (typeof window !== 'undefined') {
      const newUrl = `${buildBrowseUrl(targetRepoId, targetPath)}#L${line}`;
      window.history.replaceState(null, '', newUrl);
    }

    // Highlight the line in the correct pane ONLY
    if (splitPane.enabled) {
      // Determine which pane this file belongs to based on the file path
      const isInPrimaryPane = primaryActiveFile &&
        primaryActiveFile.repoId === targetRepoId &&
        primaryActiveFile.path === targetPath;

      const isInSecondaryPane = secondaryActiveFile &&
        secondaryActiveFile.repoId === targetRepoId &&
        secondaryActiveFile.path === targetPath;

      if (isInPrimaryPane) {
        scrollKeyRef.current += 1;
        setPrimaryScrollToLine({ line, key: scrollKeyRef.current });
        // Set primary as active pane
        setSplitPane(prev => ({ ...prev, activePane: 'primary' }));
      } else if (isInSecondaryPane) {
        scrollKeyRef.current += 1;
        setSecondaryScrollToLine({ line, key: scrollKeyRef.current });
        // Set secondary as active pane
        setSplitPane(prev => ({ ...prev, activePane: 'secondary' }));
      }
    } else {
      setScrollToLine(line);
    }

    // Copy to clipboard
    navigator.clipboard.writeText(url).then(() => {
      // Successfully copied
    }).catch((err) => {
      console.error('Failed to copy link:', err);
    });
  }, [repoId, currentPath, splitPane.enabled, primaryActiveFile, secondaryActiveFile]);  // Global keyboard shortcuts
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      // Cmd/Ctrl + P - Quick file picker
      if ((e.metaKey || e.ctrlKey) && e.key === "p" && !e.shiftKey) {
        e.preventDefault();
        updateUI({ showQuickPicker: true });
        return;
      }

      // Cmd/Ctrl + Shift + P - Show shortcuts help (VS Code style)
      if ((e.metaKey || e.ctrlKey) && e.key === "p" && e.shiftKey) {
        e.preventDefault();
        updateUI({ showShortcutsHelp: !showShortcutsHelp });
        return;
      }

      // Cmd/Ctrl + B - Toggle file tree sidebar
      if ((e.metaKey || e.ctrlKey) && e.key === "b" && !e.shiftKey) {
        e.preventDefault();
        updateUI({ showFileTree: !showFileTree });
        return;
      }

      // Cmd/Ctrl + [ - Go back in history
      if ((e.metaKey || e.ctrlKey) && e.key === "[" && !e.shiftKey) {
        e.preventDefault();
        handleHistoryBack();
        return;
      }

      // Cmd/Ctrl + ] - Go forward in history
      if ((e.metaKey || e.ctrlKey) && e.key === "]" && !e.shiftKey) {
        e.preventDefault();
        handleHistoryForward();
        return;
      }

      // Escape - Close any open panels
      if (e.key === "Escape") {
        if (showQuickPicker) {
          updateUI({ showQuickPicker: false });
          return;
        }
        if (showShortcutsHelp) {
          updateUI({ showShortcutsHelp: false });
          return;
        }
        if (referenceSearch) {
          setReferenceSearch(null);
          return;
        }
      }
    };

    document.addEventListener("keydown", handleKeyDown);
    return () => document.removeEventListener("keydown", handleKeyDown);
  }, [showQuickPicker, showShortcutsHelp, referenceSearch, showFileTree, handleHistoryBack, handleHistoryForward]);

  // Current ref display
  const currentRef = ref || repo?.default_branch || refs?.default_branch || "HEAD";

  // Get repo names from active context for reference filtering
  // Always include the current repo even if not in context
  const contextRepoNames = useMemo(() => {
    if (!activeContext || activeContext.repos.length === 0) return undefined;
    const contextNames = activeContext.repos.map((r) => r.name);
    // Include current repo if not already in context
    if (repo?.name && !contextNames.includes(repo.name)) {
      return [...contextNames, repo.name];
    }
    return contextNames;
  }, [activeContext, repo?.name]);

  // Show loading state while checking settings
  if (!settingsLoaded) {
    return (
      <div className="h-full flex items-center justify-center bg-gray-50 dark:bg-gray-900">
        <div className="flex items-center gap-3 text-gray-500 dark:text-gray-400">
          <Loader2 className="w-5 h-5 animate-spin" />
          <span>Loading...</span>
        </div>
      </div>
    );
  }

  // Block access when browse API is disabled
  if (browseDisabled) {
    return (
      <div className="h-full flex items-center justify-center bg-gray-50 dark:bg-gray-900">
        <div className="text-center max-w-md mx-auto p-8">
          <div className="w-16 h-16 mx-auto mb-4 rounded-full bg-gray-100 dark:bg-gray-800 flex items-center justify-center">
            <EyeOff className="w-8 h-8 text-gray-400 dark:text-gray-500" />
          </div>
          <h2 className="text-xl font-semibold text-gray-900 dark:text-white mb-2">
            File Browser Disabled
          </h2>
          <p className="text-gray-600 dark:text-gray-400 mb-6">
            The file browser has been disabled by your administrator.
            You can still use code search to find files and content.
          </p>
          <Link
            href="/"
            className="inline-flex items-center gap-2 px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white rounded-lg transition-colors"
          >
            <Home className="w-4 h-4" />
            Go to Search
          </Link>
        </div>
      </div>
    );
  }

  return (
    <div className="h-full flex flex-col overflow-hidden">
      {/* Top Bar with Search */}
      <div className="flex-shrink-0 border-b border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-800 px-3 py-1.5">
        <div className="flex items-center gap-3">
          {/* File tree toggle */}
          <button
            onClick={() => updateUI({ showFileTree: !showFileTree })}
            className={`p-1.5 rounded-lg transition-colors ${showFileTree
              ? "bg-blue-100 dark:bg-blue-900/30 text-blue-600 dark:text-blue-400"
              : "text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200 hover:bg-gray-100 dark:hover:bg-gray-700"
              }`}
            title={showFileTree ? "Hide file tree" : "Show file tree"}
          >
            {showFileTree ? (
              <PanelLeftClose className="w-4 h-4" />
            ) : (
              <PanelLeftOpen className="w-4 h-4" />
            )}
          </button>

          {/* Search dropdown */}
          <SearchDropdown
            repoId={repoId}
            repoName={repo?.name}
            className="flex-1 max-w-xl"
          />

          {/* Back/Forward navigation */}
          <div className="flex items-center gap-1 border-l border-gray-200 dark:border-gray-700 pl-3">
            <button
              onClick={handleHistoryBack}
              disabled={!navHistory.canGoBack}
              className={`p-1.5 rounded transition-colors ${navHistory.canGoBack
                ? "text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200 hover:bg-gray-100 dark:hover:bg-gray-700"
                : "text-gray-300 dark:text-gray-600 cursor-not-allowed"
                }`}
              title="Go back (⌘[)"
            >
              <ChevronLeft className="w-4 h-4" />
            </button>
            <button
              onClick={handleHistoryForward}
              disabled={!navHistory.canGoForward}
              className={`p-1.5 rounded transition-colors ${navHistory.canGoForward
                ? "text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200 hover:bg-gray-100 dark:hover:bg-gray-700"
                : "text-gray-300 dark:text-gray-600 cursor-not-allowed"
                }`}
              title="Go forward (⌘])"
            >
              <ChevronRight className="w-4 h-4" />
            </button>
          </div>

          {/* Actions */}
          <div className="flex items-center gap-1 border-l border-gray-200 dark:border-gray-700 pl-3">
            <button
              onClick={() => updateUI({ showQuickPicker: true })}
              className="p-1.5 text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200 hover:bg-gray-100 dark:hover:bg-gray-700 rounded transition-colors"
              title="Quick open file (⌘P)"
            >
              <Search className="w-4 h-4" />
            </button>
            <button
              onClick={() => updateUI({ showShortcutsHelp: !showShortcutsHelp })}
              className={`p-1.5 rounded transition-colors ${showShortcutsHelp
                ? "bg-blue-100 dark:bg-blue-900/30 text-blue-600 dark:text-blue-400"
                : "text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200 hover:bg-gray-100 dark:hover:bg-gray-700"
                }`}
              title="Keyboard shortcuts"
            >
              <Keyboard className="w-4 h-4" />
            </button>
            {isFile && (
              <button
                onClick={() => updateUI({ showSymbols: !showSymbols })}
                className={`p-1.5 rounded transition-colors ${showSymbols
                  ? "bg-blue-100 dark:bg-blue-900/30 text-blue-600 dark:text-blue-400"
                  : "text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200 hover:bg-gray-100 dark:hover:bg-gray-700"
                  }`}
                title={showSymbols ? "Hide symbols" : "Show symbols"}
              >
                {showSymbols ? (
                  <PanelRightClose className="w-4 h-4" />
                ) : (
                  <ListTree className="w-4 h-4" />
                )}
              </button>
            )}
          </div>
        </div>
      </div>

      {/* Secondary toolbar with repo info and breadcrumbs */}
      <div className="flex-shrink-0 border-b border-gray-200 dark:border-gray-700 bg-gray-50 dark:bg-gray-800/80 px-3 py-1">
        <div className="flex flex-wrap items-center gap-3">
          {/* Back to repos */}
          <Link
            href="/repos"
            className="flex items-center gap-1 text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200 transition-colors"
          >
            <ArrowLeft className="w-4 h-4" />
            <span className="text-sm hidden sm:inline">Repos</span>
          </Link>
          <span className="text-gray-300 dark:text-gray-600">/</span>

          {/* Repo name - shows active pane's repo in split mode */}
          <span className="text-sm font-medium text-gray-700 dark:text-gray-300 truncate max-w-[150px]">
            {displayRepo?.name || repo?.name || "Loading..."}
          </span>

          {/* Ref selector - shows active pane's refs in split mode */}
          <div className="relative">
            <select
              value={currentDisplayRef}
              onChange={(e) => handleRefChange(e.target.value)}
              className="appearance-none pl-6 pr-4 py-0.5 text-xs border border-gray-200 dark:border-gray-700 rounded bg-white dark:bg-gray-800 focus:outline-none focus:border-blue-500 cursor-pointer"
            >
              {(displayRefs || refs)?.branches?.length ? (
                <optgroup label="Branches">
                  {(displayRefs || refs)?.branches?.map((branch) => (
                    <option key={`branch-${branch}`} value={branch}>
                      {branch}
                    </option>
                  ))}
                </optgroup>
              ) : null}
              {(displayRefs || refs)?.tags?.length ? (
                <optgroup label="Tags">
                  {(displayRefs || refs)?.tags?.map((tag) => (
                    <option key={`tag-${tag}`} value={tag}>
                      {tag}
                    </option>
                  ))}
                </optgroup>
              ) : null}
              {!(displayRefs || refs)?.branches?.length && !(displayRefs || refs)?.tags?.length && (
                <option value={currentDisplayRef}>{currentDisplayRef}</option>
              )}
            </select>
            {(displayRefs || refs)?.tags?.includes(currentDisplayRef) ? (
              <Tag className="absolute left-1.5 top-1/2 -translate-y-1/2 w-3 h-3 text-gray-400 pointer-events-none" />
            ) : (
              <GitBranch className="absolute left-1.5 top-1/2 -translate-y-1/2 w-3 h-3 text-gray-400 pointer-events-none" />
            )}
          </div>

          {/* Breadcrumbs */}
          <nav className="flex items-center gap-1 text-sm overflow-x-auto flex-1">
            <button
              onClick={() => !splitPane.enabled && navigateTo("")}
              className={`flex items-center gap-1 px-1.5 py-0.5 rounded transition-colors ${splitPane.enabled ? "cursor-default" : "hover:bg-gray-200 dark:hover:bg-gray-700"
                }`}
            >
              <Home className="w-3.5 h-3.5 text-gray-400" />
              <span className="text-xs font-medium">{displayRepo?.name?.split("/").pop() || repo?.name?.split("/").pop() || "root"}</span>
            </button>

            {pathParts.map((part, index) => {
              const pathUpTo = pathParts.slice(0, index + 1).join("/");
              const isLast = index === pathParts.length - 1;

              return (
                <span key={pathUpTo} className="flex items-center">
                  <ChevronRight className="w-3.5 h-3.5 text-gray-400" />
                  <button
                    onClick={() => !splitPane.enabled && navigateTo(pathUpTo)}
                    className={`px-1.5 py-0.5 rounded transition-colors text-xs ${isLast ? "font-medium text-blue-600 dark:text-blue-400" : ""} ${splitPane.enabled ? "cursor-default" : "hover:bg-gray-200 dark:hover:bg-gray-700"
                      }`}
                  >
                    {part}
                  </button>
                </span>
              );
            })}
          </nav>

          {/* File actions */}
          {isFile && currentPath && (
            <div className="flex items-center gap-1 ml-auto">
              <button
                onClick={copyPath}
                className="p-1 text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200 transition-colors"
                title="Copy path"
              >
                {copied ? <Check className="w-4 h-4 text-green-500" /> : <Copy className="w-4 h-4" />}
              </button>
              {repo && (
                <a
                  href={(() => {
                    const baseUrl = repo.clone_url.replace(".git", "");
                    // GitHub uses /blob/, GitLab uses /-/blob/, Bitbucket uses /src/
                    if (baseUrl.includes("github.com")) {
                      return `${baseUrl}/blob/${currentRef}/${currentPath}`;
                    } else if (baseUrl.includes("bitbucket")) {
                      return `${baseUrl}/src/${currentRef}/${currentPath}`;
                    } else {
                      // GitLab and others use /-/blob/
                      return `${baseUrl}/-/blob/${currentRef}/${currentPath}`;
                    }
                  })()}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="p-1 text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200 transition-colors"
                  title="View on code host"
                >
                  <ExternalLink className="w-4 h-4" />
                </a>
              )}
            </div>
          )}
        </div>
      </div>

      {/* Main content area with sidebars */}
      <div className="flex-1 flex overflow-hidden">
        {/* File Tree Sidebar (Left) */}
        {showFileTree && (
          <>
            {/* Overlay for mobile */}
            {isMobile && (
              <button
                className="fixed inset-0 bg-black/50 z-40 w-full h-full border-0 cursor-default"
                onClick={() => updateUI({ showFileTree: false })}
                aria-label="Close file tree"
              />
            )}
            <div
              ref={fileTreeRef}
              className={`${isMobile
                ? "fixed left-0 top-0 bottom-0 z-50 w-72"
                : "flex-shrink-0 relative"
                } bg-white dark:bg-gray-800 border-r border-gray-200 dark:border-gray-700 flex flex-col`}
              style={!isMobile ? { width: fileTreeWidth } : undefined}
            >
              <div className="flex items-center justify-between px-3 py-2 border-b border-gray-200 dark:border-gray-700 bg-gray-50 dark:bg-gray-800/80">
                <div className="flex items-center gap-2">
                  <FolderTree className="w-4 h-4 text-gray-400" />
                  <span className="text-sm font-medium text-gray-600 dark:text-gray-300">Files</span>
                </div>
                {isMobile && (
                  <button
                    onClick={() => updateUI({ showFileTree: false })}
                    className="p-1 text-gray-400 hover:text-gray-600 dark:hover:text-gray-300"
                  >
                    <X className="w-4 h-4" />
                  </button>
                )}
              </div>
              <div className="flex-1 overflow-y-auto">
                <LazyFileTree
                  key={displayRepoId}
                  repoId={displayRepoId}
                  currentPath={displayPath}
                  currentRef={ref || undefined}
                  onSelect={handleTreeSelect}
                  splitEnabled={splitPane.enabled}
                />
              </div>
              {/* Resize handle */}
              {!isMobile && (
                <div
                  onMouseDown={startResizing}
                  className="absolute right-0 top-0 bottom-0 w-1 cursor-col-resize hover:bg-blue-500/50 transition-colors group"
                >
                  <div className="absolute right-0 top-1/2 -translate-y-1/2 w-1 h-8 bg-gray-300 dark:bg-gray-600 rounded opacity-0 group-hover:opacity-100 transition-opacity" />
                </div>
              )}
            </div>
          </>
        )}

        {/* Main Content Area */}
        <div className="flex-1 flex flex-col min-w-0 overflow-hidden">
          {/* Error state */}
          {error && (
            <div className="m-4 p-4 bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-lg text-sm text-red-700 dark:text-red-400 flex items-center gap-3">
              <AlertCircle className="w-5 h-5 flex-shrink-0" />
              {error}
            </div>
          )}

          {/* Loading state */}
          {loading && (
            <div className="flex-1 flex items-center justify-center">
              <div className="text-center">
                <Loader2 className="w-8 h-8 animate-spin text-blue-600 mx-auto" />
                <p className="mt-3 text-gray-500 dark:text-gray-400">Loading...</p>
              </div>
            </div>
          )}

          {/* File/Directory content */}
          {!loading && !error && (
            <div className="flex-1 flex overflow-hidden">
              {/* Code viewer or directory listing */}
              <div className="flex-1 flex flex-col overflow-hidden bg-white dark:bg-gray-800">
                {/* Tab bar - only show when viewing files and NOT in split mode (split mode has its own tabs per pane) */}
                {isFile && tabs.length > 0 && !splitPane.enabled && (
                  <BrowseTabBar
                    tabs={tabs}
                    activeTabId={activeTabId}
                    onTabSelect={(tab) => {
                      // Navigate to the tab's file
                      router.push(buildBrowseUrl(tab.repoId, tab.filePath, { ref: tab.ref }));
                    }}
                    onTabClose={(tabId) => {
                      closeTab(tabId);
                      // If closing the active tab, navigate to the new active tab or directory
                      const remainingTabs = tabs.filter(t => t.id !== tabId);
                      if (tabId === activeTabId && remainingTabs.length > 0) {
                        const closedTabIndex = tabs.findIndex(t => t.id === tabId);
                        const newActiveTab = remainingTabs[Math.min(closedTabIndex, remainingTabs.length - 1)];
                        router.push(buildBrowseUrl(newActiveTab.repoId, newActiveTab.filePath, { ref: newActiveTab.ref }));
                      } else if (remainingTabs.length === 0) {
                        // No more tabs, go to repo root
                        router.push(buildBrowseUrl(repoId, ""));
                      }
                    }}
                    onSplitVertical={handleSplitVertical}
                    onSplitHorizontal={handleSplitHorizontal}
                    showSplitButtons={true}
                  />
                )}

                {(isFile && blob) || splitPane.enabled ? (
                  // File view - with optional split
                  <div
                    ref={splitContainerRef}
                    className={`flex-1 flex ${splitPane.enabled ? (splitPane.direction === "vertical" ? "flex-row" : "flex-col") : "flex-col"} overflow-hidden`}
                  >
                    {/* Primary file pane */}
                    {(() => {
                      // Determine which blob/file to show in primary pane
                      const pFile = splitPane.enabled && primaryActiveFile ? primaryActiveFile : { repoId, path: currentPath, ref: ref || undefined };
                      const pBlob = splitPane.enabled && primaryActiveFile ? primaryBlob : blob;
                      const pLoading = splitPane.enabled && primaryActiveFile ? primaryBlobLoading : false;

                      return (
                        <div
                          role={splitPane.enabled ? "button" : undefined}
                          tabIndex={splitPane.enabled ? 0 : undefined}
                          onClick={() => splitPane.enabled && handleSetActivePane("primary")}
                          onKeyDown={(e) => { if (splitPane.enabled && (e.key === "Enter" || e.key === " ")) { e.preventDefault(); handleSetActivePane("primary"); } }}
                          className={`flex flex-col min-w-0 min-h-0 overflow-hidden ${splitPane.enabled && splitPane.activePane === "primary"
                            ? "border-2 border-blue-400/40"
                            : splitPane.enabled ? "border-2 border-transparent" : ""
                            }`}
                          style={splitPane.enabled ? {
                            [splitPane.direction === "vertical" ? "width" : "height"]: `${splitPane.ratio * 100}%`
                          } : { flex: 1 }}
                        >
                          {/* Pane tabs - VS Code style */}
                          {splitPane.enabled && splitPane.primaryTabs.length > 0 && (
                            <div className={`flex items-center border-b border-gray-200 dark:border-gray-700 ${splitPane.activePane === "primary"
                              ? "bg-blue-50/30 dark:bg-blue-900/5"
                              : "bg-gray-50 dark:bg-gray-800/80"
                              }`}>
                              <div className="flex-1 flex items-center overflow-x-auto scrollbar-thin">
                                {splitPane.primaryTabs.map((tab) => (
                                  <div
                                    key={tab.id}
                                    role="tab"
                                    tabIndex={0}
                                    aria-selected={tab.id === splitPane.primaryActiveTabId}
                                    onClick={(e) => { e.stopPropagation(); selectTabInPane("primary", tab.id); }}
                                    onKeyDown={(e) => { if (e.key === "Enter" || e.key === " ") { e.preventDefault(); e.stopPropagation(); selectTabInPane("primary", tab.id); } }}
                                    onAuxClick={(e) => { if (e.button === 1) { e.preventDefault(); closeTabInPane("primary", tab.id); } }}
                                    className={`group flex items-center gap-1.5 px-3 py-1.5 text-xs border-r border-gray-200 dark:border-gray-700 cursor-pointer ${tab.id === splitPane.primaryActiveTabId
                                      ? "bg-white dark:bg-gray-800 text-gray-800 dark:text-gray-200"
                                      : "text-gray-500 dark:text-gray-400 hover:bg-gray-100 dark:hover:bg-gray-700/50"
                                      }`}
                                  >
                                    {getFileIcon(undefined, tab.file.path.split("/").pop())}
                                    <span className="truncate max-w-[120px]">{tab.file.path.split("/").pop()}</span>
                                    <button
                                      onClick={(e) => { e.stopPropagation(); closeTabInPane("primary", tab.id); }}
                                      className="p-0.5 rounded opacity-0 group-hover:opacity-100 hover:bg-gray-200 dark:hover:bg-gray-600 transition-opacity"
                                    >
                                      <X className="w-3 h-3" />
                                    </button>
                                  </div>
                                ))}
                              </div>
                              <button
                                onClick={(e) => { e.stopPropagation(); handleCloseSplit("primary"); }}
                                className="p-1.5 text-gray-400 hover:text-gray-600 dark:hover:text-gray-300 transition-colors flex-shrink-0"
                                title="Close this pane"
                              >
                                <X className="w-4 h-4" />
                              </button>
                            </div>
                          )}

                          {/* Code content */}
                          <div className="flex-1 overflow-auto min-h-0">
                            {pLoading ? (
                              <div className="flex-1 flex items-center justify-center h-full">
                                <Loader2 className="w-6 h-6 animate-spin text-gray-400" />
                              </div>
                            ) : pBlob?.binary ? (
                              <div className="flex-1 flex flex-col items-center p-8 overflow-auto">
                                                                  {isImageFile(pFile.path) ? (
                                                                    <div className="flex flex-col items-center gap-4 w-full h-full min-h-[400px]">
                                                                      <FileImage className="w-16 h-16 text-gray-400 flex-shrink-0" />
                                                                      <div className="relative w-full flex-1">
                                                                        <Image
                                                                          src={getRawFileUrl(pFile.repoId, pFile.path, pFile.ref)}
                                                                          alt={pFile.path.split("/").pop() || "Image"}
                                                                          fill
                                                                          sizes="100vw"
                                                                          className="object-contain"
                                                                          unoptimized={true}
                                                                        />
                                                                      </div>
                                                                    </div>
                                                                  ) : isPdfFile(pFile.path) ? (
                                
                                  <div className="flex flex-col items-center gap-4">
                                    <FileText className="w-16 h-16 text-gray-400" />
                                    <iframe
                                      src={getRawFileUrl(pFile.repoId, pFile.path, pFile.ref)}
                                      className="w-full h-full border-0"
                                      title={pFile.path.split("/").pop() || "PDF"}
                                    />
                                  </div>
                                ) : isVideoFile(pFile.path) ? (
                                  <div className="flex flex-col items-center gap-4">
                                    <FileVideo className="w-16 h-16 text-gray-400" />
                                    <video
                                      src={getRawFileUrl(pFile.repoId, pFile.path, pFile.ref)}
                                      controls
                                      className="max-w-full max-h-full"
                                    >
                                      Your browser does not support the video tag.
                                    </video>
                                  </div>
                                ) : isAudioFile(pFile.path) ? (
                                  <div className="flex flex-col items-center gap-4">
                                    <FileAudio className="w-16 h-16 text-gray-400" />
                                    <audio
                                      src={getRawFileUrl(pFile.repoId, pFile.path, pFile.ref)}
                                      controls
                                      className="w-full max-w-md"
                                    >
                                      Your browser does not support the audio tag.
                                    </audio>
                                  </div>
                                ) : (
                                  <div className="text-center text-gray-500 dark:text-gray-400">
                                    <FileText className="w-16 h-16 mx-auto mb-4 text-gray-300 dark:text-gray-600" />
                                    <p className="mb-4">Binary file cannot be displayed</p>
                                    <a
                                      href={getRawFileUrl(pFile.repoId, pFile.path, pFile.ref)}
                                      download
                                      className="inline-flex items-center gap-2 px-4 py-2 bg-blue-500 text-white rounded-md hover:bg-blue-600 transition-colors"
                                    >
                                      Download file
                                    </a>
                                  </div>
                                )}
                              </div>
                            ) : pBlob ? (
                              <CodeViewer
                                key={splitPane.enabled ? `primary-${primaryScrollToLine?.key}` : undefined}
                                content={pBlob.content}
                                languageMode={pBlob.language_mode}
                                language={pBlob.language}
                                repoId={pFile.repoId}
                                filePath={pFile.path}
                                highlightLines={
                                  splitPane.enabled
                                    ? (primaryScrollToLine ? [primaryScrollToLine.line] : [])
                                    : (scrollToLine ? [scrollToLine] : (highlightLine ? [highlightLine] : []))
                                }
                                scrollToLine={splitPane.enabled ? primaryScrollToLine?.line : scrollToLine}
                                onWordClick={(word, line, col) => handleFindReferences(word, line, col, pFile.repoId, pFile.path, pBlob.language)}
                                onGoToDefinition={(word, line, col) => handleGoToDefinition(word, line, col, pFile.repoId, pFile.path, pBlob.language)}
                                onLineClick={(line) => handleLineClick(line, pFile.repoId, pFile.path)}
                              />
                            ) : (
                              <div className="p-8 text-center text-gray-500 dark:text-gray-400">
                                {splitPane.enabled ? "Select a file to view" : "Failed to load file"}
                              </div>
                            )}
                          </div>
                        </div>
                      );
                    })()}

                    {/* Split resize handle */}
                    {splitPane.enabled && (
                      <div
                        role="separator"
                        aria-orientation={splitPane.direction === "vertical" ? "vertical" : "horizontal"}
                        tabIndex={0}
                        onMouseDown={startSplitResizing}
                        className={`
                          ${splitPane.direction === "vertical" ? "w-1 cursor-col-resize" : "h-1 cursor-row-resize"}
                          bg-gray-200 dark:bg-gray-700 hover:bg-blue-400 dark:hover:bg-blue-500
                          transition-colors flex-shrink-0
                        `}
                      />
                    )}

                    {/* Secondary file pane */}
                    {splitPane.enabled && (
                      <div
                        role="button"
                        tabIndex={0}
                        onClick={() => handleSetActivePane("secondary")}
                        onKeyDown={(e) => { if (e.key === "Enter" || e.key === " ") { e.preventDefault(); handleSetActivePane("secondary"); } }}
                        className={`flex flex-col min-w-0 min-h-0 overflow-hidden ${splitPane.activePane === "secondary"
                          ? "border-2 border-blue-400/40"
                          : "border-2 border-transparent"
                          }`}
                        style={{
                          [splitPane.direction === "vertical" ? "width" : "height"]: `${(1 - splitPane.ratio) * 100}%`
                        }}
                      >
                        {/* Secondary pane tabs - VS Code style */}
                        <div className={`flex items-center border-b border-gray-200 dark:border-gray-700 ${splitPane.activePane === "secondary"
                          ? "bg-blue-50/30 dark:bg-blue-900/5"
                          : "bg-gray-50 dark:bg-gray-800/80"
                          }`}>
                          <div className="flex-1 flex items-center overflow-x-auto scrollbar-thin">
                            {splitPane.secondaryTabs.map((tab) => (
                              <div
                                key={tab.id}
                                role="tab"
                                tabIndex={0}
                                aria-selected={tab.id === splitPane.secondaryActiveTabId}
                                onClick={(e) => { e.stopPropagation(); selectTabInPane("secondary", tab.id); }}
                                onKeyDown={(e) => { if (e.key === "Enter" || e.key === " ") { e.preventDefault(); e.stopPropagation(); selectTabInPane("secondary", tab.id); } }}
                                onAuxClick={(e) => { if (e.button === 1) { e.preventDefault(); closeTabInPane("secondary", tab.id); } }}
                                className={`group flex items-center gap-1.5 px-3 py-1.5 text-xs border-r border-gray-200 dark:border-gray-700 cursor-pointer ${tab.id === splitPane.secondaryActiveTabId
                                  ? "bg-white dark:bg-gray-800 text-gray-800 dark:text-gray-200"
                                  : "text-gray-500 dark:text-gray-400 hover:bg-gray-100 dark:hover:bg-gray-700/50"
                                  }`}
                              >
                                {getFileIcon(undefined, tab.file.path.split("/").pop())}
                                <span className="truncate max-w-[120px]">{tab.file.path.split("/").pop()}</span>
                                <button
                                  onClick={(e) => { e.stopPropagation(); closeTabInPane("secondary", tab.id); }}
                                  className="p-0.5 rounded opacity-0 group-hover:opacity-100 hover:bg-gray-200 dark:hover:bg-gray-600 transition-opacity"
                                >
                                  <X className="w-3 h-3" />
                                </button>
                              </div>
                            ))}
                          </div>
                          <button
                            onClick={(e) => { e.stopPropagation(); handleCloseSplit("secondary"); }}
                            className="p-1.5 text-gray-400 hover:text-gray-600 dark:hover:text-gray-300 transition-colors flex-shrink-0"
                            title="Close this pane"
                          >
                            <X className="w-4 h-4" />
                          </button>
                        </div>

                        {/* Secondary code content */}
                        <div className="flex-1 overflow-auto min-h-0">
                          {secondaryBlobLoading ? (
                            <div className="flex-1 flex items-center justify-center h-full">
                              <Loader2 className="w-6 h-6 animate-spin text-gray-400" />
                            </div>
                          ) : secondaryBlob?.binary ? (
                            <div className="flex-1 flex flex-col items-center p-8 overflow-auto">
                                                              {secondaryActiveFile && isImageFile(secondaryActiveFile.path) ? (
                                                                <div className="flex flex-col items-center gap-4 w-full h-full min-h-[400px]">
                                                                  <FileImage className="w-16 h-16 text-gray-400 flex-shrink-0" />
                                                                  <div className="relative w-full flex-1">
                                                                    <Image
                                                                      src={getRawFileUrl(secondaryActiveFile.repoId, secondaryActiveFile.path, secondaryActiveFile.ref)}
                                                                      alt={secondaryActiveFile.path.split("/").pop() || "Image"}
                                                                      fill
                                                                      sizes="100vw"
                                                                      className="object-contain"
                                                                      unoptimized={true}
                                                                    />
                                                                  </div>
                                                                </div>
                                                              ) : secondaryActiveFile && isPdfFile(secondaryActiveFile.path) ? (
                              
                                <div className="flex flex-col items-center gap-4">
                                  <FileText className="w-16 h-16 text-gray-400" />
                                  <iframe
                                    src={getRawFileUrl(secondaryActiveFile.repoId, secondaryActiveFile.path, secondaryActiveFile.ref)}
                                    className="w-full h-full border-0"
                                    title={secondaryActiveFile.path.split("/").pop() || "PDF"}
                                  />
                                </div>
                              ) : secondaryActiveFile && isVideoFile(secondaryActiveFile.path) ? (
                                <div className="flex flex-col items-center gap-4">
                                  <FileVideo className="w-16 h-16 text-gray-400" />
                                  <video
                                    src={getRawFileUrl(secondaryActiveFile.repoId, secondaryActiveFile.path, secondaryActiveFile.ref)}
                                    controls
                                    className="max-w-full max-h-full"
                                  >
                                    Your browser does not support the video tag.
                                  </video>
                                </div>
                              ) : secondaryActiveFile && isAudioFile(secondaryActiveFile.path) ? (
                                <div className="flex flex-col items-center gap-4">
                                  <FileAudio className="w-16 h-16 text-gray-400" />
                                  <audio
                                    src={getRawFileUrl(secondaryActiveFile.repoId, secondaryActiveFile.path, secondaryActiveFile.ref)}
                                    controls
                                    className="w-full max-w-md"
                                  >
                                    Your browser does not support the audio tag.
                                  </audio>
                                </div>
                              ) : (
                                <div className="text-center text-gray-500 dark:text-gray-400">
                                  <FileText className="w-16 h-16 mx-auto mb-4 text-gray-300 dark:text-gray-600" />
                                  <p className="mb-4">Binary file cannot be displayed</p>
                                  {secondaryActiveFile && (
                                    <a
                                      href={getRawFileUrl(secondaryActiveFile.repoId, secondaryActiveFile.path, secondaryActiveFile.ref)}
                                      download
                                      className="inline-flex items-center gap-2 px-4 py-2 bg-blue-500 text-white rounded-md hover:bg-blue-600 transition-colors"
                                    >
                                      Download file
                                    </a>
                                  )}
                                </div>
                              )}
                            </div>
                          ) : secondaryBlob && secondaryActiveFile ? (
                            <CodeViewer
                              key={`secondary-${secondaryScrollToLine?.key}`}
                              content={secondaryBlob.content}
                              languageMode={secondaryBlob.language_mode}
                              language={secondaryBlob.language}
                              repoId={secondaryActiveFile.repoId}
                              filePath={secondaryActiveFile.path}
                              highlightLines={secondaryScrollToLine ? [secondaryScrollToLine.line] : []}
                              scrollToLine={secondaryScrollToLine?.line}
                              onWordClick={(word, line, col) => handleFindReferences(word, line, col, secondaryActiveFile.repoId, secondaryActiveFile.path, secondaryBlob.language)}
                              onGoToDefinition={(word, line, col) => handleGoToDefinition(word, line, col, secondaryActiveFile.repoId, secondaryActiveFile.path, secondaryBlob.language)}
                              onLineClick={(line) => handleLineClick(line, secondaryActiveFile.repoId, secondaryActiveFile.path)}
                            />
                          ) : (
                            <div className="p-8 text-center text-gray-500 dark:text-gray-400">
                              Select a file to view
                            </div>
                          )}
                        </div>
                      </div>
                    )}
                  </div>
                ) : (
                  // Directory view
                  <div className="divide-y divide-gray-100 dark:divide-gray-700/50">
                    {entries.map((entry) => (
                      <button
                        key={entry.path}
                        onClick={() => {
                          // In split mode, ignore folder clicks (only files open in panes)
                          if (splitPane.enabled && entry.type === "dir") return;
                          handleTreeSelect(entry.path, entry.type === "dir");
                        }}
                        className={`w-full flex items-center gap-3 px-4 py-2.5 text-left transition-colors ${splitPane.enabled && entry.type === "dir"
                          ? "cursor-default"
                          : "hover:bg-gray-50 dark:hover:bg-gray-700/30"
                          }`}
                      >
                        {entry.type === "dir" ? (
                          <Folder className="w-5 h-5 text-blue-500 flex-shrink-0" />
                        ) : (
                          <FileCode className="w-5 h-5 text-gray-400 flex-shrink-0" />
                        )}
                        <span className="text-sm truncate">{entry.name}</span>
                        {entry.language && (
                          <span className="ml-auto text-xs text-gray-400 dark:text-gray-500">
                            {entry.language}
                          </span>
                        )}
                      </button>
                    ))}

                    {entries.length === 0 && (
                      <div className="p-8 text-center text-gray-500 dark:text-gray-400">
                        Empty directory
                      </div>
                    )}
                  </div>
                )}
              </div>

              {/* Symbol Sidebar (Right) - only for files */}
              {isFile && symbolsBlob && !symbolsBlob.binary && showSymbols && (
                <>
                  {/* Overlay for mobile */}
                  {isMobile && (
                    <button
                      className="fixed inset-0 bg-black/50 z-40 w-full h-full border-0 cursor-default"
                      onClick={() => updateUI({ showSymbols: false })}
                      aria-label="Close symbols"
                    />
                  )}
                  <div
                    ref={symbolsRef}
                    className={`${isMobile
                      ? "fixed right-0 top-0 bottom-0 z-50 w-64"
                      : "flex-shrink-0 relative"
                      } bg-white dark:bg-gray-800 border-l border-gray-200 dark:border-gray-700 flex flex-col`}
                    style={!isMobile ? { width: symbolsWidth } : undefined}
                  >
                    {/* Resize handle */}
                    {!isMobile && (
                      <div
                        onMouseDown={startResizingSymbols}
                        className="absolute left-0 top-0 bottom-0 w-1 cursor-col-resize hover:bg-blue-500/50 transition-colors group"
                      >
                        <div className="absolute left-0 top-1/2 -translate-y-1/2 w-1 h-8 bg-gray-300 dark:bg-gray-600 rounded opacity-0 group-hover:opacity-100 transition-opacity" />
                      </div>
                    )}
                    <div className="flex items-center justify-between px-3 py-2 border-b border-gray-200 dark:border-gray-700 bg-gray-50 dark:bg-gray-800/80">
                      <div className="flex items-center gap-2">
                        <ListTree className="w-4 h-4 text-gray-400" />
                        <span className="text-sm font-medium text-gray-600 dark:text-gray-300">Symbols</span>
                        {splitPane.enabled && symbolsFile.path && (
                          <span className="text-xs text-gray-400 truncate max-w-[100px]">
                            ({symbolsFile.path.split("/").pop()})
                          </span>
                        )}
                      </div>
                      {isMobile && (
                        <button
                          onClick={() => updateUI({ showSymbols: false })}
                          className="p-1 text-gray-400 hover:text-gray-600 dark:hover:text-gray-300"
                        >
                          <X className="w-4 h-4" />
                        </button>
                      )}
                    </div>
                    <div className="flex-1 overflow-y-auto">
                      <SymbolSidebar
                        repoId={symbolsFile.repoId}
                        path={symbolsFile.path}
                        language={symbolsBlob?.language}
                        onSymbolClick={(line) => {
                          scrollKeyRef.current += 1;
                          const scrollData = { line, key: scrollKeyRef.current };
                          if (!splitPane.enabled) {
                            setScrollToLine(line);
                          } else if (splitPane.activePane === "primary") {
                            setPrimaryScrollToLine(scrollData);
                          } else {
                            setSecondaryScrollToLine(scrollData);
                          }
                        }}
                        onFindReferences={handleFindReferences}
                      />
                    </div>
                  </div>
                </>
              )}
            </div>
          )}
        </div>
      </div>

      {/* Reference Panel */}
      {referenceSearch && (
        <div
          className="fixed bottom-0 left-0 right-0 z-50 shadow-lg flex flex-col"
          style={{ height: referencePanelHeight }}
        >
          {/* Resize handle */}
          <div
            onMouseDown={startResizingReferences}
            className="h-1 cursor-row-resize hover:bg-blue-500/50 bg-gray-200 dark:bg-gray-700 transition-colors flex-shrink-0"
          />
          <div className="flex-1 overflow-hidden">
            <ReferencePanel
              symbolName={referenceSearch.symbolName}
              language={referenceSearch.language}
              repos={contextRepoNames}
              repoId={referenceSearch.repoId}
              filePath={referenceSearch.filePath}
              line={referenceSearch.line}
              col={referenceSearch.col}
              onClose={() => setReferenceSearch(null)}
              onNavigate={handleReferenceNavigate}
            />
          </div>
        </div>
      )}

      {/* Quick File Picker (Cmd+P) */}
      <QuickFilePicker
        repoId={repoId}
        currentRef={ref || undefined}
        isOpen={showQuickPicker}
        onClose={() => updateUI({ showQuickPicker: false })}
        onSelect={handleQuickPickerSelect}
      />

      {/* Keyboard Shortcuts Help */}
      {showShortcutsHelp && (
        <div
          role="dialog"
          aria-modal="true"
          className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 backdrop-blur-sm"
          onClick={() => updateUI({ showShortcutsHelp: false })}
          onKeyDown={(e) => { if (e.key === "Escape") updateUI({ showShortcutsHelp: false }); }}
        >
          <div
            className="bg-white dark:bg-gray-800 rounded-xl shadow-2xl border border-gray-200 dark:border-gray-700 w-full max-w-md overflow-hidden"
            onClick={(e) => e.stopPropagation()}
          >
            <div className="flex items-center justify-between px-4 py-3 border-b border-gray-200 dark:border-gray-700">
              <h2 className="font-semibold">Keyboard Shortcuts</h2>
              <button
                onClick={() => updateUI({ showShortcutsHelp: false })}
                className="p-1 text-gray-400 hover:text-gray-600 dark:hover:text-gray-300 transition-colors"
              >
                <X className="w-4 h-4" />
              </button>
            </div>
            <div className="p-4 space-y-3">
              <div className="flex items-center justify-between py-2">
                <span className="text-sm text-gray-600 dark:text-gray-300">Quick open file</span>
                <kbd className="px-2 py-1 text-xs font-mono bg-gray-100 dark:bg-gray-700 rounded border border-gray-200 dark:border-gray-600">
                  ⌘P
                </kbd>
              </div>
              <div className="flex items-center justify-between py-2 border-t border-gray-100 dark:border-gray-700">
                <span className="text-sm text-gray-600 dark:text-gray-300">Go to definition</span>
                <div className="flex items-center gap-2">
                  <kbd className="px-2 py-1 text-xs font-mono bg-gray-100 dark:bg-gray-700 rounded border border-gray-200 dark:border-gray-600">
                    F12
                  </kbd>
                  <span className="text-xs text-gray-400">or</span>
                  <kbd className="px-2 py-1 text-xs font-mono bg-gray-100 dark:bg-gray-700 rounded border border-gray-200 dark:border-gray-600">
                    ⌘+click
                  </kbd>
                </div>
              </div>
              <div className="flex items-center justify-between py-2 border-t border-gray-100 dark:border-gray-700">
                <span className="text-sm text-gray-600 dark:text-gray-300">Find references</span>
                <div className="flex items-center gap-2">
                  <kbd className="px-2 py-1 text-xs font-mono bg-gray-100 dark:bg-gray-700 rounded border border-gray-200 dark:border-gray-600">
                    ⇧F12
                  </kbd>
                  <span className="text-xs text-gray-400">or</span>
                  <kbd className="px-2 py-1 text-xs font-mono bg-gray-100 dark:bg-gray-700 rounded border border-gray-200 dark:border-gray-600">
                    ⌘+hover
                  </kbd>
                </div>
              </div>
              <div className="flex items-center justify-between py-2 border-t border-gray-100 dark:border-gray-700">
                <span className="text-sm text-gray-600 dark:text-gray-300">Toggle file tree</span>
                <kbd className="px-2 py-1 text-xs font-mono bg-gray-100 dark:bg-gray-700 rounded border border-gray-200 dark:border-gray-600">
                  ⌘B
                </kbd>
              </div>
              <div className="flex items-center justify-between py-2 border-t border-gray-100 dark:border-gray-700">
                <span className="text-sm text-gray-600 dark:text-gray-300">Go back</span>
                <kbd className="px-2 py-1 text-xs font-mono bg-gray-100 dark:bg-gray-700 rounded border border-gray-200 dark:border-gray-600">
                  ⌘[
                </kbd>
              </div>
              <div className="flex items-center justify-between py-2 border-t border-gray-100 dark:border-gray-700">
                <span className="text-sm text-gray-600 dark:text-gray-300">Go forward</span>
                <kbd className="px-2 py-1 text-xs font-mono bg-gray-100 dark:bg-gray-700 rounded border border-gray-200 dark:border-gray-600">
                  ⌘]
                </kbd>
              </div>
              <div className="flex items-center justify-between py-2 border-t border-gray-100 dark:border-gray-700">
                <span className="text-sm text-gray-600 dark:text-gray-300">Close panels</span>
                <kbd className="px-2 py-1 text-xs font-mono bg-gray-100 dark:bg-gray-700 rounded border border-gray-200 dark:border-gray-600">
                  Esc
                </kbd>
              </div>
            </div>
            <div className="px-4 py-3 border-t border-gray-200 dark:border-gray-700 bg-gray-50 dark:bg-gray-800/80">
              <p className="text-xs text-gray-500 dark:text-gray-400">
                Tip: Position cursor on a symbol before using F12 or Shift+F12
              </p>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

// Helper to format bytes
function _formatBytes(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}
