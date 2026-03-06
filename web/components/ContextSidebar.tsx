import React from "react";
import { FolderKanban, Plus } from "lucide-react";
import { Context } from "@/lib/contexts";

interface ContextSidebarProps {
  contexts: Context[];
  selectedContextId: string | null;
  onSelectContext: (id: string) => void;
  showNewForm: boolean;
  setShowNewForm: (val: boolean) => void;
  newContextName: string;
  setNewContextName: (name: string) => void;
  onCreateContext: () => void;
}

export function ContextSidebar({
  contexts,
  selectedContextId,
  onSelectContext,
  showNewForm,
  setShowNewForm,
  newContextName,
  setNewContextName,
  onCreateContext,
}: ContextSidebarProps) {
  return (
    <div className="sm:w-48 md:w-56 lg:w-64 flex-shrink-0 border-b sm:border-b-0 sm:border-r border-gray-200 dark:border-gray-700 flex flex-col bg-gray-50 dark:bg-gray-900/50">
      <div className="h-[49px] px-2 sm:px-4 border-b border-gray-200 dark:border-gray-700 flex items-center justify-between">
        <h2 className="text-sm font-semibold text-gray-900 dark:text-white flex items-center gap-2">
          <FolderKanban className="w-4 h-4 text-gray-400" />
          <span>Contexts</span>
        </h2>
        {/* Mobile: show new context button in header */}
        <button
          onClick={() => setShowNewForm(true)}
          className="sm:hidden p-1 text-gray-400 hover:text-blue-600 rounded transition-colors"
          title="New context"
        >
          <Plus className="w-4 h-4" />
        </button>
      </div>

      {/* Context list - horizontal scroll on mobile */}
      <div className="flex-1 overflow-x-auto sm:overflow-x-visible overflow-y-auto p-2">
        <div className="flex sm:flex-col gap-2 sm:gap-0 min-w-max sm:min-w-0">
          {contexts.map((ctx) => (
            <button
              key={ctx.id}
              onClick={() => onSelectContext(ctx.id)}
              className={`flex-shrink-0 sm:w-full flex items-center gap-2 sm:gap-3 px-3 py-2 rounded-lg transition-colors ${selectedContextId === ctx.id
                ? "bg-blue-50 dark:bg-blue-900/30"
                : "hover:bg-gray-50 dark:hover:bg-gray-700/50"
                }`}
            >
              <div
                className="w-3 h-3 rounded-full flex-shrink-0"
                style={{ backgroundColor: ctx.color }}
              />
              <div className="text-left min-w-0">
                <div className="text-xs sm:text-sm font-medium text-gray-700 dark:text-gray-300 truncate whitespace-nowrap">
                  {ctx.name}
                </div>
                <div className="text-xs text-gray-400 whitespace-nowrap">
                  {ctx.repos.length} repos
                </div>
              </div>
            </button>
          ))}

          {/* New context button - inline on mobile */}
          {showNewForm ? (
            <div className="flex-shrink-0 sm:w-full p-2 mt-0 sm:mt-2 border border-dashed border-gray-200 dark:border-gray-700 rounded-lg min-w-[150px]">
              <input
                type="text"
                value={newContextName}
                onChange={(e) => setNewContextName(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === "Enter") onCreateContext();
                  else if (e.key === "Escape") setShowNewForm(false);
                }}
                placeholder="Context name..."
                aria-label="New context name"
                className="w-full px-2 py-1 text-sm bg-transparent border-b border-blue-500 outline-none"
              />
              <div className="flex justify-end gap-2 mt-2">
                <button
                  onClick={() => setShowNewForm(false)}
                  className="text-xs text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-300"
                >
                  Cancel
                </button>
                <button
                  onClick={onCreateContext}
                  disabled={!newContextName.trim()}
                  className="text-xs text-blue-600 hover:text-blue-700 dark:text-blue-400 dark:hover:text-blue-300 disabled:text-gray-300 dark:disabled:text-gray-600"
                >
                  Create
                </button>
              </div>
            </div>
          ) : (
            <button
              onClick={() => setShowNewForm(true)}
              className="hidden sm:flex w-full items-center justify-center gap-2 px-3 py-2 mt-2 text-sm text-gray-500 hover:text-blue-600 border border-dashed border-gray-200 dark:border-gray-700 rounded-lg hover:border-blue-300 transition-colors"
            >
              <Plus className="w-4 h-4" />
              New Context
            </button>
          )}
        </div>
      </div>
    </div>
  );
}
