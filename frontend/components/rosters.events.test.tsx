import { describe, it, expect, vi, beforeEach } from 'vitest';
import { act, fireEvent, render, screen, waitFor } from '@testing-library/react';

// The three rosters: /agents, /users and /leaderboard. What a visitor does with them, as
// Google Analytics events (SPEC.md 27.7): sort_change, filter_change and load_more, each
// sent once, after the list that was asked for arrived.

const track = vi.hoisted(() => vi.fn());
vi.mock('@/lib/analytics', () => ({ track, trackOnNextPage: vi.fn() }));

vi.mock('next/link', () => ({
  default: ({ children, href, ...props }: { children: React.ReactNode; href: string; [key: string]: unknown }) => (
    <a href={href} {...props}>
      {children}
    </a>
  ),
}));
vi.mock('@/components/agents/agents-sidebar', () => ({ AgentsSidebar: () => null }));
vi.mock('@/lib/api', () => ({
  api: { getAgents: vi.fn(), getUsers: vi.fn(), getLeaderboard: vi.fn() },
  formatRelativeTime: () => '2d ago',
}));

import { api } from '@/lib/api';
import { AgentsPageClient } from '@/components/agents/agents-page-client';
import { UsersPageClient } from '@/components/users/users-page-client';
import { LeaderboardPageClient } from '@/components/leaderboard/leaderboard-page-client';

const eventsNamed = (name: string) => track.mock.calls.filter(([event]) => event === name);

const agent = (i: number) => ({
  id: `agent_${i}`,
  display_name: `Agent ${i}`,
  bio: '',
  status: 'active',
  reputation: 100 - i,
  post_count: i,
  has_human_backed_badge: false,
  created_at: '2026-09-01T00:00:00Z',
});
const agentsPage = (from: number, has_more: boolean) => ({
  data: Array.from({ length: 20 }, (_, i) => agent(from + i)),
  meta: { total: 45, has_more, active_count: 3, human_backed_count: 1 },
});

const user = (i: number) => ({
  id: `user-${i}`,
  username: `user${i}`,
  display_name: `User ${i}`,
  avatar_url: '',
  reputation: 100 - i,
  agents_count: 1,
  created_at: '2026-09-01T00:00:00Z',
});
const usersPage = (from: number, has_more: boolean) => ({
  data: Array.from({ length: 20 }, (_, i) => user(from + i)),
  meta: { total: 45, total_backed_agents: 7, has_more },
});

const entry = (i: number) => ({
  rank: i,
  id: `entry-${i}`,
  type: i % 2 ? 'agent' : 'user',
  display_name: `Entry ${i}`,
  reputation: 1000 - i,
  key_stats: { problems_solved: 1, answers_accepted: 1, upvotes_received: 1, total_contributions: 1 },
});
const boardPage = (from: number, has_more: boolean) => ({
  data: Array.from({ length: 50 }, (_, i) => entry(from + i)),
  meta: { total: 120, page: 1, per_page: 50, has_more },
});

/** A read whose answer the test releases by hand. */
function held<T>() {
  let release: (value: T) => void = () => {};
  const promise = new Promise<T>((resolve) => (release = resolve));
  return { promise, release };
}

beforeEach(() => {
  track.mockReset();
  vi.mocked(api.getAgents).mockReset();
  vi.mocked(api.getUsers).mockReset();
  vi.mocked(api.getLeaderboard).mockReset();
  vi.mocked(api.getAgents).mockResolvedValue(agentsPage(1, true) as never);
  vi.mocked(api.getUsers).mockResolvedValue(usersPage(1, true) as never);
  vi.mocked(api.getLeaderboard).mockResolvedValue(boardPage(1, true) as never);
});

describe('/agents', () => {
  async function open() {
    render(<AgentsPageClient initialAgentData={[]} />);
    await screen.findByRole('button', { name: /load more/i });
    await act(async () => {});
  }

  it('sends nothing for opening the page', async () => {
    await open();
    expect(track).not.toHaveBeenCalled();
  });

  it('sends sort_change once, after the roster in the new order arrived', async () => {
    await open();
    const reads = [held<unknown>(), held<unknown>()];
    vi.mocked(api.getAgents)
      .mockReturnValueOnce(reads[0].promise as never)
      .mockReturnValueOnce(reads[1].promise as never);

    fireEvent.click(screen.getByRole('button', { name: /^newest$/i }));
    await waitFor(() => expect(api.getAgents).toHaveBeenLastCalledWith(expect.objectContaining({ sort: 'newest' })));
    expect(track).not.toHaveBeenCalled();

    await act(async () => {
      reads[0].release(agentsPage(1, true));
      reads[1].release(agentsPage(1, true));
    });
    // The page reads the list twice (its counts and its roster): one change, one event.
    expect(track).toHaveBeenCalledTimes(1);
    expect(track).toHaveBeenCalledWith('sort_change', { list: 'agents', sort: 'newest' });
  });

  it('does not send sort_change for the order already showing, nor when the roster could not be loaded', async () => {
    await open();
    fireEvent.click(screen.getByRole('button', { name: /^rep$/i }));
    await act(async () => {});
    expect(track).not.toHaveBeenCalled();

    vi.mocked(api.getAgents).mockRejectedValue(new Error('down'));
    vi.spyOn(console, 'error').mockImplementation(() => {});
    fireEvent.click(screen.getByRole('button', { name: /^oldest$/i }));
    await waitFor(() => expect(api.getAgents).toHaveBeenLastCalledWith(expect.objectContaining({ sort: 'oldest' })));
    await act(async () => {});
    expect(track).not.toHaveBeenCalled();
    vi.mocked(console.error).mockRestore();
  });

  it('sends load_more with the page that was added, once it arrived', async () => {
    await open();
    vi.mocked(api.getAgents).mockResolvedValue(agentsPage(21, true) as never);

    fireEvent.click(screen.getByRole('button', { name: /load more/i }));
    await waitFor(() => expect(track).toHaveBeenCalledTimes(1));
    expect(api.getAgents).toHaveBeenLastCalledWith(expect.objectContaining({ page: 2 }));
    expect(track).toHaveBeenCalledWith('load_more', { list: 'agents', page: 2 });
  });

  it('does not send load_more when the next page could not be loaded', async () => {
    await open();
    vi.mocked(api.getAgents).mockRejectedValue(new Error('down'));
    vi.spyOn(console, 'error').mockImplementation(() => {});

    fireEvent.click(screen.getByRole('button', { name: /load more/i }));
    await waitFor(() => expect(api.getAgents).toHaveBeenLastCalledWith(expect.objectContaining({ page: 2 })));
    await act(async () => {});
    expect(track).not.toHaveBeenCalled();
    vi.mocked(console.error).mockRestore();
  });
});

describe('/users', () => {
  async function open() {
    render(<UsersPageClient initialUserData={[]} />);
    await screen.findByRole('button', { name: /load more/i });
    await act(async () => {});
  }

  it('sends nothing for opening the page', async () => {
    await open();
    expect(track).not.toHaveBeenCalled();
  });

  it('sends sort_change once, after the roster in the new order arrived', async () => {
    await open();
    fireEvent.click(screen.getByRole('button', { name: /^agents$/i }));

    await waitFor(() => expect(track).toHaveBeenCalledTimes(1));
    expect(api.getUsers).toHaveBeenLastCalledWith(expect.objectContaining({ sort: 'agents' }));
    expect(track).toHaveBeenCalledWith('sort_change', { list: 'users', sort: 'agents' });

    // The order already showing is not a change.
    fireEvent.click(screen.getByRole('button', { name: /^agents$/i }));
    await act(async () => {});
    expect(track).toHaveBeenCalledTimes(1);
  });

  it('does not send sort_change when the roster could not be loaded', async () => {
    await open();
    vi.mocked(api.getUsers).mockRejectedValue(new Error('down'));

    fireEvent.click(screen.getByRole('button', { name: /^newest$/i }));
    await waitFor(() => expect(api.getUsers).toHaveBeenLastCalledWith(expect.objectContaining({ sort: 'newest' })));
    await act(async () => {});
    expect(track).not.toHaveBeenCalled();
  });

  it('sends load_more with the page that was added', async () => {
    await open();
    vi.mocked(api.getUsers).mockResolvedValue(usersPage(21, true) as never);

    fireEvent.click(screen.getByRole('button', { name: /load more/i }));
    await waitFor(() => expect(track).toHaveBeenCalledTimes(1));
    expect(api.getUsers).toHaveBeenLastCalledWith(expect.objectContaining({ offset: 20 }));
    expect(track).toHaveBeenCalledWith('load_more', { list: 'users', page: 2 });
  });
});

describe('/leaderboard', () => {
  async function open() {
    render(<LeaderboardPageClient initialEntries={[]} />);
    await screen.findByRole('button', { name: /load more/i });
    await act(async () => {});
  }

  it('sends nothing for opening the page', async () => {
    await open();
    expect(track).not.toHaveBeenCalled();
  });

  it('sends sort_change for the period, after its ranking arrived', async () => {
    await open();
    const read = held<unknown>();
    vi.mocked(api.getLeaderboard).mockReturnValueOnce(read.promise as never);

    fireEvent.click(screen.getByRole('button', { name: /this week/i }));
    await waitFor(() => expect(api.getLeaderboard).toHaveBeenLastCalledWith(expect.objectContaining({ timeframe: 'weekly' })));
    expect(track).not.toHaveBeenCalled();

    await act(async () => {
      read.release(boardPage(1, false));
    });
    expect(track).toHaveBeenCalledTimes(1);
    expect(track).toHaveBeenCalledWith('sort_change', { list: 'leaderboard', sort: 'weekly' });
  });

  it('sends filter_change for humans or agents, after that ranking arrived', async () => {
    await open();
    fireEvent.click(screen.getByRole('button', { name: /^agents$/i }));

    await waitFor(() => expect(track).toHaveBeenCalledTimes(1));
    expect(api.getLeaderboard).toHaveBeenLastCalledWith(expect.objectContaining({ type: 'agents' }));
    expect(track).toHaveBeenCalledWith('filter_change', { list: 'leaderboard', item: 'agents' });

    fireEvent.click(screen.getByRole('button', { name: /^humans$/i }));
    await waitFor(() => expect(eventsNamed('filter_change')).toHaveLength(2));
    expect(eventsNamed('filter_change')[1][1]).toEqual({ list: 'leaderboard', item: 'users' });
    expect(eventsNamed('sort_change')).toEqual([]);
  });

  it('sends neither when the ranking could not be loaded', async () => {
    await open();
    vi.mocked(api.getLeaderboard).mockRejectedValue(new Error('down'));
    vi.spyOn(console, 'error').mockImplementation(() => {});

    fireEvent.click(screen.getByRole('button', { name: /this month/i }));
    await waitFor(() => expect(api.getLeaderboard).toHaveBeenLastCalledWith(expect.objectContaining({ timeframe: 'monthly' })));
    await act(async () => {});
    expect(track).not.toHaveBeenCalled();
    vi.mocked(console.error).mockRestore();
  });

  it('sends load_more with the page that was added', async () => {
    await open();
    vi.mocked(api.getLeaderboard).mockResolvedValue(boardPage(51, true) as never);

    fireEvent.click(screen.getByRole('button', { name: /load more/i }));
    await waitFor(() => expect(track).toHaveBeenCalledTimes(1));
    expect(api.getLeaderboard).toHaveBeenLastCalledWith(expect.objectContaining({ offset: 50 }));
    expect(track).toHaveBeenCalledWith('load_more', { list: 'leaderboard', page: 2 });
  });
});
