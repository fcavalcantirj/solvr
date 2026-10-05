import { describe, it, expect, vi, beforeEach } from 'vitest';
import { act, fireEvent, render, screen, waitFor } from '@testing-library/react';

// Making and reacting to content, as Google Analytics events (SPEC.md 27.7): room_create,
// room_comment, post_create, blog_vote, blog_share and page_not_found. Each is sent once,
// after the API (or the clipboard) accepted the action, and never carries what was written.

const track = vi.hoisted(() => vi.fn());
const trackOnNextPage = vi.hoisted(() => vi.fn());
vi.mock('@/lib/analytics', () => ({ track, trackOnNextPage }));

const push = vi.fn();
let pathname = '/no/such/page';
vi.mock('next/navigation', () => ({
  useRouter: () => ({ push }),
  usePathname: () => pathname,
}));
vi.mock('next/link', () => ({
  default: ({ children, href, ...props }: { children: React.ReactNode; href: string; [key: string]: unknown }) => (
    <a href={href} {...props}>
      {children}
    </a>
  ),
}));

const auth = vi.hoisted(() => ({
  isAuthenticated: true,
  isLoading: false,
  user: { id: 'u1', type: 'human', displayName: 'Jane' } as null | { id: string; type: string; displayName: string },
  showAuthWall: vi.fn(),
  setShowAuthModal: vi.fn(),
}));
vi.mock('@/hooks/use-auth', () => ({ useAuth: () => auth }));

vi.mock('@/lib/api', () => ({
  api: {
    createRoom: vi.fn(),
    postRoomMessage: vi.fn(),
    createPost: vi.fn(),
    voteBlogPost: vi.fn(),
    recordBlogView: vi.fn(),
  },
}));

import { api } from '@/lib/api';
import { CreateRoomDialog } from '@/components/rooms/create-room-dialog';
import { CommentInput } from '@/components/rooms/comment-input';
import { PostComposer } from '@/components/posts/post-composer';
import { BlogPostClient } from '@/app/blog/[slug]/blog-post-client';
import NotFound from '@/app/not-found';

const eventsNamed = (name: string) => track.mock.calls.filter(([event]) => event === name);
const everythingSent = () => JSON.stringify([track.mock.calls, trackOnNextPage.mock.calls]);

beforeEach(() => {
  track.mockReset();
  trackOnNextPage.mockReset();
  push.mockReset();
  auth.showAuthWall.mockReset();
  Object.assign(auth, { isAuthenticated: true, user: { id: 'u1', type: 'human', displayName: 'Jane' } });
  for (const fn of Object.values(api)) vi.mocked(fn as ReturnType<typeof vi.fn>).mockReset();
  vi.mocked(api.recordBlogView).mockResolvedValue(undefined);
  pathname = '/no/such/page';
});

describe('room_create (the dialog on /rooms)', () => {
  async function create(name = 'Secret launch plan') {
    render(<CreateRoomDialog />);
    fireEvent.click(screen.getByRole('button', { name: /create room/i }));
    fireEvent.change(screen.getByPlaceholderText(/room name/i), { target: { value: name } });
    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: /^create$/i }));
    });
  }

  it.each([
    [false, 'public'],
    [true, 'private'],
  ])('is sent once the API created the room, with the visibility the API answered (is_private %s)', async (is_private, visibility) => {
    vi.mocked(api.createRoom).mockResolvedValue({ data: { slug: 'secret-launch-plan', id: 'room-1', display_name: 'Secret launch plan', is_private } } as never);
    await create();

    await waitFor(() => expect(push).toHaveBeenCalledWith('/rooms/secret-launch-plan?created=1'));
    expect(track).toHaveBeenCalledTimes(1);
    expect(track).toHaveBeenCalledWith('room_create', { visibility });
    expect(everythingSent()).not.toMatch(/secret|launch|room-1/i);
  });

  it('is not sent when the API refused the room', async () => {
    vi.mocked(api.createRoom).mockRejectedValue(Object.assign(new Error('conflict'), { status: 409 }));
    await create();

    expect(await screen.findByText(/already exists/i)).toBeInTheDocument();
    expect(track).not.toHaveBeenCalled();
  });

  it('asks for a session, naming the action, when a logged-out visitor presses Create Room', () => {
    Object.assign(auth, { isAuthenticated: false, user: null });
    render(<CreateRoomDialog />);
    fireEvent.click(screen.getByRole('button', { name: /create room/i }));

    expect(auth.showAuthWall).toHaveBeenCalledTimes(1);
    expect(auth.showAuthWall).toHaveBeenCalledWith('room_create');
    expect(screen.queryByText(/room name/i)).toBeNull();
    expect(track).not.toHaveBeenCalled();
  });
});

describe('room_comment', () => {
  function send(text = 'the parser hangs on empty input') {
    const onMessageSent = vi.fn();
    render(<CommentInput slug="demo-room" onMessageSent={onMessageSent} />);
    fireEvent.change(screen.getByPlaceholderText('Type a message...'), { target: { value: text } });
    fireEvent.submit(screen.getByPlaceholderText('Type a message...').closest('form')!);
    return onMessageSent;
  }

  it('is sent once the API accepted the comment: never its words, never the room', async () => {
    vi.mocked(api.postRoomMessage).mockResolvedValue({ data: { id: 7 } } as never);
    const onMessageSent = send();

    await waitFor(() => expect(onMessageSent).toHaveBeenCalledTimes(1));
    expect(track).toHaveBeenCalledTimes(1);
    expect(track.mock.calls[0][0]).toBe('room_comment');
    expect(track.mock.calls[0][1]).toBeUndefined();
    expect(everythingSent()).not.toMatch(/parser|demo-room/);
  });

  it('is not sent when the API refused it, and the page says why, where the comment was typed', async () => {
    vi.mocked(api.postRoomMessage).mockRejectedValue(Object.assign(new Error('rate'), { status: 429 }));
    send();

    const alert = await screen.findByRole('alert');
    expect(alert).toHaveTextContent('Slow down — try again in a few seconds');
    expect(track).not.toHaveBeenCalled();
    // What was typed is still there to send again.
    expect(screen.getByPlaceholderText('Type a message...')).toHaveValue('the parser hangs on empty input');
  });

  it('says a failed post failed, and takes the message away once the next one goes through', async () => {
    vi.mocked(api.postRoomMessage).mockRejectedValueOnce(new Error('down'));
    send();
    expect(await screen.findByRole('alert')).toHaveTextContent('Failed to post — please try again.');

    vi.mocked(api.postRoomMessage).mockResolvedValue({ data: { id: 8 } } as never);
    fireEvent.submit(screen.getByPlaceholderText('Type a message...').closest('form')!);
    await waitFor(() => expect(track).toHaveBeenCalledTimes(1));
    expect(screen.queryByRole('alert')).toBeNull();
  });
});

describe('post_create', () => {
  function publish(visibility?: 'family') {
    render(<PostComposer />);
    fireEvent.change(screen.getByPlaceholderText('A clear, descriptive title'), { target: { value: 'Deadlock in the pool' } });
    fireEvent.change(screen.getByPlaceholderText('Write the details in Markdown.'), { target: { value: 'It hangs under load.' } });
    if (visibility) fireEvent.click(screen.getByRole('button', { name: /family/i }));
    fireEvent.click(screen.getByRole('button', { name: /publish post/i }));
  }

  it('is sent once the API accepted the post, with who may read it: never the title', async () => {
    vi.mocked(api.createPost).mockResolvedValue({ data: { id: 'post-9' } } as never);
    publish();

    await waitFor(() => expect(push).toHaveBeenCalledWith('/posts/post-9'));
    expect(track).toHaveBeenCalledTimes(1);
    expect(track).toHaveBeenCalledWith('post_create', { visibility: 'public' });
    expect(everythingSent()).not.toMatch(/deadlock|hangs|post-9/i);
  });

  it('says family for a post only linked agents may read', async () => {
    vi.mocked(api.createPost).mockResolvedValue({ data: { id: 'post-9' } } as never);
    publish('family');

    await waitFor(() => expect(track).toHaveBeenCalledTimes(1));
    expect(api.createPost).toHaveBeenCalledWith(expect.objectContaining({ visibility: 'family' }));
    expect(track).toHaveBeenCalledWith('post_create', { visibility: 'family' });
  });

  it('is not sent when the API refused the post', async () => {
    vi.mocked(api.createPost).mockRejectedValue(new Error('Title is too short'));
    publish();

    expect(await screen.findByText('Title is too short')).toBeInTheDocument();
    expect(track).not.toHaveBeenCalled();
    expect(push).not.toHaveBeenCalled();
  });
});

describe('blog_vote and blog_share', () => {
  const blog = () => render(<BlogPostClient slug="what-we-shipped" initialVoteScore={3} initialUserVote={null} viewCount={10} />);

  it.each(['up', 'down'] as const)('sends blog_vote %s once the API counted the vote', async (direction) => {
    vi.mocked(api.voteBlogPost).mockResolvedValue({ data: { vote_score: 4, user_vote: direction } } as never);
    blog();
    fireEvent.click(screen.getByTestId(`vote-${direction}`));

    await waitFor(() => expect(track).toHaveBeenCalledTimes(1));
    expect(track).toHaveBeenCalledWith('blog_vote', { direction });
    expect(everythingSent()).not.toContain('what-we-shipped');
  });

  it('does not send blog_vote when the API refused the vote', async () => {
    vi.mocked(api.voteBlogPost).mockRejectedValue(new Error('down'));
    blog();
    fireEvent.click(screen.getByTestId('vote-up'));
    await waitFor(() => expect(api.voteBlogPost).toHaveBeenCalledTimes(1));
    await act(async () => {});

    expect(track).not.toHaveBeenCalled();
  });

  it('asks a logged-out reader for a session, naming the action, and counts no vote', () => {
    Object.assign(auth, { isAuthenticated: false, user: null });
    blog();
    fireEvent.click(screen.getByTestId('vote-up'));

    expect(auth.showAuthWall).toHaveBeenCalledWith('blog_vote');
    expect(api.voteBlogPost).not.toHaveBeenCalled();
    expect(track).not.toHaveBeenCalled();
  });

  it('sends blog_share once the link is on the clipboard, and nothing when the clipboard is blocked', async () => {
    const writeText = vi.fn().mockResolvedValue(undefined);
    Object.assign(navigator, { clipboard: { writeText } });
    blog();
    fireEvent.click(screen.getByTestId('share-button'));

    await waitFor(() => expect(track).toHaveBeenCalledTimes(1));
    expect(track).toHaveBeenCalledWith('blog_share', { method: 'clipboard' });

    track.mockClear();
    writeText.mockRejectedValue(new Error('denied'));
    fireEvent.click(screen.getByTestId('share-button'));
    await waitFor(() => expect(writeText).toHaveBeenCalledTimes(2));
    await act(async () => {});
    expect(track).not.toHaveBeenCalled();
  });
});

describe('page_not_found', () => {
  it('is sent once when a page that does not exist is shown, with nothing of its own', () => {
    const { rerender } = render(<NotFound />);

    expect(track).toHaveBeenCalledTimes(1);
    expect(track.mock.calls[0][0]).toBe('page_not_found');
    expect(track.mock.calls[0][1]).toBeUndefined();

    // Drawing the same missing page again is not a second miss.
    rerender(<NotFound />);
    expect(track).toHaveBeenCalledTimes(1);
  });

  it('is sent again for another missing address reached without a page load', () => {
    const { rerender } = render(<NotFound />);
    pathname = '/another/missing/page';
    rerender(<NotFound />);

    expect(eventsNamed('page_not_found')).toHaveLength(2);
  });
});
