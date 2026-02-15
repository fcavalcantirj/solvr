import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderHook, waitFor } from '@testing-library/react';
import { useContributions } from './use-contributions';

// Mock the API module
vi.mock('@/lib/api', () => ({
  api: {
    getUserContributions: vi.fn(),
  },
  formatRelativeTime: vi.fn((date: string) => '2d ago'),
}));

import { api } from '@/lib/api';
import type { APIContributionsResponse } from '@/lib/api-types';

const mockContributionsResponse: APIContributionsResponse = {
  data: [
    {
      type: 'answer',
      id: 'answer-1',
      parent_id: 'question-1',
      parent_title: 'How to use React hooks?',
      parent_type: 'question',
      content_preview: 'You can use useState and useEffect...',
      status: '',
      created_at: '2026-02-10T10:00:00Z',
    },
    {
      type: 'approach',
      id: 'approach-1',
      parent_id: 'problem-1',
      parent_title: 'Fix async race condition',
      parent_type: 'problem',
      content_preview: 'Use mutex locks to prevent...',
      status: 'working',
      created_at: '2026-02-09T10:00:00Z',
    },
    {
      type: 'response',
      id: 'response-1',
      parent_id: 'idea-1',
      parent_title: 'AI-powered code review',
      parent_type: 'idea',
      content_preview: 'This is a great idea because...',
      status: '',
      created_at: '2026-02-08T10:00:00Z',
    },
  ],
  meta: {
    total: 3,
    page: 1,
    per_page: 20,
    has_more: false,
  },
};

describe('useContributions', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('fetches contributions from GET /v1/users/{id}/contributions', async () => {
    vi.mocked(api.getUserContributions).mockResolvedValueOnce(mockContributionsResponse);

    const { result } = renderHook(() => useContributions('user-123'));

    expect(result.current.loading).toBe(true);

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    expect(api.getUserContributions).toHaveBeenCalledWith('user-123', {
      page: 1,
      per_page: 20,
    });
    expect(result.current.contributions).toHaveLength(3);
    expect(result.current.error).toBeNull();
  });

  it('transforms answer contributions with parent title', async () => {
    vi.mocked(api.getUserContributions).mockResolvedValueOnce(mockContributionsResponse);

    const { result } = renderHook(() => useContributions('user-123'));

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    const answer = result.current.contributions.find(c => c.type === 'answer');
    expect(answer).toBeDefined();
    expect(answer!.parentTitle).toBe('How to use React hooks?');
    expect(answer!.parentType).toBe('question');
    expect(answer!.contentPreview).toBe('You can use useState and useEffect...');
  });

  it('transforms approach contributions with status', async () => {
    vi.mocked(api.getUserContributions).mockResolvedValueOnce(mockContributionsResponse);

    const { result } = renderHook(() => useContributions('user-123'));

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    const approach = result.current.contributions.find(c => c.type === 'approach');
    expect(approach).toBeDefined();
    expect(approach!.parentTitle).toBe('Fix async race condition');
    expect(approach!.parentType).toBe('problem');
    expect(approach!.status).toBe('working');
  });

  it('transforms response contributions correctly', async () => {
    vi.mocked(api.getUserContributions).mockResolvedValueOnce(mockContributionsResponse);

    const { result } = renderHook(() => useContributions('user-123'));

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    const response = result.current.contributions.find(c => c.type === 'response');
    expect(response).toBeDefined();
    expect(response!.parentTitle).toBe('AI-powered code review');
    expect(response!.parentType).toBe('idea');
  });

  it('supports type filter parameter', async () => {
    vi.mocked(api.getUserContributions).mockResolvedValueOnce({
      data: [mockContributionsResponse.data[0]],
      meta: { total: 1, page: 1, per_page: 20, has_more: false },
    } as APIContributionsResponse);

    const { result } = renderHook(() =>
      useContributions('user-123', { type: 'answers' })
    );

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    expect(api.getUserContributions).toHaveBeenCalledWith('user-123', {
      type: 'answers',
      page: 1,
      per_page: 20,
    });
  });

  it('supports pagination with loadMore', async () => {
    vi.mocked(api.getUserContributions).mockResolvedValueOnce({
      ...mockContributionsResponse,
      meta: { total: 5, page: 1, per_page: 3, has_more: true },
    } as APIContributionsResponse);

    const { result } = renderHook(() => useContributions('user-123'));

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    expect(result.current.hasMore).toBe(true);
    expect(result.current.contributions).toHaveLength(3);

    vi.mocked(api.getUserContributions).mockResolvedValueOnce({
      data: [
        {
          type: 'answer' as const,
          id: 'answer-2',
          parent_id: 'question-2',
          parent_title: 'Another question',
          parent_type: 'question' as const,
          content_preview: 'More content...',
          status: '',
          created_at: '2026-02-07T10:00:00Z',
        },
      ],
      meta: { total: 5, page: 2, per_page: 3, has_more: false },
    });

    result.current.loadMore();

    await waitFor(() => {
      expect(result.current.contributions).toHaveLength(4);
    });

    expect(result.current.hasMore).toBe(false);
  });

  it('handles API errors gracefully', async () => {
    vi.mocked(api.getUserContributions).mockRejectedValueOnce(
      new Error('Network error')
    );

    const { result } = renderHook(() => useContributions('user-123'));

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    expect(result.current.error).toBe('Network error');
    expect(result.current.contributions).toHaveLength(0);
  });
});
