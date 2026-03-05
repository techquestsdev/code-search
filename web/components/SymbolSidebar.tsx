"use client";

import { useEffect, useMemo, useReducer } from "react";
import { api, FileSymbol } from "@/lib/api";
import {
  Code,
  Box,
  Circle,
  Hash,
  ChevronRight,
  ChevronDown,
  Loader2,
  AlertCircle,
  Braces,
  Type,
  Variable,
  List,
  Search,
} from "lucide-react";

interface SymbolSidebarProps {
  repoId: number;
  path: string;
  language?: string;
  onSymbolClick?: (line: number) => void;
  onFindReferences?: (symbolName: string, line?: number) => void;
}

// Icon mapping for symbol kinds
const kindIcons: Record<string, React.ElementType> = {
  function: Code,
  method: Code,
  class: Box,
  struct: Box,
  interface: Braces,
  type: Type,
  enum: List,
  constant: Hash,
  variable: Variable,
  trait: Braces,
  module: Box,
  default: Circle,
};

// Color mapping for symbol kinds
const kindColors: Record<string, string> = {
  function: "text-purple-500",
  method: "text-purple-400",
  class: "text-yellow-500",
  struct: "text-yellow-400",
  interface: "text-blue-500",
  type: "text-blue-400",
  enum: "text-green-500",
  constant: "text-orange-500",
  variable: "text-gray-500",
  trait: "text-cyan-500",
  module: "text-pink-500",
};

interface GroupedSymbols {
  [parent: string]: FileSymbol[];
}

interface SymbolItemProps {
  symbol: FileSymbol;
  symbolKey: string;
  isHovered: boolean;
  hasChildren: boolean;
  isExpanded: boolean;
  onToggleGroup: (name: string) => void;
  onSymbolClick: (symbol: FileSymbol) => void;
  onHover: (key: string | null) => void;
  onFindReferences?: (name: string, line: number) => void;
  indent?: boolean;
}

function SymbolItem({
  symbol,
  symbolKey,
  isHovered,
  hasChildren,
  isExpanded,
  onToggleGroup,
  onSymbolClick,
  onHover,
  onFindReferences,
  indent = false,
}: SymbolItemProps) {
  const Icon = kindIcons[symbol.kind.toLowerCase()] || kindIcons.default;
  const color = kindColors[symbol.kind.toLowerCase()] || "text-gray-400";

  return (
    <button
      className={`w-full flex items-center gap-1 px-3 py-1.5 hover:bg-gray-100 dark:hover:bg-gray-700/50 transition-colors text-left group cursor-pointer ${
        indent ? "pl-4" : ""
      }`}
      onClick={() => {
        if (hasChildren) {
          onToggleGroup(symbol.name);
        }
        onSymbolClick(symbol);
      }}
      onKeyDown={(e) => {
        if (e.key === "Enter" || e.key === " ") {
          if (hasChildren) onToggleGroup(symbol.name);
          onSymbolClick(symbol);
        }
      }}
      onMouseEnter={() => onHover(symbolKey)}
      onMouseLeave={() => onHover(null)}
      title={symbol.signature || symbol.name}
    >
      {/* Expand/collapse chevron */}
      {hasChildren ? (
        isExpanded ? (
          <ChevronDown className="w-3 h-3 text-gray-400 flex-shrink-0" />
        ) : (
          <ChevronRight className="w-3 h-3 text-gray-400 flex-shrink-0" />
        )
      ) : !indent ? (
        <span className="w-3 flex-shrink-0" />
      ) : null}
      
      {/* Icon */}
      <Icon className={`w-4 h-4 flex-shrink-0 ${color}`} />
      
      {/* Symbol name - selectable */}
      <span
        className="truncate flex-1 min-w-0 select-text cursor-text"
        onClick={(e) => e.stopPropagation()}
        onKeyDown={(e) => { if (e.key === ' ') e.stopPropagation(); }}
        role="textbox"
        aria-readonly="true"
        tabIndex={-1}
        onDoubleClick={(e) => {
          e.stopPropagation();
          const selection = window.getSelection();
          const range = document.createRange();
          range.selectNodeContents(e.currentTarget);
          selection?.removeAllRanges();
          selection?.addRange(range);
        }}
      >
        {symbol.name}
      </span>
      
      {/* Line number - always visible, find references on hover */}
      <div className="flex items-center gap-1 flex-shrink-0 ml-1">
        {isHovered && onFindReferences ? (
          <button
            onClick={(e) => {
              e.stopPropagation();
              onFindReferences(symbol.name, symbol.line);
            }}
            className="p-0.5 text-gray-400 hover:text-blue-500 transition-colors"
            title="Find references"
          >
            <Search className="w-3.5 h-3.5" />
          </button>
        ) : null}
        <span className="text-xs text-gray-400 tabular-nums">:{symbol.line}</span>
      </div>
    </button>
  );
}

export function SymbolSidebar({ repoId, path, language: _language, onSymbolClick, onFindReferences }: SymbolSidebarProps) {
  type SymbolState = {
    symbols: FileSymbol[];
    loading: boolean;
    error: string | null;
    expandedGroups: Set<string>;
    hoveredSymbol: string | null;
  };
  const [state, updateState] = useReducer(
    (s: SymbolState, u: Partial<SymbolState>) => ({ ...s, ...u }),
    {
      symbols: [] as FileSymbol[],
      loading: true,
      error: null as string | null,
      expandedGroups: new Set([""]),
      hoveredSymbol: null as string | null,
    }
  );

  useEffect(() => {
    if (!path) {
      updateState({ symbols: [], loading: false, error: null });
      return;
    }

    updateState({ loading: true, error: null });

    const loadSymbols = async () => {
      try {
        const response = await api.getFileSymbols(repoId, path);
        const symbols = response.symbols || [];

        // Auto-expand all parent groups
        const parents = new Set<string>([""]);
        symbols.forEach(s => {
          if (s.parent) parents.add(s.parent);
        });

        updateState({
          symbols,
          expandedGroups: parents,
          loading: false,
          error: null,
        });
      } catch (err) {
        updateState({
          error: err instanceof Error ? err.message : "Failed to load symbols",
          symbols: [],
          loading: false,
        });
      }
    };

    loadSymbols();
  }, [repoId, path]);

  const { symbols, loading, error, expandedGroups, hoveredSymbol } = state;

  // Group symbols by parent (class/struct)
  const groupedSymbols = useMemo(() => {
    const groups: GroupedSymbols = { "": [] };

    symbols.forEach((symbol) => {
      const parent = symbol.parent || "";
      if (!groups[parent]) {
        groups[parent] = [];
      }
      // Don't add class/struct/interface to their own group
      if (symbol.kind === "class" || symbol.kind === "struct" || symbol.kind === "interface" || symbol.kind === "trait") {
        groups[""].push(symbol);
      } else {
        groups[parent].push(symbol);
      }
    });

    return groups;
  }, [symbols]);

  // Toggle group expansion
  const toggleGroup = (group: string) => {
    const next = new Set(expandedGroups);
    if (next.has(group)) {
      next.delete(group);
    } else {
      next.add(group);
    }
    updateState({ expandedGroups: next });
  };

  const setHoveredSymbol = (symbolKey: string | null) => {
    updateState({ hoveredSymbol: symbolKey });
  };

  // Handle symbol click
  const handleSymbolClick = (symbol: FileSymbol) => {
    onSymbolClick?.(symbol.line);
  };

  if (loading) {
    return (
      <div className="flex items-center justify-center py-8">
        <Loader2 className="w-5 h-5 animate-spin text-gray-400" />
      </div>
    );
  }

  if (error) {
    return (
      <div className="flex items-center gap-2 p-3 text-sm text-red-500">
        <AlertCircle className="w-4 h-4 flex-shrink-0" />
        <span className="truncate">{error}</span>
      </div>
    );
  }

  if (symbols.length === 0) {
    return (
      <div className="p-4 text-center text-sm text-gray-500 dark:text-gray-400">
        No symbols found
      </div>
    );
  }

  return (
    <div className="text-sm">
      {/* Top-level symbols (no parent) */}
      {groupedSymbols[""]?.map((symbol, index) => {
        const hasChildren = groupedSymbols[symbol.name]?.length > 0;
        const isExpanded = expandedGroups.has(symbol.name);
        const symbolKey = `${symbol.name}-${index}`;
        const isHovered = hoveredSymbol === symbolKey;

        return (
          <div key={symbolKey}>
            <SymbolItem
              symbol={symbol}
              symbolKey={symbolKey}
              isHovered={isHovered}
              hasChildren={hasChildren}
              isExpanded={isExpanded}
              onToggleGroup={toggleGroup}
              onSymbolClick={handleSymbolClick}
              onHover={setHoveredSymbol}
              onFindReferences={onFindReferences}
            />

            {/* Children (methods of this class/struct) */}
            {hasChildren && isExpanded && (
              <div className="ml-4 border-l border-gray-200 dark:border-gray-700">
                {groupedSymbols[symbol.name].map((child, childIndex) => {
                  const childKey = `${symbol.name}-${child.name}-${childIndex}`;
                  const isChildHovered = hoveredSymbol === childKey;

                  return (
                    <SymbolItem
                      key={childKey}
                      symbol={child}
                      symbolKey={childKey}
                      isHovered={isChildHovered}
                      hasChildren={false}
                      isExpanded={false}
                      onToggleGroup={toggleGroup}
                      onSymbolClick={handleSymbolClick}
                      onHover={setHoveredSymbol}
                      onFindReferences={onFindReferences}
                      indent
                    />
                  );
                })}
              </div>
            )}
          </div>
        );
      })}

      {/* Orphan symbols (have parent but parent wasn't found as a symbol) */}
      {Object.entries(groupedSymbols)
        .filter(([parent]) => parent !== "" && !symbols.some(s => s.name === parent))
        .map(([parent, children]) => (
          <div key={parent} className="mt-2">
            <div className="px-3 py-1 text-xs text-gray-500 dark:text-gray-400 font-medium">
              {parent}
            </div>
            {children.map((child, childIndex) => {
              const childKey = `orphan-${child.name}-${childIndex}`;
              const isHovered = hoveredSymbol === childKey;
              return (
                <SymbolItem
                  key={childKey}
                  symbol={child}
                  symbolKey={childKey}
                  isHovered={isHovered}
                  hasChildren={false}
                  isExpanded={false}
                  onToggleGroup={toggleGroup}
                  onSymbolClick={handleSymbolClick}
                  onHover={setHoveredSymbol}
                  onFindReferences={onFindReferences}
                  indent
                />
              );
            })}
          </div>
        ))}
    </div>
  );
}
