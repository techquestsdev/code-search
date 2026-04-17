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
    <div className="flex flex-shrink-0 flex-col border-b border-gray-200 bg-gray-50 dark:border-gray-700 dark:bg-gray-900/50 sm:w-48 sm:border-b-0 sm:border-r md:w-56 lg:w-64">
      <div className="flex h-[49px] items-center justify-between border-b border-gray-200 px-2 dark:border-gray-700 sm:px-4">
        <h2 className="flex items-center gap-2 text-sm font-semibold text-gray-900 dark:text-white">
          <FolderKanban className="h-4 w-4 text-gray-400" />
          <span>Contexts</span>
        </h2>
        {/* Mobile: show new context button in header */}
        <button
          onClick={() => setShowNewForm(true)}
          className="rounded p-1 text-gray-400 transition-colors hover:text-blue-600 sm:hidden"
          title="New context"
        >
          <Plus className="h-4 w-4" />
        </button>
      </div>

      {/* Context list - horizontal scroll on mobile */}
      <div className="flex-1 overflow-x-auto overflow-y-auto p-2 sm:overflow-x-visible">
        <div className="flex min-w-max gap-2 sm:min-w-0 sm:flex-col sm:gap-0">
          {contexts.map((ctx) => (
            <button
              key={ctx.id}
              onClick={() => onSelectContext(ctx.id)}
              className={`flex flex-shrink-0 items-center gap-2 rounded-lg px-3 py-2 transition-colors sm:w-full sm:gap-3 ${
                selectedContextId === ctx.id
                  ? "bg-blue-50 dark:bg-blue-900/30"
                  : "hover:bg-gray-50 dark:hover:bg-gray-700/50"
              }`}
            >
              <div
                className="h-3 w-3 flex-shrink-0 rounded-full"
                style={{ backgroundColor: ctx.color }}
              />
              <div className="min-w-0 text-left">
                <div className="truncate whitespace-nowrap text-xs font-medium text-gray-700 dark:text-gray-300 sm:text-sm">
                  {ctx.name}
                </div>
                <div className="whitespace-nowrap text-xs text-gray-400">
                  {ctx.repos.length} repos
                </div>
              </div>
            </button>
          ))}

          {/* New context button - inline on mobile */}
          {showNewForm ? (
            <div className="mt-0 min-w-[150px] flex-shrink-0 rounded-lg border border-dashed border-gray-200 p-2 dark:border-gray-700 sm:mt-2 sm:w-full">
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
                className="w-full border-b border-blue-500 bg-transparent px-2 py-1 text-sm outline-none"
              />
              <div className="mt-2 flex justify-end gap-2">
                <button
                  onClick={() => setShowNewForm(false)}
                  className="text-xs text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-300"
                >
                  Cancel
                </button>
                <button
                  onClick={onCreateContext}
                  disabled={!newContextName.trim()}
                  className="text-xs text-blue-600 hover:text-blue-700 disabled:text-gray-300 dark:text-blue-400 dark:hover:text-blue-300 dark:disabled:text-gray-600"
                >
                  Create
                </button>
              </div>
            </div>
          ) : (
            <button
              onClick={() => setShowNewForm(true)}
              className="mt-2 hidden w-full items-center justify-center gap-2 rounded-lg border border-dashed border-gray-200 px-3 py-2 text-sm text-gray-500 transition-colors hover:border-blue-300 hover:text-blue-600 dark:border-gray-700 sm:flex"
            >
              <Plus className="h-4 w-4" />
              New Context
            </button>
          )}
        </div>
      </div>
    </div>
  );
}
