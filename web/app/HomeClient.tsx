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
    api
      .listRepos()
      .then((response) => setRepoCount(response.total_count))
      .catch(() => {});
  }, []);

  return (
    <div className="h-full overflow-auto">
      <div className="container mx-auto max-w-full px-4 py-6">
        <div className="mx-auto max-w-6xl">
          {/* Header Section */}
          <div className="mb-4 flex items-center justify-between sm:mb-6">
            {!results && !loading ? (
              // Hero state - centered
              <div className="flex flex-1 flex-col items-center pt-8">
                <div className="flex items-center gap-2 sm:gap-3">
                  <CodeSearchIcon className="h-12 w-12 text-blue-600 dark:text-blue-500 sm:h-24 sm:w-24" />
                  <h1 className="text-4xl font-bold sm:text-7xl">
                    Code Search
                  </h1>
                </div>
                <p className="mt-1 text-center text-xs text-gray-500 dark:text-gray-400 sm:text-sm">
                  Search across{" "}
                  {repoCount !== null ? (
                    <span className="font-medium text-blue-600 dark:text-blue-400">
                      {repoCount} repositories
                    </span>
                  ) : (
                    "your repositories"
                  )}{" "}
                  with Zoekt
                </p>
              </div>
            ) : (
              // Results state - left aligned (consistent with other pages)
              <>
                <div className="flex items-center gap-2 sm:gap-3">
                  <Search className="h-5 w-5 text-gray-600 dark:text-gray-400 sm:h-6 sm:w-6" />
                  <h1 className="text-xl font-bold sm:text-2xl">
                    Search Results
                  </h1>
                </div>
                <div
                  className="pointer-events-none flex select-none items-center gap-2 rounded-lg px-2.5 py-1.5 text-sm font-medium opacity-0 sm:px-3 sm:py-2"
                  aria-hidden="true"
                >
                  <RefreshCw className="h-4 w-4" />
                  <span className="hidden sm:inline">
                    Easter Egg! Ths is only meant to balance the layout. c:
                  </span>
                </div>
              </>
            )}
          </div>

          <SearchForm onResults={setResults} onLoading={setLoading} />

          {/* Search Help - show when no results and not loading */}
          {!loading && !results && (
            <div className="mt-6 sm:mt-10">
              <h2 className="mb-4 text-center text-base font-medium text-gray-700 dark:text-gray-300 sm:mb-6 sm:text-lg">
                How to search
              </h2>

              <div className="mx-auto grid max-w-2xl grid-cols-2 gap-6 sm:grid-cols-2 sm:gap-x-16 sm:gap-y-8">
                {/* Left Column - Search in files */}
                <div>
                  <h3 className="mb-3 text-sm font-medium text-blue-600 dark:text-blue-400 sm:mb-4">
                    Search patterns
                  </h3>
                  <div className="space-y-2 text-sm sm:space-y-3">
                    <SearchExample
                      query='"foo bar"'
                      description="exact match"
                    />
                    <SearchExample
                      query="foo bar"
                      description="both foo and bar"
                    />
                    <SearchExample
                      query="foo or bar"
                      description="either foo or bar"
                    />
                    <SearchExample
                      query={
                        <>
                          FOO <span className="text-orange-500">case:</span>yes
                        </>
                      }
                      description="case sensitive"
                    />
                  </div>
                </div>

                {/* Right Column - Filter results */}
                <div>
                  <h3 className="mb-3 text-sm font-medium text-blue-600 dark:text-blue-400 sm:mb-4">
                    Filter results
                  </h3>
                  <div className="space-y-2 text-sm sm:space-y-3">
                    <SearchExample
                      query={
                        <>
                          <span className="text-cyan-500">lang:</span>go
                        </>
                      }
                      description="by language"
                    />
                    <SearchExample
                      query={
                        <>
                          <span className="text-cyan-500">file:</span>README
                        </>
                      }
                      description="by filename"
                    />
                    <SearchExample
                      query={
                        <>
                          <span className="text-cyan-500">repo:</span>org/repo
                        </>
                      }
                      description="by repository"
                    />
                    <SearchExample
                      query={
                        <>
                          <span className="text-cyan-500">content:</span>foo
                        </>
                      }
                      description="search content only"
                    />
                  </div>
                </div>

                {/* Advanced section */}
                <div className="col-span-2">
                  <h3 className="mb-3 text-sm font-medium text-amber-600 dark:text-amber-400 sm:mb-4">
                    Advanced
                  </h3>
                  <div className="grid grid-cols-2 gap-2 text-sm sm:gap-x-16 sm:gap-y-3">
                    <SearchExample
                      query={
                        <>
                          foo <span className="text-red-500">-lang:</span>go
                        </>
                      }
                      description="negate filter"
                    />
                    <SearchExample
                      query={
                        <>
                          <span className="text-cyan-500">sym:</span>GetFoo
                        </>
                      }
                      description="symbol search"
                    />
                    <SearchExample
                      query={
                        <>
                          <span className="text-cyan-500">file:</span>\.ts$
                        </>
                      }
                      description='files that end in ".ts"'
                    />
                    <SearchExample
                      query={
                        <>
                          <span className="text-purple-500">
                            /foo-(bar|baz)/
                          </span>
                        </>
                      }
                      description="regular expression"
                    />
                  </div>
                </div>
              </div>

              {/* Full documentation link */}
              <div className="mt-6 border-t border-gray-200 pt-4 text-center dark:border-gray-700 sm:mt-8 sm:pt-6">
                <a
                  href="https://code-search.techquests.dev/web-ui/query-syntax/"
                  target="_blank"
                  rel="noopener noreferrer"
                  className="text-xs text-gray-500 transition-colors hover:text-blue-600 dark:text-gray-400 dark:hover:text-blue-400"
                >
                  View full query syntax documentation →
                </a>
              </div>
            </div>
          )}

          {/* Show loading indicator only when no results yet */}
          {loading && !results && (
            <div className="py-12 text-center">
              <Loader2 className="mx-auto h-8 w-8 animate-spin text-blue-600" />
              <p className="mt-3 text-gray-500 dark:text-gray-400">
                Searching...
              </p>
            </div>
          )}

          {/* Always show results if available (supports streaming) */}
          <SearchResults
            response={results}
            isStreaming={loading && !!results}
          />
        </div>
      </div>
    </div>
  );
}

export default function HomeClient() {
  return (
    <Suspense
      fallback={
        <div className="mx-auto flex max-w-6xl items-center justify-center py-20">
          <Loader2 className="h-8 w-8 animate-spin text-blue-600" />
        </div>
      }
    >
      <HomeContent />
    </Suspense>
  );
}

function SearchExample({
  query,
  description,
}: {
  query: React.ReactNode;
  description: string;
}) {
  return (
    <div className="flex flex-col gap-1 sm:flex-row sm:items-baseline sm:gap-3">
      <code className="font-mono text-xs text-gray-800 dark:text-gray-100 sm:text-sm">
        {query}
      </code>
      <span className="text-xs text-gray-500 dark:text-gray-500 sm:text-sm">
        ({description})
      </span>
    </div>
  );
}
