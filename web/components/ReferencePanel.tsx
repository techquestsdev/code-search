"use client";

import { useState, useEffect, useCallback } from "react";
import { api } from "@/lib/api";
import { highlightCode } from "@/lib/syntax-highlight";
import { useTheme } from "./ThemeProvider";
import { X, Loader2, AlertCircle, FileCode, Search, Zap } from "lucide-react";

interface ReferencePanelProps {
  symbolName: string;
  language?: string;
  repos?: string[];
  // Optional SCIP position info for precise lookup
  line?: number;
  col?: number;
  filePath?: string;
  repoId?: number;
  onClose: () => void;
  onNavigate?: (repo: string, file: string, line: number) => void;
}

// Unified reference type for display
interface UnifiedReference {
  repo: string;
  file: string;
  line: number;
  column?: number;
  context?: string;
  isDefinition?: boolean;
}

export function ReferencePanel({
  symbolName,
  language,
  repos,
  line,
  col,
  filePath,
  repoId,
  onClose,
  onNavigate,
}: ReferencePanelProps) {
  const [references, setReferences] = useState<UnifiedReference[]>([]);
  const [definitions, setDefinitions] = useState<UnifiedReference[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [activeTab, setActiveTab] = useState<"definitions" | "references">(
    "definitions"
  );
  const [usingSCIP, setUsingSCIP] = useState(false);
  const [_repoName, setRepoName] = useState<string | null>(null);

  // Get current theme for syntax highlighting
  const { resolvedTheme } = useTheme();
  const isDark = resolvedTheme === "dark";

  // Load definitions and references
  useEffect(() => {
    const load = async () => {
      setLoading(true);
      setError(null);
      setUsingSCIP(false);

      // Get repo name for display if needed
      let displayRepoName = `repo-${repoId}`;
      if (repoId) {
        try {
          const repo = await api.getRepo(repoId);
          displayRepoName = repo.name;
          setRepoName(repo.name);
        } catch {
          // Ignore - fallback to ID display
        }
      }

      try {
        // Try SCIP first if we have position info
        if (repoId !== undefined && filePath && line !== undefined) {
          try {
            console.log("Trying SCIP lookup:", { repoId, filePath, line, col });
            const scipRefs = await api.getSCIPReferences(
              repoId,
              filePath,
              line,
              col ?? 0,
              200
            );
            console.log("SCIP response:", scipRefs);

            // Only use SCIP results if we found meaningful references (more than just the definition)
            // If SCIP only has 1-2 results, fall back to Zoekt which does text search
            // if (scipRefs.found && scipRefs.references.length > 2) {
            if (scipRefs.found && scipRefs.references.length > 0) {
              console.log("Using SCIP results");
              setUsingSCIP(true);

              // Convert SCIP references to unified format and filter out false positives
              // Only include references where the context actually contains the symbol name
              const unifiedRefs: UnifiedReference[] = scipRefs.references
                .filter((occ) => {
                  // Always include if no context (we can't verify)
                  if (!occ.context) return true;
                  // Include if context contains the symbol name
                  return occ.context.includes(symbolName);
                })
                .map((occ) => ({
                  repo: displayRepoName,
                  file: occ.filePath,
                  line: occ.startLine + 1, // Convert to 1-indexed
                  column: occ.startCol,
                  context: occ.context, // Use the context from SCIP API
                  isDefinition: (occ.role & 1) !== 0,
                }));

              // Separate definitions and references
              const defs = unifiedRefs.filter((r) => r.isDefinition);
              const refs = unifiedRefs.filter((r) => !r.isDefinition);

              setDefinitions(defs);
              setReferences(refs);

              // If no definitions found, switch to references tab
              if (defs.length === 0 && refs.length > 0) {
                setActiveTab("references");
              }
              return;
            }
            // else {
            //   console.debug("SCIP returned few results, falling back to Zoekt");
            // }
            console.log("SCIP did not return results, will fall back to Zoekt");
          } catch (scipErr) {
            // SCIP not available, fall through to Zoekt
            console.debug("SCIP lookup failed, using Zoekt:", scipErr);
          }
        } else {
          console.log("No SCIP position info, going straight to Zoekt");
        }

        // Fall back to Zoekt-based search
        // Don't use language filter for references - it's text-based and language detection can be wrong
        // (e.g., PHP detected as Hack)
        console.log("Falling back to Zoekt search:", {
          symbolName,
          language,
          repos,
        });
        const [defs, refs] = await Promise.all([
          api.findSymbols({
            name: symbolName,
            // Skip language filter - sym: search works better without it
            repos,
            limit: 50,
          }),
          api.findReferences({
            symbol: symbolName,
            // Skip language filter - text search works across all languages
            repos,
            limit: 100,
          }),
        ]);
        console.log("Zoekt results:", { defs: defs.length, refs: refs.length });

        // Convert to unified format
        const unifiedDefs: UnifiedReference[] = defs.map((def) => ({
          repo: def.repo,
          file: def.file,
          line: def.line,
          column: def.column,
          context: def.signature || def.name,
          isDefinition: true,
        }));

        const unifiedRefs: UnifiedReference[] = refs.map((ref) => ({
          repo: ref.repo,
          file: ref.file,
          line: ref.line,
          column: ref.column,
          context: ref.context,
        }));

        setDefinitions(unifiedDefs);
        setReferences(unifiedRefs);

        // If no definitions found, switch to references tab
        if (defs.length === 0 && refs.length > 0) {
          setActiveTab("references");
        }
      } catch (err) {
        setError(err instanceof Error ? err.message : "Failed to search");
      } finally {
        setLoading(false);
      }
    };

    if (symbolName) {
      load();
    }
  }, [symbolName, language, repos, line, col, filePath, repoId]);

  // Group references by file
  const groupedRefs = references.reduce(
    (acc, ref) => {
      const key = `${ref.repo}:${ref.file}`;
      if (!acc[key]) {
        acc[key] = { repo: ref.repo, file: ref.file, refs: [] };
      }
      acc[key].refs.push(ref);
      return acc;
    },
    {} as Record<
      string,
      { repo: string; file: string; refs: UnifiedReference[] }
    >
  );

  // Group definitions by file
  const groupedDefs = definitions.reduce(
    (acc, def) => {
      const key = `${def.repo}:${def.file}`;
      if (!acc[key]) {
        acc[key] = { repo: def.repo, file: def.file, defs: [] };
      }
      acc[key].defs.push(def);
      return acc;
    },
    {} as Record<
      string,
      { repo: string; file: string; defs: UnifiedReference[] }
    >
  );

  const handleClick = useCallback(
    (repo: string, file: string, line: number) => {
      if (onNavigate) {
        onNavigate(repo, file, line);
      }
    },
    [onNavigate]
  );

  return (
    <div className="flex h-full flex-col border-t border-gray-200 bg-white dark:border-gray-700 dark:bg-gray-800">
      {/* Header */}
      <div className="flex items-center justify-between border-b border-gray-200 bg-gray-50 px-4 py-2 dark:border-gray-700 dark:bg-gray-800/80">
        <div className="flex items-center gap-3">
          <Search className="h-4 w-4 text-gray-400" />
          <span className="text-sm font-medium text-gray-700 dark:text-gray-300">
            {symbolName}
          </span>
          {language && (
            <span className="rounded bg-gray-100 px-1.5 py-0.5 text-xs text-gray-500 dark:bg-gray-700 dark:text-gray-400">
              {language}
            </span>
          )}
          {usingSCIP && (
            <span className="flex items-center gap-1 rounded bg-green-50 px-1.5 py-0.5 text-xs text-green-600 dark:bg-green-900/30 dark:text-green-400">
              <Zap className="h-3 w-3" />
              SCIP
            </span>
          )}
        </div>
        <button
          onClick={onClose}
          className="p-1 text-gray-400 transition-colors hover:text-gray-600 dark:hover:text-gray-200"
        >
          <X className="h-4 w-4" />
        </button>
      </div>

      {/* Tabs */}
      <div className="flex border-b border-gray-200 dark:border-gray-700">
        <button
          onClick={() => setActiveTab("definitions")}
          className={`px-4 py-2 text-sm font-medium transition-colors ${
            activeTab === "definitions"
              ? "border-b-2 border-blue-600 text-blue-600 dark:border-blue-400 dark:text-blue-400"
              : "text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200"
          }`}
        >
          Definitions ({definitions.length})
        </button>
        <button
          onClick={() => setActiveTab("references")}
          className={`px-4 py-2 text-sm font-medium transition-colors ${
            activeTab === "references"
              ? "border-b-2 border-blue-600 text-blue-600 dark:border-blue-400 dark:text-blue-400"
              : "text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200"
          }`}
        >
          References ({references.length})
        </button>
      </div>

      {/* Content */}
      <div className="flex-1 overflow-y-auto">
        {loading ? (
          <div className="flex items-center justify-center py-8">
            <Loader2 className="h-5 w-5 animate-spin text-gray-400" />
            <span className="ml-2 text-sm text-gray-500">Searching...</span>
          </div>
        ) : error ? (
          <div className="flex items-center gap-2 p-4 text-sm text-red-500">
            <AlertCircle className="h-4 w-4 flex-shrink-0" />
            <span>{error}</span>
          </div>
        ) : activeTab === "definitions" ? (
          // Definitions list
          <div className="divide-y divide-gray-100 dark:divide-gray-700/50">
            {Object.values(groupedDefs).length === 0 ? (
              <div className="p-4 text-center text-sm text-gray-500 dark:text-gray-400">
                No definitions found
              </div>
            ) : (
              Object.values(groupedDefs).map(({ repo, file, defs }) => (
                <div key={`${repo}:${file}`} className="py-2">
                  <div className="flex items-center gap-2 px-4 py-1 text-xs text-gray-500 dark:text-gray-400">
                    <FileCode className="h-3 w-3" />
                    <span className="truncate" title={`${repo}/${file}`}>
                      {repo.split("/").pop()}/{file}
                    </span>
                  </div>
                  {defs.map((def, idx) => (
                    <button
                      key={idx}
                      onClick={() => handleClick(repo, file, def.line)}
                      className="w-full px-4 py-1.5 text-left transition-colors hover:bg-gray-50 dark:hover:bg-gray-700/50"
                    >
                      <div className="flex items-center gap-2">
                        <span className="w-10 flex-shrink-0 text-right font-mono text-xs text-gray-500 dark:text-gray-500">
                          :{def.line}
                        </span>
                        <code className="truncate font-mono text-sm">
                          {highlightCode(
                            def.context || symbolName,
                            language,
                            isDark
                          )}
                        </code>
                        {def.column !== undefined && (
                          <span className="ml-auto flex-shrink-0 text-xs text-gray-400">
                            col {def.column}
                          </span>
                        )}
                      </div>
                    </button>
                  ))}
                </div>
              ))
            )}
          </div>
        ) : (
          // References list
          <div className="divide-y divide-gray-100 dark:divide-gray-700/50">
            {Object.values(groupedRefs).length === 0 ? (
              <div className="p-4 text-center text-sm text-gray-500 dark:text-gray-400">
                No references found
              </div>
            ) : (
              Object.values(groupedRefs).map(({ repo, file, refs }) => (
                <div key={`${repo}:${file}`} className="py-2">
                  <div className="flex items-center gap-2 px-4 py-1 text-xs text-gray-500 dark:text-gray-400">
                    <FileCode className="h-3 w-3" />
                    <span className="truncate" title={`${repo}/${file}`}>
                      {repo.split("/").pop()}/{file}
                    </span>
                    <span className="ml-auto text-gray-400">
                      ({refs.length})
                    </span>
                  </div>
                  {refs.map((ref, idx) => (
                    <button
                      key={idx}
                      onClick={() => handleClick(repo, file, ref.line)}
                      className="w-full px-4 py-1.5 text-left transition-colors hover:bg-gray-50 dark:hover:bg-gray-700/50"
                    >
                      <div className="flex items-center gap-2">
                        <span className="w-10 flex-shrink-0 text-right font-mono text-xs text-gray-500 dark:text-gray-500">
                          :{ref.line}
                        </span>
                        <code className="truncate font-mono text-sm">
                          {highlightCode(
                            ref.context || symbolName,
                            language,
                            isDark
                          )}
                        </code>
                        {ref.column !== undefined && (
                          <span className="ml-auto flex-shrink-0 text-xs text-gray-400">
                            col {ref.column}
                          </span>
                        )}
                      </div>
                    </button>
                  ))}
                </div>
              ))
            )}
          </div>
        )}
      </div>
    </div>
  );
}
