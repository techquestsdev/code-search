"use client";

import { useMemo, useEffect, useRef, useCallback, useState } from "react";
import dynamic from "next/dynamic";
import { ReactCodeMirrorRef } from "@uiw/react-codemirror";
import { githubLight, githubDark } from "@uiw/codemirror-theme-github";
import { HoverPopup } from "./HoverPopup";
import { useTheme } from "./ThemeProvider";
import {
  EditorView,
  EditorState,
  Extension,
  languageExtensions,
  createLineNumberClickExtension,
  createHighlightExtension,
  createHoverExtension,
  createClickExtension,
  createKeyboardShortcutsExtension,
  highlightTheme,
  darkModeOverrides,
  lightModeOverrides,
} from "@/lib/code-viewer-utils";
import type { HoverState } from "@/lib/code-viewer-utils";

// Heavy imports for dynamic loading
const CodeMirror = dynamic(() => import("@uiw/react-codemirror"), {
  ssr: false,
  loading: () => <div className="h-full w-full bg-gray-50 dark:bg-gray-900 animate-pulse" />
});

// API URL for SCIP requests (same as lib/api.ts)
const API_URL = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";

// Get CodeMirror language extension for a given language mode
function getLanguageExtension(languageMode: string): Extension[] {
  const mode = languageMode?.toLowerCase() || "text";
  const factory = languageExtensions[mode];
  if (factory) {
    return [factory()];
  }
  return [];
}

interface CodeViewerProps {
  content: string;
  language?: string;
  languageMode?: string;
  highlightLines?: number[];
  scrollToLine?: number;
  className?: string;
  readonly?: boolean;
  repoId?: number; // Required for SCIP lookup on hover
  filePath?: string; // Required for SCIP lookup on hover
  onWordClick?: (word: string, line: number, col: number) => void;
  onGoToDefinition?: (word: string, line: number, col: number) => void;
  onHover?: (word: string, line: number, x: number, y: number) => void;
  onLineClick?: (line: number) => void; // Called when clicking a line number in the gutter
  showHoverPopup?: boolean;
  enableKeyboardShortcuts?: boolean;
}

// ---------- useSCIPHover hook ----------

interface UseSCIPHoverOptions {
  repoId?: number;
  filePath?: string;
  language?: string;
  showHoverPopup?: boolean;
  onWordClick?: (word: string, line: number, col: number) => void;
  onGoToDefinition?: (word: string, line: number, col: number) => void;
  onHover?: (word: string, line: number, x: number, y: number) => void;
}

function useSCIPHover({
  repoId,
  filePath,
  language,
  showHoverPopup = true,
  onWordClick,
  onGoToDefinition,
  onHover,
}: UseSCIPHoverOptions) {
  const [hoverState, setHoverState] = useState<HoverState | null>(null);

  // Fetch SCIP definition info for hover
  const fetchSCIPInfo = useCallback(async (word: string, line: number, col: number): Promise<string | undefined> => {
    if (!repoId || !filePath) {
      return undefined;
    }

    try {
      const response = await fetch(`${API_URL}/api/v1/scip/repos/${repoId}/definition`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          filePath,
          line, // API accepts 1-indexed lines (same as CodeMirror)
          column: col,
        }),
      });

      if (!response.ok) {
        return undefined;
      }

      const data = await response.json();

      if (data.found) {
        // Return raw documentation - HoverPopup will parse it
        if (data.info?.documentation) {
          return data.info.documentation;
        }
        // Fall back to the definition's source line (context)
        if (data.definition?.context) {
          return data.definition.context.trim();
        }
      }

      // SCIP didn't find anything - fall back to Zoekt symbol search
      return await fetchSymbolInfo(word, language);
    } catch (e) {
      console.error("Failed to fetch SCIP info:", e);
      // Also try symbol search on error
      return await fetchSymbolInfo(word, language);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [repoId, filePath, language]);

  // Fallback: fetch symbol info from Zoekt when SCIP doesn't have data
  const fetchSymbolInfo = useCallback(async (word: string, lang?: string): Promise<string | undefined> => {
    try {
      const response = await fetch(`${API_URL}/api/v1/symbols/find`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          name: word,
          language: lang,
          limit: 1,
        }),
      });

      if (!response.ok) {
        return undefined;
      }

      const symbols = await response.json();
      if (symbols && symbols.length > 0) {
        const sym = symbols[0];
        // Return the symbol's context/signature if available
        if (sym.context) {
          return sym.context.trim();
        }
        // Or construct a basic signature from available info
        if (sym.kind && sym.name) {
          return `${sym.kind} ${sym.name}`;
        }
      }
      return undefined;
    } catch (e) {
      console.error("Failed to fetch symbol info:", e);
      return undefined;
    }
  }, []);

  // Handle hover popup close
  const handleHoverClose = useCallback(() => {
    setHoverState(null);
  }, []);

  // Handle find references from popup
  const handleFindReferences = useCallback(() => {
    if (hoverState && onWordClick) {
      onWordClick(hoverState.word, hoverState.line, hoverState.col);
    }
  }, [hoverState, onWordClick]);

  // Handle go to definition from popup
  const handleGoToDefinition = useCallback(() => {
    if (hoverState && onGoToDefinition) {
      onGoToDefinition(hoverState.word, hoverState.line, hoverState.col);
    }
  }, [hoverState, onGoToDefinition]);

  // Create hover extension
  const hoverExtension = useMemo(() => {
    return createHoverExtension(
      !!showHoverPopup,
      language,
      fetchSCIPInfo,
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      (state: any) => {
        if (typeof state === 'function') {
          setHoverState(prev => state(prev));
        } else {
          setHoverState(state);
        }
      },
      (word, line, x, y) => {
        if (onHover) {
          onHover(word, line, x, y);
        }
      }
    );
  }, [showHoverPopup, language, fetchSCIPInfo, onHover]);

  return { hoverState, hoverExtension, handleHoverClose, handleFindReferences, handleGoToDefinition };
}

// ---------- CodeViewer component ----------

const EMPTY_HIGHLIGHT_LINES: number[] = [];

export function CodeViewer({
  content,
  language,
  languageMode,
  highlightLines = EMPTY_HIGHLIGHT_LINES,
  scrollToLine,
  className = "",
  readonly = true,
  repoId,
  filePath,
  onWordClick,
  onGoToDefinition,
  onHover,
  onLineClick,
  showHoverPopup = true,
  enableKeyboardShortcuts = true,
}: CodeViewerProps) {
  const editorRef = useRef<ReactCodeMirrorRef>(null);

  // Get current theme
  const { resolvedTheme } = useTheme();
  const editorTheme = resolvedTheme === "dark" ? githubDark : githubLight;

  // Use languageMode if provided, otherwise use language
  const effectiveMode = languageMode || language?.toLowerCase() || "text";

  // SCIP/symbol hover logic
  const { hoverState, hoverExtension, handleHoverClose, handleFindReferences, handleGoToDefinition } = useSCIPHover({
    repoId,
    filePath,
    language,
    showHoverPopup,
    onWordClick,
    onGoToDefinition,
    onHover,
  });

  // Keyboard shortcuts as a CodeMirror keymap extension
  const keyboardShortcutsExtension = useMemo(() => {
    if (!enableKeyboardShortcuts) return [];
    return createKeyboardShortcutsExtension({
      language,
      onGoToDefinition,
      onFindReferences: onWordClick,
    });
  }, [enableKeyboardShortcuts, language, onGoToDefinition, onWordClick]);

  // Build extensions array
  const extensions = useMemo(() => {
    const exts: Extension[] = [
      EditorView.lineWrapping,
      EditorView.editable.of(!readonly),
      EditorState.readOnly.of(readonly),
      highlightTheme,
      resolvedTheme === "dark" ? darkModeOverrides : lightModeOverrides,
      ...getLanguageExtension(effectiveMode),
      createClickExtension(onWordClick, language),
      createLineNumberClickExtension(onLineClick),
      hoverExtension,
      keyboardShortcutsExtension,
    ];

    if (highlightLines.length > 0) {
      exts.push(createHighlightExtension(highlightLines));
    }

    return exts;
  }, [effectiveMode, highlightLines, readonly, onWordClick, onLineClick, language, hoverExtension, keyboardShortcutsExtension, resolvedTheme]);

  // Scroll to line on mount or when scrollToLine changes
  useEffect(() => {
    if (!scrollToLine) return;

    const attemptScroll = (attempts: number = 0) => {
      if (!editorRef.current?.view) {
        if (attempts < 10) {
          setTimeout(() => attemptScroll(attempts + 1), 50);
        }
        return;
      }

      const view = editorRef.current.view;
      const doc = view.state.doc;

      if (scrollToLine >= 1 && scrollToLine <= doc.lines) {
        const line = doc.line(scrollToLine);

        view.dispatch({
          effects: EditorView.scrollIntoView(line.from, {
            y: "center",
          }),
        });
      }
    };

    requestAnimationFrame(() => attemptScroll());
  }, [scrollToLine, content]);

  return (
    <div
      role="application"
      className={`code-viewer relative h-full ${className}`}
      onKeyDown={(e) => {
        if ((e.metaKey || e.ctrlKey) && e.key === 'a') {
          e.stopPropagation();
        }
      }}
    >
      <CodeMirror
        ref={editorRef}
        value={content}
        extensions={extensions}
        editable={!readonly}
        basicSetup={{
          lineNumbers: true,
          highlightActiveLineGutter: true,
          highlightSpecialChars: true,
          history: false,
          foldGutter: true,
          drawSelection: true,
          dropCursor: false,
          allowMultipleSelections: false,
          indentOnInput: false,
          syntaxHighlighting: true,
          bracketMatching: true,
          closeBrackets: false,
          autocompletion: false,
          rectangularSelection: false,
          crosshairCursor: false,
          highlightActiveLine: true,
          highlightSelectionMatches: true,
          closeBracketsKeymap: false,
          defaultKeymap: true,
          searchKeymap: true,
          historyKeymap: false,
          foldKeymap: true,
          completionKeymap: false,
          lintKeymap: false,
        }}
        theme={editorTheme}
        className="text-sm h-full [&_.cm-editor]:h-full [&_.cm-scroller]:!overflow-auto"
      />

      {showHoverPopup && hoverState && (
        <HoverPopup
          word={hoverState.word}
          language={language}
          signature={hoverState.signature}
          x={hoverState.x}
          y={hoverState.y}
          isLoadingDefinition={hoverState.isLoading}
          onGoToDefinition={onGoToDefinition ? handleGoToDefinition : undefined}
          onFindReferences={handleFindReferences}
          onClose={handleHoverClose}
        />
      )}
    </div>
  );
}

export default CodeViewer;
