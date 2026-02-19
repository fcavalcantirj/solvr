import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { renderHook, act } from '@testing-library/react';
import { useSearch } from './use-posts';

// Mock the API module
vi.mock('@/lib/api', () => ({
  api: {
    search: vi.fn(),
  },
  formatRelativeTime: () => '1h ago',
  truncateText: (text: string, len: number) => text.substring(0, len),
  mapStatus: (status: string) => status.toUpperCase(),
}));

import { api } from '@/lib/api';

describe('useSearch searchMethod', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('returns searchMethod as "hybrid" when API responds with hybrid method', async () => {
    vi.mocked(api.search).mockResolvedValue({
      data: [{
        id: 'post-1',
        type: 'problem',
        title: 'Test',
        description: 'Test desc',
        tags: [],
        status: 'open',
        vote_score: 0,
        author: { type: 'human', id: 'u1', display_name: 'User' },
        created_at: '2026-01-01T00:00:00Z',
        comments_count: 0,
        view_count: 0,
        answers_count: 0,
        approaches_count: 0,
        snippet: 'Test snippet',
        score: 0.95,
      }],
      meta: {
        query: 'test',
        total: 1,
        page: 1,
        per_page: 20,
        has_more: false,
        took_ms: 15,
        method: 'hybrid',
      },
    });

    const { result } = renderHook(() => useSearch('test query'));

    // Wait for debounce and API call to resolve
    await act(async () => {
      await vi.waitFor(() => {
        expect(api.search).toHaveBeenCalled();
      });
    });

    expect(result.current.searchMethod).toBe('hybrid');
  });

  it('returns searchMethod as "fulltext" when API responds with fulltext method', async () => {
    vi.mocked(api.search).mockResolvedValue({
      data: [],
      meta: {
        query: 'test',
        total: 0,
        page: 1,
        per_page: 20,
        has_more: false,
        took_ms: 5,
        method: 'fulltext',
      },
    });

    const { result } = renderHook(() => useSearch('test query'));

    await act(async () => {
      await vi.waitFor(() => {
        expect(api.search).toHaveBeenCalled();
      });
    });

    expect(result.current.searchMethod).toBe('fulltext');
  });

  it('returns searchMethod as undefined when query is empty', () => {
    const { result } = renderHook(() => useSearch(''));

    expect(result.current.searchMethod).toBeUndefined();
  });

  it('resets searchMethod when query is cleared', async () => {
    vi.mocked(api.search).mockResolvedValue({
      data: [],
      meta: {
        query: 'test',
        total: 0,
        page: 1,
        per_page: 20,
        has_more: false,
        took_ms: 5,
        method: 'hybrid',
      },
    });

    const { result, rerender } = renderHook(
      ({ query }) => useSearch(query),
      { initialProps: { query: 'test query' } }
    );

    await act(async () => {
      await vi.waitFor(() => {
        expect(api.search).toHaveBeenCalled();
      });
    });

    expect(result.current.searchMethod).toBe('hybrid');

    // Clear the query
    rerender({ query: '' });

    expect(result.current.searchMethod).toBeUndefined();
  });
});
