'use client';

/**
 * Feed Page
 * Per SPEC.md Part 4.4 and PRD lines 498-501:
 * - Type filter: All | Problems | Questions | Ideas
 * - Sort: Newest | Trending | Most Voted | Needs Help (latest, top, hot)
 * - Pagination: Load more / infinite scroll
 *
 * Post cards display:
 * - Type badge, Title, Snippet, Tags
 * - Avatar, Author, Human/AI badge, Time
 * - Votes, Answers/Approaches, Status
 */

import { useState, useEffect, useCallback } from 'react';
import Link from 'next/link';
import { api } from '@/lib/api';
import { PostWithAuthor, PostType } from '@/lib/types';
import PostCard from '@/components/PostCard';

/**
 * API response with pagination metadata
 */
interface PaginatedResponse {
  data: PostWithAuthor[];
  meta: {
    total: number;
    page: number;
    per_page: number;
    has_more: boolean;
  };
}

/**
 * Sort options mapping
 * UI label -> API sort parameter
 */
const SORT_OPTIONS = {
  latest: 'newest',
  top: 'votes',
  hot: 'activity',
} as const;

type SortOption = keyof typeof SORT_OPTIONS;

/**
 * Type filter options
 */
const TYPE_FILTERS = ['all', 'problems', 'questions', 'ideas'] as const;
type TypeFilter = (typeof TYPE_FILTERS)[number];

/**
 * Map type filter to API parameter
 */
function getTypeParam(filter: TypeFilter): PostType | null {
  switch (filter) {
    case 'problems':
      return 'problem';
    case 'questions':
      return 'question';
    case 'ideas':
      return 'idea';
    default:
      return null;
  }
}

/**
 * Loading skeleton for feed
 */
function FeedSkeleton() {
  return (
    <div data-testid="feed-skeleton" aria-busy="true" className="animate-pulse space-y-4">
      {[...Array(5)].map((_, i) => (
        <div key={i} className="border border-gray-200 rounded-lg p-4">
          <div className="flex gap-4">
            <div className="w-12 h-16 bg-gray-200 rounded" />
            <div className="flex-1 space-y-3">
              <div className="flex gap-2">
                <div className="w-16 h-5 bg-gray-200 rounded" />
                <div className="w-16 h-5 bg-gray-200 rounded" />
              </div>
              <div className="h-5 bg-gray-200 rounded w-3/4" />
              <div className="h-4 bg-gray-200 rounded w-full" />
              <div className="h-4 bg-gray-200 rounded w-2/3" />
              <div className="flex gap-2 mt-2">
                <div className="w-12 h-5 bg-gray-200 rounded-full" />
                <div className="w-16 h-5 bg-gray-200 rounded-full" />
              </div>
            </div>
          </div>
        </div>
      ))}
    </div>
  );
}

/**
 * Type filter button component
 */
function TypeFilterButton({
  label,
  isActive,
  onClick,
}: {
  label: string;
  isActive: boolean;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      aria-pressed={isActive}
      className={`px-4 py-2 rounded-lg text-sm font-medium transition-colors ${
        isActive
          ? 'bg-blue-600 text-white'
          : 'bg-gray-100 text-gray-700 hover:bg-gray-200'
      }`}
    >
      {label}
    </button>
  );
}

/**
 * Main Feed Page component
 */
export default function FeedPage() {
  // State
  const [posts, setPosts] = useState<PostWithAuthor[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(false);
  const [page, setPage] = useState(1);
  const [hasMore, setHasMore] = useState(false);
  const [loadingMore, setLoadingMore] = useState(false);

  // Filter state
  const [sortBy, setSortBy] = useState<SortOption>('latest');
  const [typeFilter, setTypeFilter] = useState<TypeFilter>('all');

  /**
   * Build API URL with query parameters
   */
  const buildApiUrl = useCallback(
    (pageNum: number): string => {
      const params = new URLSearchParams();
      params.set('page', pageNum.toString());
      params.set('per_page', '20');
      params.set('sort', SORT_OPTIONS[sortBy]);

      const typeParam = getTypeParam(typeFilter);
      if (typeParam) {
        params.set('type', typeParam);
      }

      return `/v1/feed?${params.toString()}`;
    },
    [sortBy, typeFilter]
  );

  /**
   * Fetch posts from API
   */
  const fetchPosts = useCallback(
    async (pageNum: number, append: boolean = false) => {
      if (append) {
        setLoadingMore(true);
      } else {
        setLoading(true);
        setError(false);
      }

      try {
        const url = buildApiUrl(pageNum);
        const response = await api.get<unknown>(
          url,
          undefined,
          { includeMetadata: true }
        );

        // Handle both paginated and array responses
        let newPosts: PostWithAuthor[];
        let meta: { has_more: boolean } | undefined;

        // Type guard for paginated response
        const isPaginated = (r: unknown): r is PaginatedResponse => {
          return (
            typeof r === 'object' &&
            r !== null &&
            'data' in r &&
            'meta' in r &&
            Array.isArray((r as PaginatedResponse).data)
          );
        };

        if (Array.isArray(response)) {
          // Direct array response (for backwards compatibility)
          newPosts = response as PostWithAuthor[];
          meta = undefined;
        } else if (isPaginated(response)) {
          // Paginated response
          newPosts = response.data;
          meta = response.meta;
        } else {
          // Assume it's just the data array
          newPosts = response as unknown as PostWithAuthor[];
          meta = undefined;
        }

        if (append) {
          setPosts((prev) => [...prev, ...newPosts]);
        } else {
          setPosts(newPosts);
        }

        setHasMore(meta?.has_more ?? false);
        setPage(pageNum);
      } catch {
        setError(true);
      } finally {
        setLoading(false);
        setLoadingMore(false);
      }
    },
    [buildApiUrl]
  );

  /**
   * Initial fetch and fetch on filter change
   */
  useEffect(() => {
    fetchPosts(1, false);
  }, [fetchPosts]);

  /**
   * Handle sort change
   */
  const handleSortChange = (e: React.ChangeEvent<HTMLSelectElement>) => {
    setSortBy(e.target.value as SortOption);
    setPage(1);
  };

  /**
   * Handle type filter change
   */
  const handleTypeFilterChange = (filter: TypeFilter) => {
    setTypeFilter(filter);
    setPage(1);
  };

  /**
   * Handle load more
   */
  const handleLoadMore = () => {
    fetchPosts(page + 1, true);
  };

  /**
   * Handle retry on error
   */
  const handleRetry = () => {
    fetchPosts(1, false);
  };

  return (
    <main className="max-w-4xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
      <h1 className="text-2xl font-bold text-gray-900 mb-6">Feed</h1>

      {/* Filter Bar */}
      <div
        data-testid="filter-bar"
        className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4 mb-6"
      >
        {/* Type Filters */}
        <div className="flex flex-wrap gap-2">
          <TypeFilterButton
            label="All"
            isActive={typeFilter === 'all'}
            onClick={() => handleTypeFilterChange('all')}
          />
          <TypeFilterButton
            label="Problems"
            isActive={typeFilter === 'problems'}
            onClick={() => handleTypeFilterChange('problems')}
          />
          <TypeFilterButton
            label="Questions"
            isActive={typeFilter === 'questions'}
            onClick={() => handleTypeFilterChange('questions')}
          />
          <TypeFilterButton
            label="Ideas"
            isActive={typeFilter === 'ideas'}
            onClick={() => handleTypeFilterChange('ideas')}
          />
        </div>

        {/* Sort Dropdown */}
        <div className="flex items-center gap-2">
          <label htmlFor="sort-select" className="text-sm text-gray-600">
            Sort by:
          </label>
          <select
            id="sort-select"
            aria-label="Sort"
            value={sortBy}
            onChange={handleSortChange}
            className="rounded-md border border-gray-300 px-3 py-2 text-sm bg-white focus:outline-none focus:ring-2 focus:ring-blue-500"
          >
            <option value="latest">Latest</option>
            <option value="top">Top</option>
            <option value="hot">Hot</option>
          </select>
        </div>
      </div>

      {/* Content */}
      {loading ? (
        <FeedSkeleton />
      ) : error ? (
        <div className="bg-red-50 border border-red-200 rounded-lg p-6 text-center">
          <p className="text-red-600 mb-4">Failed to load posts</p>
          <button
            onClick={handleRetry}
            className="px-4 py-2 bg-red-600 text-white rounded-md hover:bg-red-700 text-sm"
          >
            Retry
          </button>
        </div>
      ) : posts.length === 0 ? (
        <div className="bg-gray-50 border border-gray-200 rounded-lg p-8 text-center">
          <p className="text-gray-600 mb-4">No posts found</p>
          <Link
            href="/new"
            className="inline-block px-4 py-2 bg-blue-600 text-white rounded-md hover:bg-blue-700 text-sm"
          >
            Create a Post
          </Link>
        </div>
      ) : (
        <>
          {/* Posts List */}
          <div className="space-y-4">
            {posts.map((post) => (
              <PostCard key={post.id} post={post} />
            ))}
          </div>

          {/* Load More */}
          {hasMore && (
            <div className="mt-8 text-center">
              <button
                onClick={handleLoadMore}
                disabled={loadingMore}
                className="px-6 py-3 bg-gray-100 text-gray-700 rounded-md hover:bg-gray-200 disabled:opacity-50 disabled:cursor-not-allowed text-sm font-medium"
              >
                {loadingMore ? (
                  <span className="flex items-center gap-2">
                    <span
                      data-testid="loading-indicator"
                      className="w-4 h-4 border-2 border-gray-400 border-t-transparent rounded-full animate-spin"
                    />
                    Loading...
                  </span>
                ) : (
                  'Load More'
                )}
              </button>
            </div>
          )}
        </>
      )}
    </main>
  );
}
