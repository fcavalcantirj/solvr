import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, waitFor, act } from '@testing-library/react';

// What opening a post page tells the API: one view per browser session per post
// (POST /v1/posts/{id}/view), and one share_visit when the page was opened through a
// share link (?via=share). Neither was sent by the site before: 218 of 467 posts showed
// 0 views, and a shared post could not be told from any other visit.

const useAuthMock = vi.fn();
vi.mock('@/hooks/use-auth', () => ({ useAuth: () => useAuthMock() }));
vi.mock('@/lib/api', () => ({ api: { recordView: vi.fn(), postFunnelEvent: vi.fn() } }));

import { api } from '@/lib/api';
import { PostPageSignals } from './post-page-signals';

const anonymous = { user: null, isAuthenticated: false, isLoading: false };

function at(path: string) {
  window.history.replaceState(null, '', path);
}

// Lets the view request of a mounted page settle.
const settle = () => act(async () => {});

beforeEach(() => {
  vi.mocked(api.recordView).mockReset();
  vi.mocked(api.recordView).mockResolvedValue({ data: { view_count: 11 } });
  vi.mocked(api.postFunnelEvent).mockReset();
  useAuthMock.mockReset();
  useAuthMock.mockReturnValue(anonymous);
  window.sessionStorage.clear();
  at('/posts/p1');
});

describe('PostPageSignals: the view', () => {
  it('records one view of the post, under this browser session', async () => {
    const { container } = render(<PostPageSignals postId="p1" />);

    await waitFor(() => expect(api.recordView).toHaveBeenCalledTimes(1));
    const [postId, sessionId] = vi.mocked(api.recordView).mock.calls[0];
    expect(postId).toBe('p1');
    // The session id is this tab's own, made here and kept in session storage only.
    expect(sessionId).toBeTruthy();
    expect(sessionId).toBe(window.sessionStorage.getItem('solvr_session_id'));
    // It draws nothing on the page.
    expect(container.innerHTML).toBe('');
  });

  it('does not record the same post again in the same session', async () => {
    const first = render(<PostPageSignals postId="p1" />);
    await waitFor(() => expect(api.recordView).toHaveBeenCalledTimes(1));
    await waitFor(() => expect(window.sessionStorage.getItem('solvr_viewed_posts')).toContain('p1'));
    first.unmount();

    // The reader comes back to the post later in the same tab.
    render(<PostPageSignals postId="p1" />);
    await new Promise((resolve) => setTimeout(resolve, 20));

    expect(api.recordView).toHaveBeenCalledTimes(1);
  });

  it('records another post of the same session once as well', async () => {
    const first = render(<PostPageSignals postId="p1" />);
    await waitFor(() => expect(api.recordView).toHaveBeenCalledTimes(1));
    await waitFor(() => expect(window.sessionStorage.getItem('solvr_viewed_posts')).toContain('p1'));
    first.unmount();

    render(<PostPageSignals postId="p2" />);

    await waitFor(() => expect(api.recordView).toHaveBeenCalledTimes(2));
    expect(vi.mocked(api.recordView).mock.calls[1][0]).toBe('p2');
    // One reader, one session id.
    expect(vi.mocked(api.recordView).mock.calls[1][1]).toBe(vi.mocked(api.recordView).mock.calls[0][1]);
  });

  // The API counts a signed-in reader under their own account and an anonymous one under
  // the session id. The view therefore waits until the stored session is read, or every
  // signed-in reader would be sent before their credential is attached.
  it('waits until the session is read before recording', async () => {
    useAuthMock.mockReturnValue({ user: null, isAuthenticated: false, isLoading: true });
    const { rerender } = render(<PostPageSignals postId="p1" />);
    await new Promise((resolve) => setTimeout(resolve, 20));
    expect(api.recordView).not.toHaveBeenCalled();

    useAuthMock.mockReturnValue({
      user: { id: 'u-1', type: 'human', displayName: 'Dev' },
      isAuthenticated: true,
      isLoading: false,
    });
    rerender(<PostPageSignals postId="p1" />);

    await waitFor(() => expect(api.recordView).toHaveBeenCalledTimes(1));
  });

  it('stays silent when the view cannot be recorded', async () => {
    vi.mocked(api.recordView).mockRejectedValue(new Error('post not found'));

    const { container } = render(<PostPageSignals postId="p1" />);

    await waitFor(() => expect(api.recordView).toHaveBeenCalledTimes(1));
    await new Promise((resolve) => setTimeout(resolve, 20));
    expect(container.innerHTML).toBe('');
    // A view that was not recorded is not remembered as recorded.
    expect(window.sessionStorage.getItem('solvr_viewed_posts')).toBeNull();
  });
});

describe('PostPageSignals: the share visit', () => {
  it('reports one share_visit for the post when it was opened through a share link, and cleans the URL', async () => {
    at('/posts/p1?via=share#r2');

    render(<PostPageSignals postId="p1" />);

    expect(api.postFunnelEvent).toHaveBeenCalledTimes(1);
    expect(api.postFunnelEvent).toHaveBeenCalledWith({
      event: 'share_visit',
      entry_surface: 'post_page',
      source: { kind: 'post', ref: 'p1' },
    });
    expect(window.location.search).toBe('');
    expect(window.location.hash).toBe('#r2');
    await settle();
  });

  it('counts the same tab once', async () => {
    at('/posts/p1?via=share');
    const first = render(<PostPageSignals postId="p1" />);
    await settle();
    first.unmount();
    at('/posts/p1?via=share');
    render(<PostPageSignals postId="p1" />);
    await settle();

    expect(api.postFunnelEvent).toHaveBeenCalledTimes(1);
  });

  it('reports no share visit for an ordinary visit', async () => {
    render(<PostPageSignals postId="p1" />);
    await settle();

    expect(api.postFunnelEvent).not.toHaveBeenCalled();
  });
});
