import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen } from '@testing-library/react';

// A public room's server HTML tells an agent how to join it (SPEC.md 18.3): the room's own
// join sentence, read from the API (GET /v1/rooms/{slug}/connect?role=collaborator), with a
// Copy button and a plain link to the skill. An agent that was given only the room's URL
// reads both in the page it fetched. A private room, a finished room and a room whose
// sentence could not be read get no block; the page renders either way.

vi.mock('react', async (importOriginal) => ({
  ...(await importOriginal<typeof import('react')>()),
  cache: <T,>(fn: T) => fn,
}));
vi.mock('@/components/header', () => ({ Header: () => null }));
vi.mock('@/components/rooms/room-detail-client', () => ({
  RoomDetailClient: ({ agentJoin }: { agentJoin?: React.ReactNode }) => <div data-testid="room-detail">{agentJoin}</div>,
}));
vi.mock('@/components/rooms/private-room-view', () => ({
  PrivateRoomView: ({ slug }: { slug: string }) => <div data-testid="private-gate">{slug}</div>,
}));
vi.mock('@/components/seo/json-ld', () => ({ JsonLd: () => null, roomJsonLd: () => ({}), breadcrumbJsonLd: () => ({}) }));
vi.mock('next/navigation', () => ({
  notFound: () => {
    throw new Error('NEXT_NOT_FOUND');
  },
}));

import RoomDetailPage from './page';

const SENTENCE =
  'Learn Solvr from https://solvr.dev/skill.md. Join the public Solvr room "The room" at https://solvr.dev/rooms/the-room as the COLLABORATOR, read it, and help with the work pinned there, and post what you did.';

function connectPayload() {
  return {
    data: {
      instruction_version: '2.2',
      room_slug: 'the-room',
      room_url: 'https://solvr.dev/rooms/the-room',
      private: false,
      role: 'collaborator',
      task: '',
      prompt: {
        text: SENTENCE,
        word_count: SENTENCE.split(/\s+/).length,
        segments: [
          { kind: 'text', text: 'Learn Solvr from ' },
          { kind: 'link', text: 'https://solvr.dev/skill.md' },
          { kind: 'text', text: SENTENCE.slice('Learn Solvr from https://solvr.dev/skill.md'.length) },
        ],
      },
    },
  };
}

function roomPayload(archived = false) {
  return {
    data: {
      room: {
        id: 'r1',
        slug: 'the-room',
        display_name: 'The room',
        message_count: 3,
        created_at: '2026-09-01T00:00:00Z',
        last_active_at: '2026-09-02T00:00:00Z',
        archived_at: archived ? '2026-09-03T00:00:00Z' : null,
      },
      agents: [],
      recent_messages: [],
    },
  };
}

const fetchMock = vi.fn();

function apiAnswers({ room = 200, archived = false, connect = 200 }: { room?: number; archived?: boolean; connect?: number }) {
  fetchMock.mockImplementation(async (url: string) => {
    const u = String(url);
    if (u.includes('/connect?')) {
      return { ok: connect === 200, status: connect, json: async () => (connect === 200 ? connectPayload() : null) };
    }
    if (u.endsWith('/seo')) return { ok: true, status: 200, json: async () => ({ data: { indexable: true, title: 'The room', description: 'd' } }) };
    if (u.endsWith('/posts')) return { ok: true, status: 200, json: async () => ({ data: [] }) };
    return { ok: room === 200, status: room, json: async () => (room === 200 ? roomPayload(archived) : null) };
  });
}

const params = () => ({ params: Promise.resolve({ slug: 'the-room' }) });
const connectCalls = () => fetchMock.mock.calls.filter(([url]) => String(url).includes('/connect?'));

beforeEach(() => {
  fetchMock.mockReset();
  vi.stubGlobal('fetch', fetchMock);
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe('a room page lets an agent join', () => {
  it("renders the room's join sentence, a Copy button and a plain link to the skill", async () => {
    apiAnswers({});
    render(await RoomDetailPage(params()));

    const block = screen.getByTestId('agent-join');
    expect(block.textContent).toContain('Let your agent join');
    expect(screen.getByTestId('agent-join-sentence').textContent).toBe(SENTENCE);
    expect(screen.getByRole('button', { name: /copy/i })).toBeInTheDocument();
    const skill = screen.getByRole('link', { name: 'https://solvr.dev/skill.md' });
    expect(skill.getAttribute('href')).toBe('https://solvr.dev/skill.md');

    const [url, init] = connectCalls()[0];
    expect(String(url)).toContain('/v1/rooms/the-room/connect?role=collaborator');
    expect(init).toMatchObject({ cache: 'no-store' });
  });

  it('renders no block for a private room, and never asks for its sentence', async () => {
    apiAnswers({ room: 403 });
    render(await RoomDetailPage(params()));
    expect(screen.getByTestId('private-gate')).toBeInTheDocument();
    expect(screen.queryByTestId('agent-join')).toBeNull();
    expect(connectCalls()).toHaveLength(0);
  });

  it('renders no block for a finished room: there is nothing to join', async () => {
    apiAnswers({ archived: true });
    render(await RoomDetailPage(params()));
    expect(screen.getByTestId('room-detail')).toBeInTheDocument();
    expect(screen.queryByTestId('agent-join')).toBeNull();
    expect(connectCalls()).toHaveLength(0);
  });

  it('renders the room without the block when its sentence cannot be read', async () => {
    apiAnswers({ connect: 500 });
    render(await RoomDetailPage(params()));
    expect(screen.getByTestId('room-detail')).toBeInTheDocument();
    expect(screen.queryByTestId('agent-join')).toBeNull();
  });
});
