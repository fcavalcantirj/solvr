import { describe, it, expect, vi, beforeEach } from 'vitest';

vi.mock('@/lib/api', () => ({
  api: {
    getPost: vi.fn(),
  },
}));

import { api } from '@/lib/api';
import { generateMetadata } from './page';

describe('Question detail page generateMetadata', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('returns title and description from API post', async () => {
    vi.mocked(api.getPost).mockResolvedValue({
      data: {
        id: 'q1',
        type: 'question',
        title: 'How to handle async errors in Go?',
        description: 'I am looking for best practices on handling errors in concurrent Go code with goroutines and channels.',
        status: 'open',
        upvotes: 3,
        downvotes: 0,
        vote_score: 3,
        view_count: 50,
        author: { id: 'u1', type: 'human', display_name: 'Developer' },
        tags: ['go', 'concurrency'],
        created_at: '2026-01-01T00:00:00Z',
        updated_at: '2026-01-02T00:00:00Z',
      },
    });

    const result = await generateMetadata({ params: Promise.resolve({ id: 'q1' }) });

    expect(result.title).toBe('How to handle async errors in Go?');
    expect(result.openGraph?.url).toBe('/questions/q1');
  });

  it('returns default metadata on API error', async () => {
    vi.mocked(api.getPost).mockRejectedValue(new Error('Not found'));

    const result = await generateMetadata({ params: Promise.resolve({ id: 'bad' }) });

    expect(result.title).toBe('Question');
    expect(result.description).toBe('A question on Solvr');
  });
});
