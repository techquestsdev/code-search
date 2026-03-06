"use client";

import React, { useState, useEffect, useRef, useCallback, useMemo } from "react";
import { useContexts } from "@/hooks/useContexts";
import { api, Repository } from "@/lib/api";
import { ContextSidebar } from "./ContextSidebar";
import { RepoSelector } from "./RepoSelector";

interface ContextManagerProps {
  isOpen: boolean;
  onClose: () => void;
  initialContextId?: string;
}

export function ContextManager({ isOpen, onClose, initialContextId }: ContextManagerProps) {
  const {
    contexts,
    activeContext,
    createContext,
    updateContext,
    deleteContext,
    addRepo,
    removeRepo,
    setActiveContext,
    getNextColor,
  } = useContexts();

  const [managerState, setManagerState] = useState({
    selectedContextId: initialContextId || activeContext?.id || null as string | null,
    allRepos: [] as Repository[],
    allReposLoaded: false,
    loadingRepos: true,
    searchQuery: "",
    isRegexSearch: false,
    newContextName: "",
    showNewForm: false,
    editingName: false,
    tempName: "",
  });

  const {
    selectedContextId,
    allRepos,
    allReposLoaded,
    loadingRepos,
    searchQuery,
    isRegexSearch,
    newContextName,
    showNewForm,
    editingName,
    tempName,
  } = managerState;

  // Track if filter has been loaded from context (prevents saving before load)
  const filterInitialized = useRef(false);
  const wasOpenRef = useRef(false);
  const savePendingRef = useRef(false);

  const selectedContext = contexts.find((c) => c.id === selectedContextId) || null;

  // Update selected context only when modal OPENS
  useEffect(() => {
    if (isOpen && !wasOpenRef.current) {
      filterInitialized.current = false;
      const contextToSelect = initialContextId || activeContext?.id || contexts[0]?.id || null;
      const ctx = contexts.find(c => c.id === contextToSelect);

      setManagerState(prev => ({
        ...prev,
        selectedContextId: contextToSelect,
        allReposLoaded: false,
        searchQuery: ctx?.repoFilter || "",
        isRegexSearch: ctx?.isRegexFilter || false,
      }));
      filterInitialized.current = true;
    }
    wasOpenRef.current = isOpen;
  }, [isOpen, initialContextId, activeContext?.id, contexts]);

  // Load all repos progressively
  useEffect(() => {
    if (!isOpen || allReposLoaded) return;

    const loadAllRepos = async () => {
      try {
        let allLoaded: Repository[] = [];
        let offset = 0;
        const batchSize = 500;
        let hasMoreToLoad = true;

        while (hasMoreToLoad) {
          const data = await api.listRepos({ limit: batchSize, offset });
          allLoaded = [...allLoaded, ...data.repos];
          offset += batchSize;
          hasMoreToLoad = data.has_more;
          setManagerState(prev => ({
            ...prev,
            allRepos: allLoaded,
            loadingRepos: hasMoreToLoad,
            allReposLoaded: !hasMoreToLoad,
          }));
        }
      } catch (err) {
        console.error("Failed to load repos:", err);
        setManagerState(prev => ({ ...prev, loadingRepos: false }));
      }
    };

    loadAllRepos();
  }, [isOpen, allReposLoaded]);

  // Debounced filter saving
  useEffect(() => {
    if (!filterInitialized.current || !selectedContextId || !selectedContext) return;

    const timer = setTimeout(() => {
      if (selectedContext.repoFilter !== searchQuery || selectedContext.isRegexFilter !== isRegexSearch) {
        updateContext(selectedContextId, { repoFilter: searchQuery, isRegexFilter: isRegexSearch });
      }
    }, 500);

    return () => clearTimeout(timer);
  }, [selectedContextId, searchQuery, isRegexSearch, updateContext, selectedContext]);

  const handleSelectContext = useCallback((id: string) => {
    filterInitialized.current = false;
    const ctx = contexts.find(c => c.id === id);
    setManagerState(prev => ({
      ...prev,
      selectedContextId: id,
      searchQuery: ctx?.repoFilter || "",
      isRegexSearch: ctx?.isRegexFilter || false,
    }));
    filterInitialized.current = true;
  }, [contexts]);

  const filteredRepos = useMemo(() => {
    if (!searchQuery) return allRepos;
    if (isRegexSearch) {
      try {
        const regex = new RegExp(searchQuery, "i");
        return allRepos.filter(r => regex.test(r.name));
      } catch { return allRepos; }
    }
    const q = searchQuery.toLowerCase();
    return allRepos.filter(r => r.name.toLowerCase().includes(q));
  }, [allRepos, searchQuery, isRegexSearch]);

  const matchInfo = useMemo(() => {
    if (!searchQuery) return null;
    if (isRegexSearch) {
      try {
        new RegExp(searchQuery, "i");
        return { valid: true, matchCount: filteredRepos.length, totalCount: allRepos.length };
      } catch { return { valid: false, matchCount: 0, totalCount: allRepos.length }; }
    }
    return { valid: true, matchCount: filteredRepos.length, totalCount: allRepos.length };
  }, [isRegexSearch, searchQuery, filteredRepos.length, allRepos.length]);

  const contextRepoIds = useMemo(() => new Set(selectedContext?.repos.map(r => r.id) || []), [selectedContext?.repos]);
  const isRepoInContext = useCallback((id: number) => contextRepoIds.has(id), [contextRepoIds]);

  const handleToggleRepo = useCallback((repo: Repository) => {
    if (!selectedContextId) return;
    if (contextRepoIds.has(repo.id)) removeRepo(selectedContextId, repo.id);
    else addRepo(selectedContextId, repo.id, repo.name, searchQuery || undefined, isRegexSearch);
  }, [selectedContextId, contextRepoIds, addRepo, removeRepo, searchQuery, isRegexSearch]);

  const handleAddAllFiltered = useCallback(() => {
    if (!selectedContextId) return;
    filteredRepos.forEach(repo => {
      if (!contextRepoIds.has(repo.id)) addRepo(selectedContextId, repo.id, repo.name, searchQuery || undefined, isRegexSearch);
    });
  }, [selectedContextId, filteredRepos, contextRepoIds, addRepo, searchQuery, isRegexSearch]);

  const handleRemoveAllFiltered = useCallback(() => {
    if (!selectedContextId) return;
    filteredRepos.forEach(repo => {
      if (contextRepoIds.has(repo.id)) removeRepo(selectedContextId, repo.id);
    });
  }, [selectedContextId, filteredRepos, contextRepoIds, removeRepo]);

  const filteredInContextCount = filteredRepos.filter(r => isRepoInContext(r.id)).length;
  const filteredNotInContextCount = filteredRepos.length - filteredInContextCount;

  const handleCreateContext = () => {
    if (!newContextName.trim()) return;
    const ctx = createContext(newContextName.trim(), undefined, getNextColor());
    setManagerState(prev => ({ ...prev, selectedContextId: ctx.id, newContextName: "", showNewForm: false, searchQuery: "", isRegexSearch: false }));
  };

  const handleSaveName = (name: string) => {
    if (savePendingRef.current || !selectedContextId || !name.trim()) return;
    savePendingRef.current = true;
    updateContext(selectedContextId, { name: name.trim() });
    setManagerState(prev => ({ ...prev, editingName: false }));
    setTimeout(() => { savePendingRef.current = false; }, 200);
  };

  const handleDeleteContext = () => {
    if (!selectedContextId || !confirm("Delete this context? Repos won't be affected.")) return;
    deleteContext(selectedContextId);
    setManagerState(prev => ({ ...prev, selectedContextId: contexts[0]?.id || null }));
  };

  return (
    <div role="dialog" aria-modal="true" className={`fixed inset-0 z-50 flex items-center justify-center pt-4 transition-opacity ${isOpen ? "opacity-100" : "opacity-0 pointer-events-none"}`}>
      <button 
        className="absolute inset-0 bg-black/50 backdrop-blur-sm cursor-default w-full h-full border-0" 
        onClick={onClose} 
        onKeyDown={e => { if (e.key === 'Escape') onClose(); }}
        aria-label="Close modal" 
      />
      <div 
        className="relative w-full max-w-5xl h-[85vh] sm:h-[70vh] bg-white dark:bg-gray-800 rounded-xl shadow-2xl border border-gray-200 dark:border-gray-700 flex flex-col sm:flex-row overflow-hidden m-4" 
        onClick={e => e.stopPropagation()}
        onKeyDown={e => e.stopPropagation()}
        role="presentation"
      >
        <ContextSidebar
          contexts={contexts}
          selectedContextId={selectedContextId}
          onSelectContext={handleSelectContext}
          showNewForm={showNewForm}
          setShowNewForm={val => setManagerState(prev => ({ ...prev, showNewForm: val }))}
          newContextName={newContextName}
          setNewContextName={val => setManagerState(prev => ({ ...prev, newContextName: val }))}
          onCreateContext={handleCreateContext}
        />
        <RepoSelector
          selectedContext={selectedContext}
          searchQuery={searchQuery}
          setSearchQuery={val => setManagerState(prev => ({ ...prev, searchQuery: val }))}
          isRegexSearch={isRegexSearch}
          setIsRegexSearch={val => setManagerState(prev => ({ ...prev, isRegexSearch: val }))}
          allRepos={allRepos}
          allReposLoaded={allReposLoaded}
          loadingRepos={loadingRepos}
          filteredRepos={filteredRepos}
          matchInfo={matchInfo}
          refreshRepos={() => setManagerState(prev => ({ ...prev, allReposLoaded: false, allRepos: [] }))}
          isRepoInContext={isRepoInContext}
          handleToggleRepo={handleToggleRepo}
          handleAddAllFiltered={handleAddAllFiltered}
          handleRemoveAllFiltered={handleRemoveAllFiltered}
          filteredInContextCount={filteredInContextCount}
          filteredNotInContextCount={filteredNotInContextCount}
          editingName={editingName}
          tempName={tempName}
          setTempName={val => setManagerState(prev => ({ ...prev, tempName: val }))}
          setEditingName={val => setManagerState(prev => ({ ...prev, editingName: val }))}
          handleSaveName={handleSaveName}
          updateContextColor={color => selectedContextId && updateContext(selectedContextId, { color })}
          handleDeleteContext={handleDeleteContext}
          onUseContext={() => { setActiveContext(selectedContextId); onClose(); }}
          onClose={onClose}
        />
      </div>
    </div>
  );
}

export default ContextManager;
