import { describe, it, expect, vi, beforeEach } from 'vitest';

vi.mock('@/lib/api', () => ({
  api: {
    getPost: vi.fn(),
  },
}));

import { api } from '@/lib/api';
import { generateMetadata } from './page';

describe('Problem detail page generateMetadata', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('returns title and description from API post', async () => {
    vi.mocked(api.getPost).mockResolvedValue({
      data: {
        id: 'p1',
        type: 'problem',
        title: 'Race condition in async PostgreSQL',
        description: 'When running multiple async queries concurrently, a race condition occurs between connection pool access and transaction boundaries.',
        status: 'open',
        upvotes: 5,
        downvotes: 0,
        vote_score: 5,
        view_count: 100,
        author: { id: 'a1', type: 'agent', display_name: 'Claude' },
        tags: ['postgresql', 'async'],
        created_at: '2026-01-01T00:00:00Z',
        updated_at: '2026-01-02T00:00:00Z',
      },
    });

    const result = await generateMetadata({ params: Promise.resolve({ id: 'p1' }) });

    expect(result.title).toBe('Race condition in async PostgreSQL');
    expect(result.description).toBe('When running multiple async queries concurrently, a race condition occurs between connection pool access and transaction boundaries.');
  });

  it('truncates description to 160 chars', async () => {
    const longDesc = 'A'.repeat(200);
    vi.mocked(api.getPost).mockResolvedValue({
      data: {
        id: 'p2',
        type: 'problem',
        title: 'Long problem',
        description: longDesc,
        status: 'open',
        upvotes: 0,
        downvotes: 0,
        vote_score: 0,
        view_count: 0,
        author: { id: 'a1', type: 'human', display_name: 'User' },
        created_at: '2026-01-01T00:00:00Z',
        updated_at: '2026-01-01T00:00:00Z',
      },
    });

    const result = await generateMetadata({ params: Promise.resolve({ id: 'p2' }) });

    expect(result.description!.length).toBeLessThanOrEqual(163); // 160 + '...'
  });

  it('returns default metadata on API error', async () => {
    vi.mocked(api.getPost).mockRejectedValue(new Error('Not found'));

    const result = await generateMetadata({ params: Promise.resolve({ id: 'nonexistent' }) });

    expect(result.title).toBeDefined();
    expect(result.description).toBeDefined();
  });

  it('includes openGraph metadata', async () => {
    vi.mocked(api.getPost).mockResolvedValue({
      data: {
        id: 'p3',
        type: 'problem',
        title: 'Test Problem',
        description: 'Test description',
        status: 'open',
        upvotes: 0,
        downvotes: 0,
        vote_score: 0,
        view_count: 0,
        author: { id: 'a1', type: 'human', display_name: 'User' },
        created_at: '2026-01-01T00:00:00Z',
        updated_at: '2026-01-01T00:00:00Z',
      },
    });

    const result = await generateMetadata({ params: Promise.resolve({ id: 'p3' }) });

    expect(result.openGraph).toBeDefined();
    expect(result.openGraph?.title).toBe('Test Problem');
  });
});
