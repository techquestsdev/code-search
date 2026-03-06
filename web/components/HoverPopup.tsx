"use client";

import { useState, useEffect, useRef, useMemo } from "react";
import { Code2, ArrowRight, Search, Loader2, X } from "lucide-react";
import { useTheme } from "./ThemeProvider";

interface HoverPopupProps {
  word: string;
  language?: string;
  signature?: string; // Raw documentation from SCIP (may contain code block + description)
  x: number;
  y: number;
  isLoadingDefinition?: boolean;
  onGoToDefinition?: () => void;
  onFindReferences?: () => void;
  onClose: () => void;
}

// Check if content is just a type definition without useful documentation
function isBareboneTypeDefinition(sig: string): boolean {
  const trimmed = sig.trim();

  // Empty struct definitions
  if (/^struct\s*\{\s*\}$/.test(trimmed)) return true;

  // Empty interface definitions
  if (/^interface\s*\{\s*\}$/.test(trimmed)) return true;

  // Just a basic type with no function signature
  if (/^(\[\]|\*|map[\w[\]]+)?[\w.]+$/.test(trimmed)) return true;

  return false;
}

// Format struct fields for readability (add newlines between fields)
function formatStructFields(sig: string): string {
  const structMatch = sig.match(/(struct\s*\{)([\s\S]*?)(\})/);
  if (!structMatch) return sig;

  const prefix = sig.slice(0, sig.indexOf("struct"));
  const structBody = structMatch[2].trim();

  // If already has newlines, don't reformat
  if (structBody.includes("\n")) {
    return sig;
  }

  // Match fields with optional backtick tags
  const fields: string[] = [];
  const fieldRegex = /(\w+)\s+([\w[\].*]+)\s*(`[^`]*`)?/g;
  let match;

  while ((match = fieldRegex.exec(structBody)) !== null) {
    const fieldName = match[1];
    const fieldType = match[2];
    const fieldTag = match[3] || "";
    fields.push(`  ${fieldName} ${fieldType}${fieldTag ? " " + fieldTag : ""}`);
  }

  if (fields.length === 0) {
    return sig;
  }

  return `${prefix}struct {\n${fields.join("\n")}\n}`;
}

// Parse documentation to extract signature and description
function parseDocumentation(doc: string | undefined): {
  signature: string | null;
  description: string | null;
} {
  if (!doc) return { signature: null, description: null };

  // Normalize the doc string - unescape common escape sequences
  let normalizedDoc = doc
    .replace(/\\n/g, "\n")
    .replace(/\\t/g, "\t")
    .replace(/\\`/g, "`");

  // Find ALL code blocks - use greedy matching to get the largest one for signature
  const allCodeBlocks = [...normalizedDoc.matchAll(/```(\w*)\s*([\s\S]*?)```/g)];

  let signature: string | null = null;
  let description: string | null = null;

  if (allCodeBlocks.length > 0) {
    // Find the most complete code block (largest with actual code content)
    let bestBlock = allCodeBlocks[0];
    for (const block of allCodeBlocks) {
      const content = block[2].trim();
      // Prefer blocks with struct/func definitions
      if (
        content.includes("struct {") ||
        content.includes("func ") ||
        content.length > bestBlock[2].length
      ) {
        bestBlock = block;
      }
    }

    let extractedSig = bestBlock[2].trim();

    // Fix escaped quotes from SCIP indexers
    extractedSig = extractedSig.replace(/\\"/g, '"');

    // Convert struct tags from SCIP format to Go backtick format
    extractedSig = extractedSig.replace(/"(\w+):"([^"]*)""?/g, '`$1:"$2"`');

    // Format struct fields
    if (extractedSig.includes("struct {")) {
      extractedSig = formatStructFields(extractedSig);
    }

    // Remove ALL code blocks from doc to extract plain text description
    let textOnly = normalizedDoc;
    for (const block of allCodeBlocks) {
      textOnly = textOnly.replace(block[0], " ");
    }
    textOnly = textOnly.trim();

    // Clean up multiple spaces
    textOnly = textOnly.replace(/\s+/g, " ").trim();

    if (textOnly && textOnly.length > 0) {
      description = textOnly;
    }

    if (!isBareboneTypeDefinition(extractedSig) || description) {
      signature = extractedSig;
    }
  } else {
    // No code block found - try to separate description from code
    let text = normalizedDoc.trim();

    // Fix escaped quotes
    text = text.replace(/\\"/g, '"');
    text = text.replace(/"(\w+):"([^"]*)""?/g, '`$1:"$2"`');

    // Check if it starts with a description followed by code keywords
    const codeKeywords = /\b(func|type|struct|interface|const|var)\s/;
    const keywordMatch = text.match(codeKeywords);

    if (keywordMatch && keywordMatch.index && keywordMatch.index > 0) {
      description = text.slice(0, keywordMatch.index).trim();
      let codePart = text.slice(keywordMatch.index).trim();

      if (codePart.includes("struct {")) {
        codePart = formatStructFields(codePart);
      }
      signature = codePart;
    } else if (codeKeywords.test(text)) {
      if (text.includes("struct {")) {
        text = formatStructFields(text);
      }
      signature = text;
    } else if (!isBareboneTypeDefinition(text)) {
      if (/\w+\s*\(.*\)/.test(text) || /^\w+\s+\w+/.test(text)) {
        signature = text;
      } else {
        description = text;
      }
    }
  }

  return { signature, description };
}

// GitHub theme colors for syntax highlighting
const githubLightColors = {
  keyword: "#d73a49",
  typeName: "#d73a49",
  className: "#6f42c1",
  variableName: "#005cc5",
  propertyName: "#6f42c1",
  string: "#032f62",
  number: "#005cc5",
  operator: "#005cc5",
  comment: "#6a737d",
  bracket: "#6a737d",
  atom: "#e36209",
  foreground: "#24292e",
};

const githubDarkColors = {
  keyword: "#ff7b72",
  typeName: "#ff7b72",
  className: "#d2a8ff",
  variableName: "#79c0ff",
  propertyName: "#d2a8ff",
  string: "#a5d6ff",
  number: "#79c0ff",
  operator: "#79c0ff",
  comment: "#8b949e",
  bracket: "#8b949e",
  atom: "#ffab70",
  foreground: "#c9d1d9",
};

// Basic syntax highlighting for Go function signatures
function highlightGoSignature(
  signature: string,
  isDark: boolean
): React.ReactNode {
  const colors = isDark ? githubDarkColors : githubLightColors;
  const parts: React.ReactNode[] = [];
  let keyIndex = 0;

  const keywords = [
    "func",
    "interface",
    "struct",
    "type",
    "const",
    "var",
    "package",
    "import",
    "return",
    "if",
    "else",
    "for",
    "range",
    "switch",
    "case",
    "default",
    "break",
    "continue",
    "go",
    "defer",
    "chan",
    "map",
    "make",
    "new",
  ];

  const builtinTypes = [
    "string",
    "int",
    "int8",
    "int16",
    "int32",
    "int64",
    "uint",
    "uint8",
    "uint16",
    "uint32",
    "uint64",
    "float32",
    "float64",
    "bool",
    "byte",
    "rune",
    "error",
    "any",
    "uintptr",
    "complex64",
    "complex128",
  ];

  const boolValues = ["nil", "true", "false"];

  // Improved tokenizer that handles struct tags, strings, and := operator
  const tokenRegex = /(`[^`]*`|"[^"]*"|:=|[()[\]{},.*:]|\w+|\s+)/g;
  let match;

  while ((match = tokenRegex.exec(signature)) !== null) {
    const token = match[0];
    const key = `token-${keyIndex++}`;

    if (token.startsWith("`") && token.endsWith("`")) {
      // Struct tag (backtick string)
      parts.push(
        <span key={key} style={{ color: colors.string }}>
          {token}
        </span>
      );
    } else if (token.startsWith('"') && token.endsWith('"')) {
      // String literal
      parts.push(
        <span key={key} style={{ color: colors.string }}>
          {token}
        </span>
      );
    } else if (token === ":=") {
      // Short variable declaration operator
      parts.push(
        <span key={key} style={{ color: colors.operator }}>
          {token}
        </span>
      );
    } else if (keywords.includes(token)) {
      parts.push(
        <span key={key} style={{ color: colors.keyword }}>
          {token}
        </span>
      );
    } else if (builtinTypes.includes(token)) {
      parts.push(
        <span key={key} style={{ color: colors.typeName }}>
          {token}
        </span>
      );
    } else if (boolValues.includes(token)) {
      parts.push(
        <span key={key} style={{ color: colors.atom }}>
          {token}
        </span>
      );
    } else if (/^\d+$/.test(token)) {
      parts.push(
        <span key={key} style={{ color: colors.number }}>
          {token}
        </span>
      );
    } else if (token === "*") {
      parts.push(
        <span key={key} style={{ color: colors.keyword }}>
          {token}
        </span>
      );
    } else if (/^[A-Z][a-zA-Z0-9_]*$/.test(token)) {
      // Type names (PascalCase)
      parts.push(
        <span key={key} style={{ color: colors.className }}>
          {token}
        </span>
      );
    } else if (/^[()[\]{},.:*]$/.test(token)) {
      parts.push(
        <span key={key} style={{ color: colors.bracket }}>
          {token}
        </span>
      );
    } else if (/^\s+$/.test(token)) {
      parts.push(<span key={key}>{token}</span>);
    } else {
      parts.push(
        <span key={key} style={{ color: colors.foreground }}>
          {token}
        </span>
      );
    }
  }

  return parts;
}

// Syntax highlighting based on language
function highlightSignature(
  signature: string,
  language?: string,
  isDark?: boolean
): React.ReactNode {
  if (!language) return signature;

  const lang = language.toLowerCase();
  if (lang === "go") {
    return highlightGoSignature(signature, isDark ?? false);
  }

  return signature;
}

export function HoverPopup({
  word,
  language,
  signature: rawSignature,
  x,
  y,
  isLoadingDefinition,
  onGoToDefinition,
  onFindReferences,
  onClose,
}: HoverPopupProps) {
  const popupRef = useRef<HTMLDivElement>(null);
  const [position, setPosition] = useState({ x, y });

  const { resolvedTheme } = useTheme();
  const isDark = resolvedTheme === "dark";

  const { signature, description } = useMemo(
    () => parseDocumentation(rawSignature),
    [rawSignature]
  );

  const highlightedSignature = useMemo(() => {
    if (!signature) return null;
    return highlightSignature(signature, language, isDark);
  }, [signature, language, isDark]);

  // Calculate initial position and adjust for viewport bounds
  useEffect(() => {
    if (!popupRef.current) return;

    const rect = popupRef.current.getBoundingClientRect();
    const viewportWidth = window.innerWidth;
    const viewportHeight = window.innerHeight;

    let newX = x;
    let newY = y;

    if (newX + rect.width > viewportWidth - 16) {
      newX = viewportWidth - rect.width - 16;
    }
    if (newX < 16) newX = 16;

    if (newY + rect.height > viewportHeight - 16) {
      newY = y - rect.height - 24;
    }
    if (newY < 16) newY = 16;

    setPosition({ x: newX, y: newY });
  }, [x, y]);

  // Close popup when scrolling (standard IDE behavior)
  useEffect(() => {
    const handleScroll = () => {
      onClose();
    };

    const scroller = document.querySelector('.cm-scroller');
    if (scroller) {
      scroller.addEventListener('scroll', handleScroll, { passive: true });
    }

    return () => {
      if (scroller) {
        scroller.removeEventListener('scroll', handleScroll);
      }
    };
  }, [onClose]);

  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === "Escape") {
        onClose();
      }
    };
    document.addEventListener("keydown", handleKeyDown);
    return () => document.removeEventListener("keydown", handleKeyDown);
  }, [onClose]);

  useEffect(() => {
    const handleClickOutside = (e: MouseEvent) => {
      if (popupRef.current && !popupRef.current.contains(e.target as Node)) {
        onClose();
      }
    };
    const timeoutId = setTimeout(() => {
      document.addEventListener("mousedown", handleClickOutside);
    }, 100);
    return () => {
      clearTimeout(timeoutId);
      document.removeEventListener("mousedown", handleClickOutside);
    };
  }, [onClose]);

  return (
    <div
      ref={popupRef}
      className="fixed z-50 bg-white dark:bg-gray-800 rounded-lg shadow-xl border border-gray-200 dark:border-gray-700 overflow-hidden min-w-[300px] max-w-[500px]"
      style={{
        left: `${position.x}px`,
        top: `${position.y}px`,
      }}
    >
      {/* Header with word and language */}
      <div className="px-3 py-2 bg-gray-50 dark:bg-gray-900/50 border-b border-gray-200 dark:border-gray-700">
        <div className="flex items-center justify-between mb-1">
          <div className="flex items-center gap-2">
            <Code2 className="w-4 h-4 text-blue-500" />
            <span className="font-mono font-semibold text-sm text-gray-900 dark:text-gray-100">
              {word}
            </span>
            {language && (
              <span className="text-xs px-1.5 py-0.5 bg-blue-100 dark:bg-blue-900/50 text-blue-700 dark:text-blue-300 rounded font-medium">
                {language}
              </span>
            )}
          </div>
          <button
            onClick={onClose}
            className="p-1 hover:bg-gray-200 dark:hover:bg-gray-600 rounded transition-colors"
          >
            <X className="w-3.5 h-3.5 text-gray-400" />
          </button>
        </div>

        {/* Signature with syntax highlighting */}
        {isLoadingDefinition && !signature && (
          <div className="flex items-center gap-2 mt-2">
            <Loader2 className="w-3.5 h-3.5 animate-spin text-gray-400" />
            <span className="text-xs text-gray-500 dark:text-gray-400">
              Loading signature...
            </span>
          </div>
        )}
        {highlightedSignature && (
          <div className="mt-2 p-2 bg-gray-100 dark:bg-gray-800 rounded border border-gray-200 dark:border-gray-700">
            <code className="text-xs block whitespace-pre-wrap break-all font-mono text-gray-800 dark:text-gray-200 leading-relaxed">
              {highlightedSignature}
            </code>
          </div>
        )}
      </div>

      {/* Description/Documentation */}
      {description && (
        <div className="px-3 py-2 border-b border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-800">
          <p className="text-xs text-gray-600 dark:text-gray-300 leading-relaxed">
            {description}
          </p>
        </div>
      )}

      {/* Actions */}
      <div className="p-2 flex gap-2 bg-white dark:bg-gray-800">
        {onGoToDefinition && (
          <button
            onClick={() => {
              onGoToDefinition();
              onClose();
            }}
            disabled={isLoadingDefinition}
            className="flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium bg-blue-50 hover:bg-blue-100 dark:bg-blue-900/30 dark:hover:bg-blue-900/50 text-blue-600 dark:text-blue-400 rounded transition-colors disabled:opacity-50"
          >
            {isLoadingDefinition ? (
              <Loader2 className="w-3.5 h-3.5 animate-spin" />
            ) : (
              <ArrowRight className="w-3.5 h-3.5" />
            )}
            Go to Definition
          </button>
        )}
        {onFindReferences && (
          <button
            onClick={() => {
              onFindReferences();
              onClose();
            }}
            className="flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium bg-gray-100 hover:bg-gray-200 dark:bg-gray-700 dark:hover:bg-gray-600 text-gray-700 dark:text-gray-300 rounded transition-colors"
          >
            <Search className="w-3.5 h-3.5" />
            Find References
          </button>
        )}
      </div>

      {/* Keyboard hint */}
      <div className="px-3 py-1.5 bg-gray-50 dark:bg-gray-700/30 border-t border-gray-200 dark:border-gray-700">
        <span className="text-xs text-gray-400">
          Press{" "}
          <kbd className="px-1 py-0.5 bg-gray-200 dark:bg-gray-600 rounded text-[10px]">
            Esc
          </kbd>{" "}
          to close
        </span>
      </div>
    </div>
  );
}

export default HoverPopup;
