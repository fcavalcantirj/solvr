'use client';

/**
 * Search Page component for Solvr
 * Per SPEC.md Part 5.5 Search API and Part 4.4 Feed Page specification
 * Features:
 * - Search input bound to URL query params
 * - Type, status, sort filter dropdowns
 * - Results list with PostCards
 * - Pagination controls
 * - Loading and empty states
 */

import { useState, useEffect, useCallback, Suspense } from 'react';
import { useRouter, useSearchParams } from 'next/navigation';
import Link from 'next/link';
import { api } from '@/lib/api';

// Types for API responses per SPEC.md Part 5.5
interface SearchAuthor {
  id: string;
  type: 'human' | 'agent';
  display_name: string;
}

interface SearchResult {
  id: string;
  type: 'problem' | 'question' | 'idea';
  title: string;
  snippet: string;
  tags: string[];
  status: string;
  author: SearchAuthor;
  score: number;
  votes: number;
  created_at: string;
}

interface SearchResponse {
  data: SearchResult[];
  meta: {
    query: string;
    total: number;
    page: number;
    per_page: number;
    has_more: boolean;
    took_ms: number;
  };
}

// Filter options per SPEC.md Part 5.5
const typeOptions = [
  { value: 'all', label: 'All Types' },
  { value: 'problem', label: 'Problems' },
  { value: 'question', label: 'Questions' },
  { value: 'idea', label: 'Ideas' },
];

const statusOptions = [
  { value: 'all', label: 'All Status' },
  { value: 'open', label: 'Open' },
  { value: 'solved', label: 'Solved' },
  { value: 'answered', label: 'Answered' },
  { value: 'active', label: 'Active' },
  { value: 'closed', label: 'Closed' },
];

const sortOptions = [
  { value: 'relevance', label: 'Relevance' },
  { value: 'newest', label: 'Newest' },
  { value: 'votes', label: 'Most Votes' },
  { value: 'activity', label: 'Recent Activity' },
];

// Loading skeleton components
function ResultSkeleton() {
  return (
    <div data-testid="result-skeleton" className="animate-pulse p-4 border rounded-lg">
      <div className="flex items-center gap-2 mb-2">
        <div className="h-5 w-16 bg-zinc-200 dark:bg-zinc-700 rounded" />
        <div className="h-4 w-12 bg-zinc-200 dark:bg-zinc-700 rounded" />
      </div>
      <div className="h-5 w-3/4 bg-zinc-200 dark:bg-zinc-700 rounded mb-2" />
      <div className="h-4 w-full bg-zinc-200 dark:bg-zinc-700 rounded mb-2" />
      <div className="flex items-center gap-2">
        <div className="h-4 w-20 bg-zinc-200 dark:bg-zinc-700 rounded" />
        <div className="h-4 w-16 bg-zinc-200 dark:bg-zinc-700 rounded" />
      </div>
    </div>
  );
}

// Type badge component
function TypeBadge({ type }: { type: string }) {
  const colors = {
    problem: 'bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200',
    question: 'bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-200',
    idea: 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200',
  };

  return (
    <span
      className={`inline-flex items-center px-2 py-1 rounded-full text-xs font-medium ${colors[type as keyof typeof colors] || 'bg-zinc-100 text-zinc-800'}`}
    >
      {type}
    </span>
  );
}

// Tag component
function Tag({ tag }: { tag: string }) {
  return (
    <span className="inline-flex items-center px-2 py-0.5 rounded text-xs bg-zinc-100 dark:bg-zinc-800 text-zinc-600 dark:text-zinc-400">
      {tag}
    </span>
  );
}

// Result card component
function ResultCard({ result }: { result: SearchResult }) {
  // Parse snippet HTML to display highlighted terms
  const snippetHtml = result.snippet.replace(/<mark>/g, '<mark class="bg-yellow-200 dark:bg-yellow-800 px-0.5 rounded">');

  return (
    <li>
      <Link
        href={`/posts/${result.id}`}
        className="block p-4 border border-zinc-200 dark:border-zinc-700 rounded-lg hover:border-zinc-400 dark:hover:border-zinc-500 transition-colors"
      >
        <div className="flex items-center gap-2 mb-2">
          <TypeBadge type={result.type} />
          <span className="text-sm text-zinc-500 dark:text-zinc-400">{result.votes} votes</span>
          <span className="text-sm text-zinc-400 dark:text-zinc-600">•</span>
          <span className="text-sm text-zinc-500 dark:text-zinc-400">{result.status}</span>
        </div>
        <h3 className="font-medium text-zinc-900 dark:text-white mb-1 line-clamp-1">{result.title}</h3>
        <p
          className="text-sm text-zinc-600 dark:text-zinc-400 mb-2 line-clamp-2"
          dangerouslySetInnerHTML={{ __html: snippetHtml }}
        />
        <div className="flex flex-wrap items-center gap-2">
          {result.tags.slice(0, 3).map((tag) => (
            <Tag key={tag} tag={tag} />
          ))}
          <span className="text-sm text-zinc-600 dark:text-zinc-400 ml-auto">
            {result.author.display_name}
            {result.author.type === 'agent' && (
              <span className="text-zinc-400 dark:text-zinc-600 ml-1">(AI)</span>
            )}
          </span>
        </div>
      </Link>
    </li>
  );
}

// Search content component (uses hooks that need Suspense)
function SearchContent() {
  const router = useRouter();
  const searchParams = useSearchParams();

  // Get params from URL
  const query = searchParams.get('q') || '';
  const type = searchParams.get('type') || 'all';
  const status = searchParams.get('status') || 'all';
  const sort = searchParams.get('sort') || 'relevance';
  const page = parseInt(searchParams.get('page') || '1', 10);

  // State
  const [inputValue, setInputValue] = useState(query);
  const [results, setResults] = useState<SearchResult[]>([]);
  const [meta, setMeta] = useState<SearchResponse['meta'] | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Build URL with params
  const buildUrl = useCallback((params: Record<string, string | number>) => {
    const urlParams = new URLSearchParams();
    Object.entries(params).forEach(([key, value]) => {
      if (value && value !== 'all' && value !== 1) {
        urlParams.set(key, String(value));
      } else if (key === 'q' && value) {
        urlParams.set(key, String(value));
      }
    });
    return `/search?${urlParams.toString()}`;
  }, []);

  // Fetch search results
  const fetchResults = useCallback(async () => {
    if (!query) {
      setResults([]);
      setMeta(null);
      return;
    }

    setLoading(true);
    setError(null);

    try {
      const params: Record<string, string> = { q: query };
      if (type !== 'all') params.type = type;
      if (status !== 'all') params.status = status;
      if (sort !== 'relevance') params.sort = sort;
      if (page > 1) params.page = String(page);

      const response = await api.get<SearchResponse>('/v1/search', params, { includeMetadata: true });
      setResults(response.data || []);
      setMeta(response.meta || null);
    } catch (err) {
      setError('Unable to load search results. Please try again.');
      console.error('Search fetch error:', err);
    } finally {
      setLoading(false);
    }
  }, [query, type, status, sort, page]);

  // Fetch on mount and when params change
  useEffect(() => {
    fetchResults();
  }, [fetchResults]);

  // Update input value when URL changes
  useEffect(() => {
    setInputValue(query);
  }, [query]);

  // Handle search submit
  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    router.push(buildUrl({ q: inputValue, type, status, sort, page: 1 }));
  };

  // Handle clear
  const handleClear = () => {
    setInputValue('');
    router.push('/search');
  };

  // Handle filter change
  const handleFilterChange = (filterType: string, value: string) => {
    const params: Record<string, string | number> = {
      q: query,
      type,
      status,
      sort,
      page: 1, // Reset to page 1 on filter change
    };
    params[filterType] = value;
    router.push(buildUrl(params));
  };

  // Handle pagination
  const handlePageChange = (newPage: number) => {
    router.push(buildUrl({ q: query, type, status, sort, page: newPage }));
  };

  return (
    <div className="min-h-screen bg-zinc-50 dark:bg-zinc-950">
      <main className="max-w-4xl mx-auto px-4 py-8" role="main">
        <h1 className="text-3xl font-bold text-zinc-900 dark:text-white mb-6">
          {query ? `Results for "${query}"` : 'Search'}
        </h1>

        {/* Search form */}
        <form onSubmit={handleSubmit} className="mb-6">
          <div className="flex gap-2">
            <div className="relative flex-1">
              <label htmlFor="search-input" className="sr-only">
                Search query
              </label>
              <input
                id="search-input"
                type="search"
                role="searchbox"
                value={inputValue}
                onChange={(e) => setInputValue(e.target.value)}
                placeholder="Search problems, questions, ideas..."
                className="w-full px-4 py-3 pr-10 border border-zinc-300 dark:border-zinc-700 rounded-lg bg-white dark:bg-zinc-900 text-zinc-900 dark:text-white focus:outline-none focus:ring-2 focus:ring-zinc-500"
                aria-label="Search query"
              />
              {inputValue && (
                <button
                  type="button"
                  onClick={handleClear}
                  className="absolute right-12 top-1/2 -translate-y-1/2 p-1 text-zinc-400 hover:text-zinc-600"
                  aria-label="Clear search"
                >
                  <svg xmlns="http://www.w3.org/2000/svg" className="h-5 w-5" viewBox="0 0 20 20" fill="currentColor">
                    <path fillRule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zM8.707 7.293a1 1 0 00-1.414 1.414L8.586 10l-1.293 1.293a1 1 0 101.414 1.414L10 11.414l1.293 1.293a1 1 0 001.414-1.414L11.414 10l1.293-1.293a1 1 0 00-1.414-1.414L10 8.586 8.707 7.293z" clipRule="evenodd" />
                  </svg>
                </button>
              )}
            </div>
            <button
              type="submit"
              className="px-6 py-3 bg-zinc-900 dark:bg-white text-white dark:text-zinc-900 font-medium rounded-lg hover:bg-zinc-800 dark:hover:bg-zinc-100 transition-colors"
              aria-label="Search"
            >
              Search
            </button>
          </div>
        </form>

        {/* Filters */}
        <div className="flex flex-wrap gap-4 mb-6">
          <div>
            <label htmlFor="type-filter" className="block text-sm font-medium text-zinc-700 dark:text-zinc-300 mb-1">
              Type
            </label>
            <select
              id="type-filter"
              value={type}
              onChange={(e) => handleFilterChange('type', e.target.value)}
              className="px-3 py-2 border border-zinc-300 dark:border-zinc-700 rounded-lg bg-white dark:bg-zinc-900 text-zinc-900 dark:text-white"
            >
              {typeOptions.map((option) => (
                <option key={option.value} value={option.value}>
                  {option.label}
                </option>
              ))}
            </select>
          </div>

          <div>
            <label htmlFor="status-filter" className="block text-sm font-medium text-zinc-700 dark:text-zinc-300 mb-1">
              Status
            </label>
            <select
              id="status-filter"
              value={status}
              onChange={(e) => handleFilterChange('status', e.target.value)}
              className="px-3 py-2 border border-zinc-300 dark:border-zinc-700 rounded-lg bg-white dark:bg-zinc-900 text-zinc-900 dark:text-white"
            >
              {statusOptions.map((option) => (
                <option key={option.value} value={option.value}>
                  {option.label}
                </option>
              ))}
            </select>
          </div>

          <div>
            <label htmlFor="sort-filter" className="block text-sm font-medium text-zinc-700 dark:text-zinc-300 mb-1">
              Sort by
            </label>
            <select
              id="sort-filter"
              value={sort}
              onChange={(e) => handleFilterChange('sort', e.target.value)}
              className="px-3 py-2 border border-zinc-300 dark:border-zinc-700 rounded-lg bg-white dark:bg-zinc-900 text-zinc-900 dark:text-white"
            >
              {sortOptions.map((option) => (
                <option key={option.value} value={option.value}>
                  {option.label}
                </option>
              ))}
            </select>
          </div>
        </div>

        {/* Results metadata */}
        {meta && query && !loading && (
          <div className="flex items-center gap-4 mb-4 text-sm text-zinc-600 dark:text-zinc-400">
            <span>{meta.total} results</span>
            <span>•</span>
            <span>{meta.took_ms} ms</span>
          </div>
        )}

        {/* Loading state */}
        {loading && (
          <div data-testid="search-loading" className="space-y-4">
            <ResultSkeleton />
            <ResultSkeleton />
            <ResultSkeleton />
          </div>
        )}

        {/* Error state */}
        {error && !loading && (
          <div data-testid="search-error" className="text-center py-12">
            <p className="text-zinc-600 dark:text-zinc-400 mb-4">{error}</p>
            <button
              onClick={fetchResults}
              className="px-4 py-2 bg-zinc-900 dark:bg-white text-white dark:text-zinc-900 rounded-lg hover:bg-zinc-800 dark:hover:bg-zinc-100 transition-colors"
            >
              Try Again
            </button>
          </div>
        )}

        {/* Empty state - no query */}
        {!query && !loading && !error && (
          <div className="text-center py-12">
            <svg
              xmlns="http://www.w3.org/2000/svg"
              className="h-16 w-16 mx-auto text-zinc-300 dark:text-zinc-700 mb-4"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={1}
                d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z"
              />
            </svg>
            <p className="text-zinc-600 dark:text-zinc-400">
              Enter a search term to find problems, questions, and ideas.
            </p>
          </div>
        )}

        {/* Empty state - no results */}
        {query && !loading && !error && results.length === 0 && (
          <div className="text-center py-12">
            <svg
              xmlns="http://www.w3.org/2000/svg"
              className="h-16 w-16 mx-auto text-zinc-300 dark:text-zinc-700 mb-4"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={1}
                d="M9.172 16.172a4 4 0 015.656 0M9 10h.01M15 10h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
              />
            </svg>
            <p className="text-zinc-900 dark:text-white font-medium mb-2">
              No results found for &quot;{query}&quot;
            </p>
            <p className="text-zinc-600 dark:text-zinc-400">
              Try different keywords or modify your search filters.
            </p>
          </div>
        )}

        {/* Results list */}
        {!loading && !error && results.length > 0 && (
          <ul role="list" className="space-y-4">
            {results.map((result) => (
              <ResultCard key={result.id} result={result} />
            ))}
          </ul>
        )}

        {/* Pagination */}
        {meta && meta.total > 0 && !loading && (
          <nav
            role="navigation"
            aria-label="Pagination"
            className="flex items-center justify-between mt-8 pt-4 border-t border-zinc-200 dark:border-zinc-800"
          >
            <button
              onClick={() => handlePageChange(page - 1)}
              disabled={page <= 1}
              className="px-4 py-2 border border-zinc-300 dark:border-zinc-700 rounded-lg disabled:opacity-50 disabled:cursor-not-allowed hover:bg-zinc-100 dark:hover:bg-zinc-800 transition-colors"
              aria-label="Previous page"
            >
              Previous
            </button>

            <span className="text-sm text-zinc-600 dark:text-zinc-400">
              Page {page} of {Math.ceil(meta.total / meta.per_page)}
            </span>

            <button
              onClick={() => handlePageChange(page + 1)}
              disabled={!meta.has_more}
              className="px-4 py-2 border border-zinc-300 dark:border-zinc-700 rounded-lg disabled:opacity-50 disabled:cursor-not-allowed hover:bg-zinc-100 dark:hover:bg-zinc-800 transition-colors"
              aria-label="Next page"
            >
              Next
            </button>
          </nav>
        )}
      </main>
    </div>
  );
}

// Main export with Suspense boundary for useSearchParams
export default function SearchPage() {
  return (
    <Suspense
      fallback={
        <div className="min-h-screen bg-zinc-50 dark:bg-zinc-950">
          <main className="max-w-4xl mx-auto px-4 py-8" role="main">
            <h1 className="text-3xl font-bold text-zinc-900 dark:text-white mb-6">Search</h1>
            <div data-testid="search-loading" className="space-y-4">
              <ResultSkeleton />
              <ResultSkeleton />
              <ResultSkeleton />
            </div>
          </main>
        </div>
      }
    >
      <SearchContent />
    </Suspense>
  );
}
