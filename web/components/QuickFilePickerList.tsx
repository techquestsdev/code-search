import React from "react";
import { Folder, FileCode } from "lucide-react";

interface FileEntry {
  path: string;
  name: string;
  type: "file" | "dir";
  language?: string;
}

interface QuickFilePickerListProps {
  filteredFiles: FileEntry[];
  loading: boolean;
  selectedIndex: number;
  query: string;
  onSelect: (path: string) => void;
  onClose: () => void;
  listRef: React.RefObject<HTMLDivElement | null>;
}

// Highlight matching text in results
function highlightMatch(text: string, query: string): React.ReactNode {
  if (!query.trim()) return text;

  const lowerText = text.toLowerCase();
  const lowerQuery = query.toLowerCase();
  const index = lowerText.indexOf(lowerQuery);

  if (index === -1) return text;

  return (
    <>
      {text.slice(0, index)}
      <mark className="bg-yellow-200 dark:bg-yellow-500/40 text-inherit rounded">
        {text.slice(index, index + query.length)}
      </mark>
      {text.slice(index + query.length)}
    </>
  );
}

export function QuickFilePickerList({
  filteredFiles,
  loading,
  selectedIndex,
  query,
  onSelect,
  onClose,
  listRef,
}: QuickFilePickerListProps) {
  return (
    <div
      ref={listRef}
      className="max-h-80 overflow-y-auto"
    >
      {filteredFiles.length === 0 ? (
        <div className="px-4 py-8 text-center text-gray-500 dark:text-gray-400">
          {loading ? "Loading files..." : "No matching files"}
        </div>
      ) : (
        filteredFiles.map((file, index) => (
          <button
            key={file.path}
            data-index={index}
            onClick={() => {
              onSelect(file.path);
              onClose();
            }}
            className={`w-full flex items-center gap-3 px-4 py-2 text-left transition-colors ${index === selectedIndex
              ? "bg-blue-50 dark:bg-blue-900/30"
              : "hover:bg-gray-50 dark:hover:bg-gray-700/30"
              }`}
          >
            {file.type === "dir" ? (
              <Folder className="w-4 h-4 text-blue-500 flex-shrink-0" />
            ) : (
              <FileCode className="w-4 h-4 text-gray-400 flex-shrink-0" />
            )}
            <div className="flex-1 min-w-0">
              <div className="text-sm font-medium truncate">
                {highlightMatch(file.name, query)}
              </div>
              {file.path !== file.name && (
                <div className="text-xs text-gray-500 dark:text-gray-400 truncate">
                  {highlightMatch(file.path, query)}
                </div>
              )}
            </div>
            {file.language && (
              <span className="text-xs text-gray-400 dark:text-gray-500 flex-shrink-0">
                {file.language}
              </span>
            )}
          </button>
        ))
      )}
    </div>
  );
}
