"use client";

import { useState, useEffect, Suspense } from "react";
import { SearchForm, SearchResults } from "@/components/Search";
import { SearchResponse, api } from "@/lib/api";
import { Search, Loader2, RefreshCw } from "lucide-react";
import { CodeSearchIcon } from "@/components/CodeSearchIcon";

function HomeContent() {
  const [results, setResults] = useState<SearchResponse | null>(null);
  const [loading, setLoading] = useState(false);
  const [repoCount, setRepoCount] = useState<number | null>(null);

  useEffect(() => {
    api.listRepos().then((response) => setRepoCount(response.total_count)).catch(() => { });
  }, []);

  return (
    <div className="h-full overflow-auto">
      <div className="container mx-auto px-4 py-6 max-w-full">
        <div className="max-w-6xl mx-auto">
          {/* Header Section */}
          <div className="flex items-center justify-between mb-4 sm:mb-6">
            {!results && !loading ? (
              // Hero state - centered
              <div className="flex-1 flex flex-col items-center pt-8">
                <div className="flex items-center gap-2 sm:gap-3">
                  <CodeSearchIcon className="w-12 h-12 sm:w-24 sm:h-24 text-blue-600 dark:text-blue-500" />
                  <h1 className="text-4xl sm:text-7xl font-bold">
                    Code Search
                  </h1>
                </div>
                <p className="text-xs sm:text-sm text-gray-500 dark:text-gray-400 mt-1 text-center">
                  Search across {repoCount !== null ? (
                    <span className="text-blue-600 dark:text-blue-400 font-medium">{repoCount} repositories</span>
                  ) : (
                    "your repositories"
                  )} with Zoekt
                </p>
              </div>
            ) : (
              // Results state - left aligned (consistent with other pages)
              <>
                <div className="flex items-center gap-2 sm:gap-3">
                  <Search className="w-5 h-5 sm:w-6 sm:h-6 text-gray-600 dark:text-gray-400" />
                  <h1 className="text-xl sm:text-2xl font-bold">Search Results</h1>
                </div>
                <div
                  className="flex items-center gap-2 px-2.5 sm:px-3 py-1.5 sm:py-2 text-sm font-medium rounded-lg opacity-0 pointer-events-none select-none"
                  aria-hidden="true"
                >
                  <RefreshCw className="w-4 h-4" />
                  <span className="hidden sm:inline">Easter Egg! Ths is only meant to balance the layout. c:</span>
                </div>
              </>
            )}
          </div>

          <SearchForm onResults={setResults} onLoading={setLoading} />

          {/* Search Help - show when no results and not loading */}
          {!loading && !results && (
            <div className="mt-6 sm:mt-10">
              <h2 className="text-center text-base sm:text-lg font-medium text-gray-700 dark:text-gray-300 mb-4 sm:mb-6">
                How to search
              </h2>

              <div className="grid grid-cols-2 sm:grid-cols-2 gap-6 sm:gap-x-16 sm:gap-y-8 max-w-2xl mx-auto">
                {/* Left Column - Search in files */}
                <div>
                  <h3 className="text-sm font-medium text-blue-600 dark:text-blue-400 mb-3 sm:mb-4">
                    Search patterns
                  </h3>
                  <div className="space-y-2 sm:space-y-3 text-sm">
                    <SearchExample query='"foo bar"' description="exact match" />
                    <SearchExample query="foo bar" description="both foo and bar" />
                    <SearchExample query='foo or bar' description="either foo or bar" />
                    <SearchExample
                      query={<>FOO <span className="text-orange-500">case:</span>yes</>}
                      description="case sensitive"
                    />
                  </div>
                </div>

                {/* Right Column - Filter results */}
                <div>
                  <h3 className="text-sm font-medium text-blue-600 dark:text-blue-400 mb-3 sm:mb-4">
                    Filter results
                  </h3>
                  <div className="space-y-2 sm:space-y-3 text-sm">
                    <SearchExample
                      query={<><span className="text-cyan-500">lang:</span>go</>}
                      description="by language"
                    />
                    <SearchExample
                      query={<><span className="text-cyan-500">file:</span>README</>}
                      description="by filename"
                    />
                    <SearchExample
                      query={<><span className="text-cyan-500">repo:</span>org/repo</>}
                      description="by repository"
                    />
                    <SearchExample
                      query={<><span className="text-cyan-500">content:</span>foo</>}
                      description="search content only"
                    />
                  </div>
                </div>

                {/* Advanced section */}
                <div className="col-span-2">
                  <h3 className="text-sm font-medium text-amber-600 dark:text-amber-400 mb-3 sm:mb-4">
                    Advanced
                  </h3>
                  <div className="grid grid-cols-2 gap-2 sm:gap-x-16 sm:gap-y-3 text-sm">
                    <SearchExample
                      query={<>foo <span className="text-red-500">-lang:</span>go</>}
                      description="negate filter"
                    />
                    <SearchExample
                      query={<><span className="text-cyan-500">sym:</span>GetFoo</>}
                      description="symbol search"
                    />
                    <SearchExample
                      query={<><span className="text-cyan-500">file:</span>\.ts$</>}
                      description='files that end in ".ts"'
                    />
                    <SearchExample
                      query={<><span className="text-purple-500">/foo-(bar|baz)/</span></>}
                      description="regular expression"
                    />
                  </div>
                </div>
              </div>

              {/* Full documentation link */}
              <div className="mt-6 sm:mt-8 pt-4 sm:pt-6 border-t border-gray-200 dark:border-gray-700 text-center">
                <a
                  href="https://code-search.techquests.dev/web-ui/query-syntax/"
                  target="_blank"
                  rel="noopener noreferrer"
                  className="text-xs text-gray-500 dark:text-gray-400 hover:text-blue-600 dark:hover:text-blue-400 transition-colors"
                >
                  View full query syntax documentation →
                </a>
              </div>
            </div>
          )}

          {/* Show loading indicator only when no results yet */}
          {loading && !results && (
            <div className="text-center py-12">
              <Loader2 className="w-8 h-8 animate-spin text-blue-600 mx-auto" />
              <p className="mt-3 text-gray-500 dark:text-gray-400">Searching...</p>
            </div>
          )}

          {/* Always show results if available (supports streaming) */}
          <SearchResults response={results} isStreaming={loading && !!results} />
        </div>
      </div>
    </div>
  );
}

export default function HomeClient() {
  return (
    <Suspense fallback={
      <div className="max-w-6xl mx-auto flex items-center justify-center py-20">
        <Loader2 className="w-8 h-8 animate-spin text-blue-600" />
      </div>
    }>
      <HomeContent />
    </Suspense>
  );
}

function SearchExample({
  query,
  description
}: {
  query: React.ReactNode;
  description: string;
}) {
  return (
    <div className="flex flex-col sm:flex-row sm:items-baseline gap-1 sm:gap-3">
      <code className="font-mono text-gray-800 dark:text-gray-100 text-xs sm:text-sm">{query}</code>
      <span className="text-gray-500 dark:text-gray-500 text-xs sm:text-sm">({description})</span>
    </div>
  );
}
