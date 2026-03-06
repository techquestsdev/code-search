import React from "react";

// GitHub theme colors for syntax highlighting (from @uiw/codemirror-theme-github)
// Light mode colors
export const githubLightColors = {
  keyword: "#d73a49",      // red - func, return, if, type keywords
  typeName: "#d73a49",     // red - type names like string, int, error (same as keyword)
  className: "#6f42c1",    // purple - class/struct names (PascalCase)
  variableName: "#005cc5", // blue - variables, parameters
  propertyName: "#6f42c1", // purple - property names
  string: "#032f62",       // dark blue - strings
  number: "#005cc5",       // blue - numbers
  operator: "#005cc5",     // blue - operators
  comment: "#6a737d",      // gray - comments, brackets
  bracket: "#6a737d",      // gray - (), {}, etc.
  atom: "#e36209",         // orange - bool, special
  foreground: "#24292e",   // default text color
};

// Dark mode colors
export const githubDarkColors = {
  keyword: "#ff7b72",      // coral red - func, return, if, type keywords
  typeName: "#ff7b72",     // coral red - type names like string, int, error
  className: "#d2a8ff",    // purple - class/struct names (PascalCase)
  variableName: "#79c0ff", // light blue - variables, parameters
  propertyName: "#d2a8ff", // purple - property names
  string: "#a5d6ff",       // light blue - strings
  number: "#79c0ff",       // light blue - numbers
  operator: "#79c0ff",     // light blue - operators
  comment: "#8b949e",      // gray - comments, brackets
  bracket: "#8b949e",      // gray - (), {}, etc.
  atom: "#ffab70",         // orange - bool, special
  foreground: "#c9d1d9",   // default text color
};

// Go keywords (red/coral)
const goKeywords = ['func', 'interface', 'struct', 'type', 'const', 'var', 'package', 'import', 'return', 'if', 'else', 'for', 'range', 'switch', 'case', 'default', 'break', 'continue', 'go', 'defer', 'chan', 'map', 'make', 'new', 'select', 'fallthrough', 'goto'];

// Built-in types (same red/coral as keywords in GitHub theme)
const goBuiltinTypes = ['string', 'int', 'int8', 'int16', 'int32', 'int64', 'uint', 'uint8', 'uint16', 'uint32', 'uint64', 'float32', 'float64', 'bool', 'byte', 'rune', 'error', 'any', 'uintptr', 'complex64', 'complex128'];

// Bool/special values (orange)
const goBoolValues = ['nil', 'true', 'false'];

// Basic syntax highlighting for Go code using GitHub colors
export function highlightGoCode(code: string, isDark: boolean): React.ReactNode {
  const colors = isDark ? githubDarkColors : githubLightColors;
  const parts: React.ReactNode[] = [];
  let keyIndex = 0;

  // Simple tokenizer regex - handles strings, comments, and identifiers
  const tokenRegex = /("(?:[^"\\]|\\.)*"|'(?:[^'\\]|\\.)*'|\/\/[^\n]*|\/\*[\s\S]*?\*\/|\w+|[()[\]{},.*:;=<>!&|+\-/]|\s+)/g;
  let match;

  while ((match = tokenRegex.exec(code)) !== null) {
    const token = match[1];
    const key = `token-${keyIndex++}`;

    // String literals
    if (token.startsWith('"') || token.startsWith("'")) {
      parts.push(<span key={key} style={{ color: colors.string }}>{token}</span>);
    }
    // Comments
    else if (token.startsWith('//') || token.startsWith('/*')) {
      parts.push(<span key={key} style={{ color: colors.comment }}>{token}</span>);
    }
    // Keywords
    else if (goKeywords.includes(token)) {
      parts.push(<span key={key} style={{ color: colors.keyword }}>{token}</span>);
    }
    // Built-in types
    else if (goBuiltinTypes.includes(token)) {
      parts.push(<span key={key} style={{ color: colors.typeName }}>{token}</span>);
    }
    // Bool/nil values
    else if (goBoolValues.includes(token)) {
      parts.push(<span key={key} style={{ color: colors.atom }}>{token}</span>);
    }
    // Numbers
    else if (token.match(/^\d+$/)) {
      parts.push(<span key={key} style={{ color: colors.number }}>{token}</span>);
    }
    // Pointer/dereference operator
    else if (token === '*' || token === '&') {
      parts.push(<span key={key} style={{ color: colors.keyword }}>{token}</span>);
    }
    // Type names (PascalCase) - purple
    else if (token.match(/^[A-Z][a-zA-Z0-9_]*$/)) {
      parts.push(<span key={key} style={{ color: colors.className }}>{token}</span>);
    }
    // Brackets and punctuation - gray
    else if (token.match(/^[()[\]{},.:;]$/)) {
      parts.push(<span key={key} style={{ color: colors.bracket }}>{token}</span>);
    }
    // Operators
    else if (token.match(/^[=<>!&|+\-/]+$/)) {
      parts.push(<span key={key} style={{ color: colors.operator }}>{token}</span>);
    }
    // Whitespace - preserve as is
    else if (token.match(/^\s+$/)) {
      parts.push(<span key={key}>{token}</span>);
    }
    // Variables and other identifiers - foreground color
    else {
      parts.push(<span key={key} style={{ color: colors.foreground }}>{token}</span>);
    }
  }

  return parts;
}

// Syntax highlighting based on language
export function highlightCode(code: string, language?: string, isDark?: boolean): React.ReactNode {
  if (!code) return null;

  const dark = isDark ?? false;
  const lang = language?.toLowerCase();

  if (lang === 'go') {
    return highlightGoCode(code, dark);
  }

  // For other languages, return plain text for now
  // Could add more language support here
  return code;
}
