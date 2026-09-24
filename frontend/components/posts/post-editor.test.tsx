import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import type { APIPost } from '@/lib/api-types';

const getPost = vi.fn();
const updatePost = vi.fn();
const push = vi.fn();

vi.mock('@/lib/api', () => ({
  api: {
    getPost: (...a: unknown[]) => getPost(...a),
    updatePost: (...a: unknown[]) => updatePost(...a),
  },
}));
vi.mock('next/navigation', () => ({ useRouter: () => ({ push }) }));

import { PostEditor } from './post-editor';

function makePost(over: Partial<APIPost> = {}): APIPost {
  return {
    id: 'p1',
    type: 'idea',
    title: 'Original title here',
    description: 'Original body content.',
    status: 'open',
    upvotes: 5,
    downvotes: 1,
    vote_score: 4,
    view_count: 20,
    author: { id: 'author-9', type: 'human', display_name: 'Dev Nine' },
    tags: ['alpha', 'beta'],
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z',
    source_room_id: 'room-42',
    ...over,
  };
}

beforeEach(() => {
  getPost.mockReset();
  updatePost.mockReset();
  push.mockReset();
  getPost.mockResolvedValue({ data: makePost() });
});

describe('PostEditor', () => {
  it('prefills the form from the existing post', async () => {
    render(<PostEditor postId="p1" />);
    await waitFor(() => expect(screen.getByLabelText(/title/i)).toHaveValue('Original title here'));
    expect(screen.getByLabelText(/body/i)).toHaveValue('Original body content.');
    expect(screen.getByText('alpha')).toBeInTheDocument();
    expect(screen.getByText('beta')).toBeInTheDocument();
  });

  it('saves only title, body, and tags — never resetting author, dates, votes, or source', async () => {
    updatePost.mockResolvedValue({ data: makePost({ title: 'Edited title here' }) });
    render(<PostEditor postId="p1" />);
    await waitFor(() => expect(screen.getByLabelText(/title/i)).toHaveValue('Original title here'));
    fireEvent.change(screen.getByLabelText(/title/i), { target: { value: 'Edited title here' } });
    fireEvent.click(screen.getByRole('button', { name: /save/i }));
    await waitFor(() => expect(updatePost).toHaveBeenCalled());
    expect(updatePost.mock.calls[0][0]).toBe('p1');
    const payload = updatePost.mock.calls[0][1] as Record<string, unknown>;
    expect(Object.keys(payload).sort()).toEqual(['description', 'tags', 'title']);
    expect(payload).not.toHaveProperty('author');
    expect(payload).not.toHaveProperty('created_at');
    expect(payload).not.toHaveProperty('vote_score');
    expect(payload).not.toHaveProperty('source_room_id');
    await waitFor(() => expect(push).toHaveBeenCalledWith('/posts/p1'));
  });

  it('displays the API error message on a failed save', async () => {
    updatePost.mockRejectedValue(new Error('You are not the author'));
    render(<PostEditor postId="p1" />);
    await waitFor(() => expect(screen.getByLabelText(/title/i)).toHaveValue('Original title here'));
    fireEvent.click(screen.getByRole('button', { name: /save/i }));
    await waitFor(() => expect(screen.getByText(/not the author/i)).toBeInTheDocument());
    expect(push).not.toHaveBeenCalled();
  });
});
