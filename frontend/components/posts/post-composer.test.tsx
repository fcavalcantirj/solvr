import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';

const createPost = vi.fn();
const push = vi.fn();
let authState = { isAuthenticated: true, isLoading: false, user: { id: 'u1' } };

vi.mock('@/lib/api', () => ({
  api: { createPost: (...a: unknown[]) => createPost(...a) },
}));
vi.mock('@/hooks/use-auth', () => ({ useAuth: () => authState }));
vi.mock('next/navigation', () => ({ useRouter: () => ({ push }) }));

import { PostComposer } from './post-composer';

beforeEach(() => {
  createPost.mockReset();
  push.mockReset();
  authState = { isAuthenticated: true, isLoading: false, user: { id: 'u1' } };
});

describe('PostComposer', () => {
  it('shows one title field, one markdown body field, tags, and visibility — with NO content-type question', () => {
    render(<PostComposer />);
    expect(screen.getByLabelText(/title/i)).toBeInTheDocument();
    expect(screen.getByLabelText(/body/i)).toBeInTheDocument();
    expect(screen.getByLabelText(/tags/i)).toBeInTheDocument();
    // visibility choices exist
    expect(screen.getByRole('button', { name: /^public$/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /^family$/i })).toBeInTheDocument();
    // no content-type selector / wording anywhere
    expect(screen.queryByText(/^type$/i)).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /^problem$/i })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /^idea$/i })).not.toBeInTheDocument();
    expect(document.body.textContent?.toLowerCase()).not.toContain('problem');
    expect(document.body.textContent?.toLowerCase()).not.toContain('idea');
  });

  it('publishes through the canonical API with NO type field and routes to /posts/{id}', async () => {
    createPost.mockResolvedValue({ data: { id: 'new-id' } });
    render(<PostComposer />);
    fireEvent.change(screen.getByLabelText(/title/i), { target: { value: 'A canonical post title' } });
    fireEvent.change(screen.getByLabelText(/body/i), { target: { value: 'Detailed markdown body content here.' } });
    fireEvent.click(screen.getByRole('button', { name: /publish/i }));
    await waitFor(() => expect(createPost).toHaveBeenCalled());
    const payload = createPost.mock.calls[0][0] as Record<string, unknown>;
    expect(payload).not.toHaveProperty('type');
    expect(payload).toMatchObject({ title: 'A canonical post title', description: 'Detailed markdown body content here.' });
    expect(payload).toHaveProperty('visibility');
    await waitFor(() => expect(push).toHaveBeenCalledWith('/posts/new-id'));
  });

  it('displays the API error message and never refers to a problem or idea', async () => {
    createPost.mockRejectedValue(new Error('Title must be at least 10 characters'));
    render(<PostComposer />);
    fireEvent.change(screen.getByLabelText(/title/i), { target: { value: 'x' } });
    fireEvent.click(screen.getByRole('button', { name: /publish/i }));
    await waitFor(() => expect(screen.getByText(/at least 10 characters/i)).toBeInTheDocument());
    expect(push).not.toHaveBeenCalled();
  });

  it('requires authentication before showing the composer form', () => {
    authState = { isAuthenticated: false, isLoading: false, user: null } as never;
    render(<PostComposer />);
    expect(screen.getByText(/sign in to publish/i)).toBeInTheDocument();
    expect(screen.queryByLabelText(/title/i)).not.toBeInTheDocument();
  });
});
