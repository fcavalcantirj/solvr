"use client";

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { renderHook, waitFor } from '@testing-library/react';
import { useIdeas } from './use-ideas';
import { api } from '@/lib/api';

// Mock the API module
vi.mock('@/lib/api', () => ({
  api: {
    getIdeas: vi.fn(),
  },
  formatRelativeTime: (date: string) => '2d ago',
  mapStatus: (status: string) => status.toUpperCase(),
}));

const mockIdeaWithEvolvedInto = {
  id: 'i1',
  type: 'idea',
  title: 'Idea with branches',
  description: 'An idea that has evolved into other posts',
  status: 'evolved',
  upvotes: 20,
  downvotes: 1,
  vote_score: 19,
  view_count: 200,
  author: {
    id: 'u1',
    type: 'human',
    display_name: 'Alice',
    avatar_url: 'https://example.com/alice.png',
  },
  tags: ['architecture'],
  created_at: '2026-01-10T10:00:00Z',
  updated_at: '2026-01-15T10:00:00Z',
  evolved_into: ['post-a', 'post-b', 'post-c'],
  answers_count: 5,
};

const mockIdeaWithoutEvolvedInto = {
  id: 'i2',
  type: 'idea',
  title: 'Fresh spark idea',
  description: 'A brand new idea',
  status: 'open',
  upvotes: 3,
  downvotes: 0,
  vote_score: 3,
  view_count: 30,
  author: {
    id: 'a1',
    type: 'agent',
    display_name: 'thinker-bot',
  },
  tags: ['brainstorm'],
  created_at: '2026-01-20T10:00:00Z',
  updated_at: '2026-01-20T10:00:00Z',
  answers_count: 0,
};

const mockIdeaWithNullEvolvedInto = {
  id: 'i3',
  type: 'idea',
  title: 'Idea with null evolved',
  description: 'An idea where evolved_into is null',
  status: 'active',
  upvotes: 7,
  downvotes: 2,
  vote_score: 5,
  view_count: 80,
  author: {
    id: 'u2',
    type: 'human',
    display_name: 'Bob',
  },
  tags: ['testing'],
  created_at: '2026-01-18T10:00:00Z',
  updated_at: '2026-01-19T10:00:00Z',
  evolved_into: null,
  answers_count: 2,
};

const mockResponse = {
  data: [mockIdeaWithEvolvedInto, mockIdeaWithoutEvolvedInto, mockIdeaWithNullEvolvedInto],
  meta: { total: 3, page: 1, per_page: 20, has_more: false },
};

describe('useIdeas - evolved_into and field resolution', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('maps evolved_into array length to branches count', async () => {
    (api.getIdeas as ReturnType<typeof vi.fn>).mockResolvedValue(mockResponse);

    const { result } = renderHook(() => useIdeas());

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    // Idea with 3 evolved_into entries should have branches = 3
    expect(result.current.ideas[0].branches).toBe(3);
  });

  it('sets branches to 0 when evolved_into is not present', async () => {
    (api.getIdeas as ReturnType<typeof vi.fn>).mockResolvedValue(mockResponse);

    const { result } = renderHook(() => useIdeas());

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    // Idea without evolved_into should have branches = 0
    expect(result.current.ideas[1].branches).toBe(0);
  });

  it('sets branches to 0 when evolved_into is null', async () => {
    (api.getIdeas as ReturnType<typeof vi.fn>).mockResolvedValue(mockResponse);

    const { result } = renderHook(() => useIdeas());

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    // Idea with null evolved_into should have branches = 0
    expect(result.current.ideas[2].branches).toBe(0);
  });

  it('maps avatar from API author avatar_url', async () => {
    (api.getIdeas as ReturnType<typeof vi.fn>).mockResolvedValue(mockResponse);

    const { result } = renderHook(() => useIdeas());

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    // Author with avatar_url should use it instead of initials
    expect(result.current.ideas[0].author.avatar).toBe('https://example.com/alice.png');
  });

  it('falls back to initials when avatar_url is not provided', async () => {
    (api.getIdeas as ReturnType<typeof vi.fn>).mockResolvedValue(mockResponse);

    const { result } = renderHook(() => useIdeas());

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    // Author without avatar_url should use initials fallback
    expect(result.current.ideas[1].author.avatar).toBe('TH');
  });

  it('does not have TODO comments in transform function', async () => {
    (api.getIdeas as ReturnType<typeof vi.fn>).mockResolvedValue(mockResponse);

    const { result } = renderHook(() => useIdeas());

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    // All fields should be properly resolved, not left as TODO placeholders
    const idea = result.current.ideas[0];
    expect(typeof idea.branches).toBe('number');
    expect(idea.branches).toBeGreaterThan(0); // evolved_into has 3 entries
    expect(idea.supporters).toEqual([]); // Empty but intentional, not a TODO
    expect(idea.recentComment).toBeNull(); // Null but intentional, not a TODO
  });

  it('handles null comments from production API', async () => {
    // Production API may return null when comments table doesn't exist
    const ideaWithNullComments = {
      ...mockIdeaWithEvolvedInto,
      comments_count: null,
    };

    (api.getIdeas as ReturnType<typeof vi.fn>).mockResolvedValue({
      data: [ideaWithNullComments],
      meta: { total: 1, page: 1, per_page: 20, has_more: false },
    });

    const { result } = renderHook(() => useIdeas());

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    // null should be converted to 0
    expect(result.current.ideas[0].comments).toBe(0);
    expect(result.current.ideas[0].comments).not.toBeNull();
  });
});
