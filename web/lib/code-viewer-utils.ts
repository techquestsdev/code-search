import { javascript } from "@codemirror/lang-javascript";
import { python } from "@codemirror/lang-python";
import { go } from "@codemirror/lang-go";
import { java } from "@codemirror/lang-java";
import { rust } from "@codemirror/lang-rust";
import { cpp } from "@codemirror/lang-cpp";
import { sql } from "@codemirror/lang-sql";
import { json } from "@codemirror/lang-json";
import { markdown } from "@codemirror/lang-markdown";
import { html } from "@codemirror/lang-html";
import { css } from "@codemirror/lang-css";
import { xml } from "@codemirror/lang-xml";
import { yaml } from "@codemirror/lang-yaml";
import { php } from "@codemirror/lang-php";
import { LanguageSupport } from "@codemirror/language";
import { Extension, Text } from "@codemirror/state";
import { EditorView, ViewPlugin, ViewUpdate, keymap, Decoration } from "@codemirror/view";

// Re-export heavy imports so consumers don't need direct @codemirror dependencies
export { EditorView };
export { Text, EditorState } from "@codemirror/state";
export type { Extension } from "@codemirror/state";

// Word info interface
export interface WordInfo {
  word: string;
  line: number;
  startCol: number;
  endCol: number;
}

// Hover state for popup
export interface HoverState {
  word: string;
  line: number;
  col: number;
  x: number;
  y: number;
  signature?: string;
  isLoading: boolean;
}

// Get word at specific position in doc
export function getWordAtPosition(doc: Text, pos: number): WordInfo | null {
  const line = doc.lineAt(pos);
  const lineText = line.text;
  const linePos = pos - line.from;

  // Find word boundaries (alphanumeric + underscore)
  let start = linePos;
  let end = linePos;

  // Go backwards to find start of word
  while (start > 0 && /[a-zA-Z0-9_]/.test(lineText[start - 1])) {
    start--;
  }

  // Go forwards to find end of word
  while (end < lineText.length && /[a-zA-Z0-9_]/.test(lineText[end])) {
    end++;
  }

  if (start === end) return null;

  const word = lineText.slice(start, end);
  // Only return if it looks like an identifier (starts with letter or underscore)
  if (/^[a-zA-Z_]/.test(word)) {
    return {
      word,
      line: line.number,
      startCol: start,
      endCol: end,
    };
  }
  return null;
}

// Create cursor tracking extension
export function createCursorTrackingExtension(
  onCursorChange: (word: WordInfo | null) => void
): Extension {
  return ViewPlugin.fromClass(
    class {
      update(update: ViewUpdate) {
        if (update.selectionSet || update.docChanged) {
          const { state } = update;
          const selection = state.selection.main;

          // Get word at cursor position
          const wordInfo = getWordAtPosition(state.doc, selection.head);
          onCursorChange(wordInfo);
        }
      }
    }
  );
}

// Create hover extension
export function createHoverExtension(
  showHoverPopup: boolean,
  language: string | undefined,
  fetchInfo: (word: string, line: number, col: number) => Promise<string | undefined>,
  onHoverStateChange: (state: HoverState | null | ((prev: HoverState | null) => HoverState | null)) => void,
  onHoverAction?: (word: string, line: number, x: number, y: number) => void
): Extension {
  if (!showHoverPopup) return [];

  let hoverTimeout: NodeJS.Timeout | null = null;

  return EditorView.domEventHandlers({
    mousemove: (event, view) => {
      // Clear existing timeout
      if (hoverTimeout) {
        clearTimeout(hoverTimeout);
        hoverTimeout = null;
      }

      // Only show hover on Cmd/Ctrl held
      if (!event.metaKey && !event.ctrlKey) {
        return false;
      }

      const pos = view.posAtCoords({ x: event.clientX, y: event.clientY });
      if (pos === null) return false;

      const wordInfo = getWordAtPosition(view.state.doc, pos);
      if (!wordInfo || isReservedKeyword(wordInfo.word, language)) return false;

      // Calculate the word's position in the editor
      const line = view.state.doc.line(wordInfo.line);
      const wordStartPos = line.from + wordInfo.startCol;
      const wordEndPos = line.from + wordInfo.endCol;

      const startCoords = view.coordsAtPos(wordStartPos);
      const endCoords = view.coordsAtPos(wordEndPos);

      if (!startCoords || !endCoords) return false;

      const popupX = startCoords.left;
      const popupY = startCoords.bottom + 4; // 4px below the line

      hoverTimeout = setTimeout(async () => {
        onHoverStateChange({
          word: wordInfo.word,
          line: wordInfo.line,
          col: wordInfo.startCol,
          x: popupX,
          y: popupY,
          isLoading: true,
        });

        const signature = await fetchInfo(wordInfo.word, wordInfo.line, wordInfo.startCol);

        onHoverStateChange((prev: HoverState | null) => {
          if (prev && prev.word === wordInfo.word && prev.line === wordInfo.line) {
            return { ...prev, signature, isLoading: false };
          }
          return prev;
        });

        if (onHoverAction) {
          onHoverAction(wordInfo.word, wordInfo.line, popupX, popupY);
        }
      }, 300);

      return false;
    },
    mouseleave: () => {
      if (hoverTimeout) {
        clearTimeout(hoverTimeout);
        hoverTimeout = null;
      }
      return false;
    },
  });
}

// Create click handler extension
export function createClickExtension(
  onWordClick?: (word: string, line: number, col: number) => void,
  language?: string
): Extension {
  if (!onWordClick) return [];

  return EditorView.domEventHandlers({
    click: (event, view) => {
      // Check for Cmd (Mac) or Ctrl (Windows/Linux)
      if (event.metaKey || event.ctrlKey) {
        const pos = view.posAtCoords({ x: event.clientX, y: event.clientY });
        if (pos !== null) {
          const wordInfo = getWordAtPosition(view.state.doc, pos);
          if (wordInfo && !isReservedKeyword(wordInfo.word, language)) {
            onWordClick(wordInfo.word, wordInfo.line, wordInfo.startCol);
            event.preventDefault();
            return true;
          }
        }
      }
      return false;
    },
  });
}

// Reserved keywords by language that should not trigger navigation
export const RESERVED_KEYWORDS: Record<string, Set<string>> = {
  go: new Set([
    "break", "case", "chan", "const", "continue", "default", "defer", "else",
    "fallthrough", "for", "func", "go", "goto", "if", "import", "interface",
    "map", "package", "range", "return", "select", "struct", "switch", "type",
    "var", "true", "false", "nil", "iota", "append", "cap", "close", "complex",
    "copy", "delete", "imag", "len", "make", "new", "panic", "print", "println",
    "real", "recover", "string", "int", "int8", "int16", "int32", "int64",
    "uint", "uint8", "uint16", "uint32", "uint64", "uintptr", "float32", "float64",
    "complex64", "complex128", "byte", "rune", "bool", "error", "any",
  ]),
  typescript: new Set([
    "abstract", "any", "as", "async", "await", "boolean", "break", "case",
    "catch", "class", "const", "constructor", "continue", "debugger", "declare",
    "default", "delete", "do", "else", "enum", "export", "extends", "false",
    "finally", "for", "from", "function", "get", "if", "implements", "import",
    "in", "instanceof", "interface", "is", "keyof", "let", "module", "namespace",
    "never", "new", "null", "number", "object", "of", "package", "private",
    "protected", "public", "readonly", "require", "return", "set", "static",
    "string", "super", "switch", "symbol", "this", "throw", "true", "try",
    "type", "typeof", "undefined", "unique", "unknown", "var", "void", "while",
    "with", "yield", "bigint",
  ]),
  javascript: new Set([
    "abstract", "arguments", "async", "await", "boolean", "break", "byte",
    "case", "catch", "char", "class", "const", "continue", "debugger", "default",
    "delete", "do", "double", "else", "enum", "eval", "export", "extends",
    "false", "final", "finally", "float", "for", "function", "goto", "if",
    "implements", "import", "in", "instanceof", "int", "interface", "let",
    "long", "native", "new", "null", "package", "private", "protected", "public",
    "return", "short", "static", "super", "switch", "synchronized", "this",
    "throw", "throws", "transient", "true", "try", "typeof", "undefined", "var",
    "void", "volatile", "while", "with", "yield",
  ]),
  python: new Set([
    "False", "None", "True", "and", "as", "assert", "async", "await", "break",
    "class", "continue", "def", "del", "elif", "else", "except", "finally",
    "for", "from", "global", "if", "import", "in", "is", "lambda", "nonlocal",
    "not", "or", "pass", "raise", "return", "try", "while", "with", "yield",
    "int", "float", "str", "bool", "list", "dict", "set", "tuple", "type",
    "print", "len", "range", "open", "self", "cls",
  ]),
  java: new Set([
    "abstract", "assert", "boolean", "break", "byte", "case", "catch", "char",
    "class", "const", "continue", "default", "do", "double", "else", "enum",
    "extends", "final", "finally", "float", "for", "goto", "if", "implements",
    "import", "instanceof", "int", "interface", "long", "native", "new", "null",
    "package", "private", "protected", "public", "return", "short", "static",
    "strictfp", "super", "switch", "synchronized", "this", "throw", "throws",
    "transient", "try", "void", "volatile", "while", "true", "false", "var",
    "record", "sealed", "permits", "yield",
  ]),
  rust: new Set([
    "as", "async", "await", "break", "const", "continue", "crate", "dyn",
    "else", "enum", "extern", "false", "fn", "for", "if", "impl", "in", "let",
    "loop", "match", "mod", "move", "mut", "pub", "ref", "return", "self",
    "Self", "static", "struct", "super", "trait", "true", "type", "unsafe",
    "use", "where", "while", "abstract", "become", "box", "do", "final",
    "macro", "override", "priv", "typeof", "unsized", "virtual", "yield",
    "i8", "i16", "i32", "i64", "i128", "isize", "u8", "u16", "u32", "u64",
    "u128", "usize", "f32", "f64", "bool", "char", "str", "String", "Vec",
    "Option", "Result", "Some", "None", "Ok", "Err",
  ]),
  cpp: new Set([
    "alignas", "alignof", "and", "and_eq", "asm", "auto", "bitand", "bitor",
    "bool", "break", "case", "catch", "char", "char8_t", "char16_t", "char32_t",
    "class", "compl", "concept", "const", "consteval", "constexpr", "constinit",
    "const_cast", "continue", "co_await", "co_return", "co_yield", "decltype",
    "default", "delete", "do", "double", "dynamic_cast", "else", "enum",
    "explicit", "export", "extern", "false", "float", "for", "friend", "goto",
    "if", "inline", "int", "long", "mutable", "namespace", "new", "noexcept",
    "not", "not_eq", "nullptr", "operator", "or", "or_eq", "private", "protected",
    "public", "register", "reinterpret_cast", "requires", "return", "short",
    "signed", "sizeof", "static", "static_assert", "static_cast", "struct",
    "switch", "template", "this", "thread_local", "throw", "true", "try",
    "typedef", "typeid", "typename", "union", "unsigned", "using", "virtual",
    "void", "volatile", "wchar_t", "while", "xor", "xor_eq",
  ]),
  php: new Set([
    "abstract", "and", "array", "as", "break", "callable", "case", "catch",
    "class", "clone", "const", "continue", "declare", "default", "die", "do",
    "echo", "else", "elseif", "empty", "enddeclare", "endfor", "endforeach",
    "endif", "endswitch", "endwhile", "eval", "exit", "extends", "final",
    "finally", "for", "foreach", "function", "global", "goto", "if", "implements",
    "include", "include_once", "instanceof", "insteadof", "interface", "isset",
    "list", "namespace", "new", "or", "print", "private", "protected", "public",
    "require", "require_once", "return", "static", "switch", "throw", "trait",
    "try", "unset", "use", "var", "while", "xor", "yield",
  ]),
};

// Check if a word is a reserved keyword for a language
export function isReservedKeyword(word: string, language?: string): boolean {
  if (!language) return false;
  const lang = language.toLowerCase();

  // Check the exact language
  if (RESERVED_KEYWORDS[lang]?.has(word)) return true;

  // Check related languages
  if ((lang === "tsx" || lang === "jsx") && RESERVED_KEYWORDS["typescript"]?.has(word)) return true;

  return false;
}

// Language mode mapping from backend to CodeMirror extensions
export const languageExtensions: Record<string, () => LanguageSupport> = {
  javascript: () => javascript(),
  typescript: () => javascript({ typescript: true }),
  jsx: () => javascript({ jsx: true }),
  tsx: () => javascript({ jsx: true, typescript: true }),
  python: () => python(),
  go: () => go(),
  java: () => java(),
  rust: () => rust(),
  cpp: () => cpp(),
  csharp: () => cpp(), // C# can use cpp highlighting as fallback
  sql: () => sql(),
  json: () => json(),
  markdown: () => markdown(),
  html: () => html(),
  css: () => css(),
  xml: () => xml(),
  yaml: () => yaml(),
  php: () => php(),
  hack: () => php(), // Hack is similar to PHP
  // Fallback for common languages
  shell: () => markdown(), // Basic highlighting
  dockerfile: () => markdown(),
  text: () => markdown(),
};

// Create extension for line number gutter clicks using ViewPlugin
export function createLineNumberClickExtension(onLineClick?: (line: number) => void): Extension {
  if (!onLineClick) return [];

  return ViewPlugin.define((view) => {
    const handleClick = (event: MouseEvent) => {
      const target = event.target as HTMLElement;

      // Check if the click target is inside the line numbers gutter
      if (target.closest('.cm-lineNumbers')) {
        // Find the gutter element
        const gutterElement = target.classList.contains('cm-gutterElement')
          ? target
          : target.closest('.cm-gutterElement');

        if (gutterElement) {
          const lineText = gutterElement.textContent?.trim();
          if (lineText) {
            const lineNumber = parseInt(lineText, 10);
            if (!isNaN(lineNumber) && lineNumber > 0) {
              event.preventDefault();
              event.stopPropagation();
              onLineClick(lineNumber);
            }
          }
        }
      }
    };

    // Attach click handler to the editor's DOM
    view.dom.addEventListener('click', handleClick, true);

    return {
      destroy() {
        view.dom.removeEventListener('click', handleClick, true);
      }
    };
  });
}

// Line highlighting extension
const highlightedLineDecoration = Decoration.line({
  class: "cm-highlighted-line",
});

export function createHighlightExtension(
  highlightLines: number[]
): Extension {
  return EditorView.decorations.compute([], (state) => {
    const decorations: ReturnType<typeof highlightedLineDecoration.range>[] = [];
    const doc = state.doc;

    for (const lineNum of highlightLines) {
      if (lineNum >= 1 && lineNum <= doc.lines) {
        const line = doc.line(lineNum);
        decorations.push(highlightedLineDecoration.range(line.from));
      }
    }

    return Decoration.set(decorations);
  });
}

// Custom theme for highlighted lines and dark mode fixes
export const highlightTheme = EditorView.baseTheme({
  ".cm-highlighted-line": {
    backgroundColor: "rgba(255, 213, 79, 0.3) !important",
  },
  "&.cm-focused .cm-highlighted-line": {
    backgroundColor: "rgba(255, 213, 79, 0.4) !important",
  },
  // Clickable word styling when holding Cmd/Ctrl
  ".cm-content": {
    cursor: "text",
  },
});

// Dark mode specific overrides for GitHub theme
export const darkModeOverrides = EditorView.theme({
  "&": {
    backgroundColor: "#0d1117",
  },
  ".cm-gutters": {
    backgroundColor: "#0d1117",
    borderRight: "1px solid #30363d",
  },
  ".cm-lineNumbers .cm-gutterElement": {
    color: "#6e7681",
    cursor: "pointer",
    "&:hover": {
      color: "#58a6ff",
    },
  },
  ".cm-activeLineGutter": {
    backgroundColor: "#161b22",
  },
  ".cm-activeLine": {
    backgroundColor: "#161b22",
  },
  ".cm-highlighted-line": {
    backgroundColor: "rgba(187, 128, 9, 0.15) !important",
  },
}, { dark: true });

// Light mode highlighted line
export const lightModeOverrides = EditorView.theme({
  ".cm-highlighted-line": {
    backgroundColor: "rgba(255, 213, 79, 0.3) !important",
  },
  ".cm-lineNumbers .cm-gutterElement": {
    cursor: "pointer",
    "&:hover": {
      color: "#0969da",
    },
  },
}, { dark: false });

// Create keyboard shortcuts as a CodeMirror keymap extension
export function createKeyboardShortcutsExtension(options: {
  language?: string;
  onGoToDefinition?: (word: string, line: number, col: number) => void;
  onFindReferences?: (word: string, line: number, col: number) => void;
}): Extension {
  const { language, onGoToDefinition, onFindReferences } = options;

  const bindings = [];

  // F12 - Go to Definition
  if (onGoToDefinition) {
    bindings.push({
      key: "F12",
      run: (view: EditorView): boolean => {
        const selection = view.state.selection.main;
        const wordInfo = getWordAtPosition(view.state.doc, selection.head);
        if (wordInfo && !isReservedKeyword(wordInfo.word, language)) {
          onGoToDefinition(wordInfo.word, wordInfo.line, wordInfo.startCol);
          return true;
        }
        return false;
      },
    });
  }

  // Shift+F12 - Find References
  if (onFindReferences) {
    bindings.push({
      key: "Shift-F12",
      run: (view: EditorView): boolean => {
        const selection = view.state.selection.main;
        const wordInfo = getWordAtPosition(view.state.doc, selection.head);
        if (wordInfo && !isReservedKeyword(wordInfo.word, language)) {
          onFindReferences(wordInfo.word, wordInfo.line, wordInfo.startCol);
          return true;
        }
        return false;
      },
    });
  }

  if (bindings.length === 0) return [];
  return keymap.of(bindings);
}
