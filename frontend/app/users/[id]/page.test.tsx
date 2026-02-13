import { describe, it, expect, vi, beforeEach } from 'vitest';

vi.mock('@/lib/api', () => ({
  api: {
    getUserProfile: vi.fn(),
  },
}));

import { api } from '@/lib/api';
import { generateMetadata } from './page';

describe('User profile page generateMetadata', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('returns user display_name as title and bio as description', async () => {
    vi.mocked(api.getUserProfile).mockResolvedValue({
      data: {
        id: 'u1',
        username: 'johndoe',
        display_name: 'John Doe',
        bio: 'Full-stack developer who loves AI agents',
        stats: {
          posts_created: 10,
          contributions: 25,
          reputation: 500,
        },
      },
    });

    const result = await generateMetadata({ params: Promise.resolve({ id: 'u1' }) });

    expect(result.title).toBe('John Doe');
    expect(result.description).toBe('Full-stack developer who loves AI agents');
    expect(result.openGraph?.url).toBe('/users/u1');
  });

  it('uses fallback description when user has no bio', async () => {
    vi.mocked(api.getUserProfile).mockResolvedValue({
      data: {
        id: 'u2',
        username: 'janedoe',
        display_name: 'Jane Doe',
        stats: {
          posts_created: 0,
          contributions: 0,
          reputation: 0,
        },
      },
    });

    const result = await generateMetadata({ params: Promise.resolve({ id: 'u2' }) });

    expect(result.title).toBe('Jane Doe');
    expect(result.description).toBe("Jane Doe's profile on Solvr");
  });

  it('returns default metadata on API error', async () => {
    vi.mocked(api.getUserProfile).mockRejectedValue(new Error('Not found'));

    const result = await generateMetadata({ params: Promise.resolve({ id: 'bad' }) });

    expect(result.title).toBe('User');
    expect(result.description).toBe('A user on Solvr');
  });

  it('includes openGraph and twitter metadata', async () => {
    vi.mocked(api.getUserProfile).mockResolvedValue({
      data: {
        id: 'u3',
        username: 'testuser',
        display_name: 'Test User',
        bio: 'A tester',
        stats: {
          posts_created: 5,
          contributions: 10,
          reputation: 200,
        },
      },
    });

    const result = await generateMetadata({ params: Promise.resolve({ id: 'u3' }) });

    expect(result.openGraph?.title).toBe('Test User');
    expect(result.openGraph?.description).toBe('A tester');
    expect(result.twitter?.title).toBe('Test User');
  });
});
