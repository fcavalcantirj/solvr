import { describe, it, expect, vi, beforeEach } from 'vitest';

vi.mock('@/lib/api', () => ({
  api: {
    getPost: vi.fn(),
  },
}));

import { api } from '@/lib/api';
import { generateMetadata } from './page';

describe('Idea detail page generateMetadata', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('returns title and description from API post', async () => {
    vi.mocked(api.getPost).mockResolvedValue({
      data: {
        id: 'i1',
        type: 'idea',
        title: 'AI agents should share debugging context',
        description: 'What if AI agents could share their debugging context with each other through a shared knowledge layer?',
        status: 'open',
        upvotes: 10,
        downvotes: 1,
        vote_score: 9,
        view_count: 200,
        author: { id: 'a1', type: 'agent', display_name: 'Claudius' },
        tags: ['ai', 'debugging'],
        created_at: '2026-01-01T00:00:00Z',
        updated_at: '2026-01-02T00:00:00Z',
      },
    });

    const result = await generateMetadata({ params: Promise.resolve({ id: 'i1' }) });

    expect(result.title).toBe('AI agents should share debugging context');
    expect(result.openGraph?.url).toBe('/ideas/i1');
  });

  it('returns default metadata on API error', async () => {
    vi.mocked(api.getPost).mockRejectedValue(new Error('Not found'));

    const result = await generateMetadata({ params: Promise.resolve({ id: 'bad' }) });

    expect(result.title).toBe('Idea');
    expect(result.description).toBe('An idea on Solvr');
  });
});
