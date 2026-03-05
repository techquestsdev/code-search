"use client";

import React, { useState, useRef, useEffect } from "react";
import { useContexts } from "@/hooks/useContexts";
import { Context } from "@/lib/contexts";
import { ContextManager } from "./ContextManager";
import {
  FolderKanban,
  ChevronDown,
  Plus,
  Settings,
  Check,
  X,
  Trash2,
  Edit2,
  Globe,
} from "lucide-react";

interface ContextSwitcherProps {
  className?: string;
  onContextChange?: (context: Context | null) => void;
  onOpenManager?: () => void;
  labelPrefix?: string; // e.g., "Searching in" or "Replacing in"
}

interface ContextSwitcherListProps {
  contexts: Context[];
  activeContext: Context | null;
  editingId: string | null;
  editName: string;
  onSelect: (id: string | null) => void;
  onDelete: (e: React.MouseEvent, id: string) => void;
  onEdit: (id: string, name: string) => void;
  onEditNameChange: (name: string) => void;
  onUpdateName: (id: string, name: string) => void;
  onCancelEdit: () => void;
}

function ContextSwitcherList({
  contexts,
  activeContext,
  editingId,
  editName,
  onSelect,
  onDelete,
  onEdit,
  onEditNameChange,
  onUpdateName,
  onCancelEdit,
}: ContextSwitcherListProps) {
  return (
    <div className="max-h-64 overflow-y-auto">
      <button
        onClick={() => onSelect(null)}
        className={`w-full flex items-center gap-3 px-3 py-2 hover:bg-gray-50 dark:hover:bg-gray-700/50 transition-colors ${!activeContext ? "bg-blue-50 dark:bg-blue-900/20" : ""}`}
      >
        <Globe className="w-4 h-4 text-gray-400" />
        <span className="flex-1 text-sm text-left text-gray-700 dark:text-gray-300">All Repositories</span>
        {!activeContext && <Check className="w-4 h-4 text-blue-600" />}
      </button>

      {contexts.length > 0 && <div className="border-t border-gray-100 dark:border-gray-700 my-1" />}

      {contexts.map((ctx) => (
        <div
          key={ctx.id}
          className={`group flex items-center gap-3 px-3 py-2 hover:bg-gray-50 dark:hover:bg-gray-700/50 transition-colors ${activeContext?.id === ctx.id ? "bg-blue-50 dark:bg-blue-900/20" : ""}`}
        >
          <button
            type="button"
            className="flex-1 flex items-center gap-3 text-left outline-none"
            onClick={() => onSelect(ctx.id)}
            onKeyDown={(e) => { if (e.key === 'Enter' || e.key === ' ') onSelect(ctx.id); }}
          >
            <div className="w-3 h-3 rounded-full flex-shrink-0" style={{ backgroundColor: ctx.color }} />
            {editingId === ctx.id ? (
              <input
                type="text"
                value={editName}
                onChange={(e) => onEditNameChange(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === "Enter" && editName.trim()) onUpdateName(ctx.id, editName.trim());
                  else if (e.key === "Escape") onCancelEdit();
                }}
                onBlur={() => { if (editName.trim() && editName.trim() !== ctx.name) onUpdateName(ctx.id, editName.trim()); else onCancelEdit(); }}
                onClick={(e) => e.stopPropagation()}
                className="flex-1 text-sm bg-transparent border-b border-blue-500 outline-none"
              />
            ) : (
              <div className="flex-1 min-w-0">
                <span className="text-sm text-gray-700 dark:text-gray-300 truncate block">{ctx.name}</span>
                <span className="text-xs text-gray-400">{ctx.repos.length} repo{ctx.repos.length !== 1 ? "s" : ""}</span>
              </div>
            )}
            {activeContext?.id === ctx.id && editingId !== ctx.id && <Check className="w-4 h-4 text-blue-600 flex-shrink-0" />}
          </button>
          
          <div className="hidden group-hover:flex items-center gap-1">
            <button onClick={(e) => { e.stopPropagation(); onEdit(ctx.id, ctx.name); }} className="p-1 text-gray-400 hover:text-gray-600 rounded" title="Edit"><Edit2 className="w-3 h-3" /></button>
            <button onClick={(e) => onDelete(e, ctx.id)} className="p-1 text-gray-400 hover:text-red-600 rounded" title="Delete"><Trash2 className="w-3 h-3" /></button>
          </div>
        </div>
      ))}
      {contexts.length === 0 && <div className="px-3 py-4 text-center text-sm text-gray-500 dark:text-gray-400">No contexts yet.</div>}
    </div>
  );
}

interface NewContextFormProps {
  newName: string;
  setNewName: (name: string) => void;
  onCreate: () => void;
  onCancel: () => void;
  inputRef: React.RefObject<HTMLInputElement | null>;
}

function NewContextForm({ newName, setNewName, onCreate, onCancel, inputRef }: NewContextFormProps) {
  return (
    <div className="border-t border-gray-100 dark:border-gray-700 p-3">
      <div className="flex items-center gap-2">
        <FolderKanban className="w-4 h-4 text-gray-400" />
        <input
          ref={inputRef}
          type="text"
          value={newName}
          onChange={(e) => setNewName(e.target.value)}
          onKeyDown={(e) => { if (e.key === "Enter") onCreate(); else if (e.key === "Escape") onCancel(); }}
          placeholder="Context name..."
          aria-label="New context name"
          className="flex-1 text-sm bg-transparent outline-none placeholder-gray-400"
        />
        <button onClick={onCreate} disabled={!newName.trim()} className="p-1 text-green-600 hover:text-green-700 disabled:text-gray-300 rounded"><Check className="w-4 h-4" /></button>
        <button onClick={onCancel} className="p-1 text-gray-400 hover:text-gray-600 rounded"><X className="w-4 h-4" /></button>
      </div>
    </div>
  );
}

export function ContextSwitcher({ className = "", onContextChange, onOpenManager, labelPrefix = "Searching in" }: ContextSwitcherProps) {
  const {
    contexts,
    activeContext,
    setActiveContext,
    createContext,
    updateContext,
    deleteContext,
    isLoading,
  } = useContexts();

  const [state, setState] = useState({
    isOpen: false,
    showNewForm: false,
    newName: "",
    editingId: null as string | null,
    editName: "",
    showManager: false,
  });

  const dropdownRef = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLInputElement>(null);

  // Close dropdown on outside click
  useEffect(() => {
    const handleClickOutside = (e: MouseEvent) => {
      if (dropdownRef.current && !dropdownRef.current.contains(e.target as Node)) {
        setState(prev => ({ ...prev, isOpen: false, showNewForm: false, editingId: null }));
      }
    };
    document.addEventListener("mousedown", handleClickOutside);
    return () => document.removeEventListener("mousedown", handleClickOutside);
  }, []);

  // Focus input when showing new form
  useEffect(() => {
    if (state.showNewForm && inputRef.current) {
      inputRef.current.focus();
    }
  }, [state.showNewForm]);

  const handleSelectContext = (contextId: string | null) => {
    setActiveContext(contextId);
    setState(prev => ({ ...prev, isOpen: false }));

    const ctx = contextId ? contexts.find(c => c.id === contextId) || null : null;
    onContextChange?.(ctx);
  };

  const handleCreateContext = () => {
    if (!state.newName.trim()) return;
    const ctx = createContext(state.newName.trim());
    setState(prev => ({ ...prev, newName: "", showNewForm: false }));
    setActiveContext(ctx.id);
    onContextChange?.(ctx);
  };

  const handleDeleteContext = (e: React.MouseEvent, id: string) => {
    e.stopPropagation();
    if (confirm("Delete this context? Repos won't be affected.")) {
      deleteContext(id);
      if (activeContext?.id === id) {
        onContextChange?.(null);
      }
    }
  };

  if (isLoading) {
    return (
      <div className={`h-8 w-32 bg-gray-100 dark:bg-gray-700 rounded animate-pulse ${className}`} />
    );
  }

  return (
    <div className={`relative ${className}`} ref={dropdownRef}>
      {/* Trigger button - styled badge when context active */}
      <button
        type="button"
        onClick={() => setState(prev => ({ ...prev, isOpen: !prev.isOpen }))}
        className={`flex items-center gap-2 px-3 py-1.5 transition-all ${activeContext
          ? "rounded-full text-sm hover:opacity-80"
          : "rounded-lg border border-gray-200 dark:border-gray-700 hover:bg-gray-50 dark:hover:bg-gray-800"
          }`}
        style={activeContext ? {
          backgroundColor: `${activeContext.color}15`,
          color: activeContext.color,
          borderColor: `${activeContext.color}40`,
          borderWidth: 1
        } : undefined}
      >
        {activeContext ? (
          <>
            <div
              className="w-2 h-2 rounded-full"
              style={{ backgroundColor: activeContext.color }}
            />
            <span className="font-medium">{labelPrefix} {activeContext.name}</span>
            <span className="text-xs opacity-70">
              ({activeContext.repos.length} repos{activeContext.repoFilter ? ` · ${activeContext.isRegexFilter ? 'regex' : 'filter'}: ${activeContext.repoFilter}` : ''})
            </span>
            <ChevronDown className={`w-3 h-3 opacity-70 transition-transform ${state.isOpen ? "rotate-180" : ""}`} />
          </>
        ) : (
          <>
            <Globe className="w-4 h-4 text-gray-500" />
            <span className="text-sm text-gray-600 dark:text-gray-400">All Repos</span>
            <ChevronDown className={`w-4 h-4 transition-transform ${state.isOpen ? "rotate-180" : ""}`} />
          </>
        )}
      </button>

      {/* Dropdown */}
      {state.isOpen && (
        <div className="absolute top-full left-0 mt-1 w-64 bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-xl shadow-xl z-50 overflow-hidden">
          {/* Header */}
          <div className="px-3 py-2 border-b border-gray-100 dark:border-gray-700 flex items-center justify-between">
            <span className="text-xs font-semibold text-gray-500 dark:text-gray-400 uppercase tracking-wider">
              Contexts
            </span>
            <button
              onClick={() => setState(prev => ({ ...prev, showNewForm: true }))}
              className="p-1 text-gray-400 hover:text-blue-600 dark:hover:text-blue-400 rounded transition-colors"
              title="Create new context"
            >
              <Plus className="w-4 h-4" />
            </button>
          </div>

          {/* Context list */}
          <ContextSwitcherList
            contexts={contexts}
            activeContext={activeContext}
            editingId={state.editingId}
            editName={state.editName}
            onSelect={handleSelectContext}
            onDelete={handleDeleteContext}
            onEdit={(id, name) => setState(prev => ({ ...prev, editingId: id, editName: name }))}
            onEditNameChange={name => setState(prev => ({ ...prev, editName: name }))}
            onUpdateName={(id, name) => { updateContext(id, { name }); setState(prev => ({ ...prev, editingId: null })); }}
            onCancelEdit={() => setState(prev => ({ ...prev, editingId: null }))}
          />

          {/* New context form */}
          {state.showNewForm && (
            <NewContextForm
              newName={state.newName}
              setNewName={name => setState(prev => ({ ...prev, newName: name }))}
              onCreate={handleCreateContext}
              onCancel={() => setState(prev => ({ ...prev, showNewForm: false, newName: "" }))}
              inputRef={inputRef}
            />
          )}

          {/* Footer */}
          <div className="border-t border-gray-100 dark:border-gray-700 px-3 py-2">
            <button
              onClick={() => {
                setState(prev => ({ ...prev, isOpen: false }));
                if (onOpenManager) {
                  onOpenManager();
                } else {
                  setState(prev => ({ ...prev, showManager: true }));
                }
              }}
              className="flex items-center gap-2 text-xs text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-300 transition-colors"
            >
              <Settings className="w-3 h-3" />
              Manage Contexts
            </button>
          </div>
        </div>
      )}

      {/* Context Manager Modal - only render if no external manager */}
      {!onOpenManager && (
        <ContextManager
          isOpen={state.showManager}
          onClose={() => setState(prev => ({ ...prev, showManager: false }))}
          initialContextId={activeContext?.id}
        />
      )}
    </div>
  );
}

export default ContextSwitcher;
