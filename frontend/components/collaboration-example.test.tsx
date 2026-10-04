import { render, screen, within } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import { CollaborationExample } from './collaboration-example';
import type { APICollaborationExample } from '@/lib/api-types';

// The homepage proof is a REAL collaboration served by the API. Everything the
// section says — the beats, the excerpt labels, the completed/live wording, the
// fallback when the room is gone — is decided by the API. These tests hold the
// component to rendering that answer and inventing nothing.

const mockUseExample = vi.fn();
vi.mock('@/hooks/use-collaboration-example', () => ({
  useCollaborationExample: () => mockUseExample(),
}));

const REAL: APICollaborationExample = {
  kind: 'real',
  state: 'completed',
  label: 'COMPLETED COLLABORATION',
  headline: 'Tic-Tac-Toe Human vs Computer',
  summary:
    'A finished two-agent collaboration in a public room: 9 messages, last one 2 days ago, nobody in the room now.',
  live_agent_count: 0,
  room: {
    slug: 'tictactoe-human-vs-computer-20260920',
    display_name: 'Tic-Tac-Toe Human vs Computer',
    message_count: 9,
    last_active_at: '2026-09-20T19:49:32Z',
  },
  room_url: '/rooms/tictactoe-human-vs-computer-20260920',
  participants: [
    { name: 'raphael_tictactoe_planner', role: 'planner' },
    { name: 'raphael_tictactoe_executor', role: 'executor' },
  ],
  steps: [
    {
      beat: 'planner_directive',
      label: 'PLANNER DIRECTIVE',
      author: 'raphael_tictactoe_planner',
      author_role: 'planner',
      excerpt: 'PLANNER DIRECTIVE v1 — acknowledge after joining, then execute…',
      is_excerpt: true,
      excerpt_note: 'Excerpt — the original message is 1643 characters',
      sequence_num: 2,
      message_url: '/rooms/tictactoe-human-vs-computer-20260920#message-2',
    },
    {
      beat: 'executor_plan',
      label: 'EXECUTOR PLAN',
      author: 'raphael_tictactoe_executor',
      author_role: 'executor',
      excerpt: 'IMPLEMENTATION PLAN: update the scope and refactor the controller…',
      is_excerpt: true,
      excerpt_note: 'Excerpt — the original message is 755 characters',
      sequence_num: 4,
      message_url: '/rooms/tictactoe-human-vs-computer-20260920#message-4',
    },
    {
      beat: 'planner_feedback',
      label: 'PLANNER FEEDBACK',
      author: 'raphael_tictactoe_planner',
      author_role: 'planner',
      excerpt: 'PLAN APPROVED. Proceed exactly as proposed.',
      is_excerpt: false,
      sequence_num: 5,
      message_url: '/rooms/tictactoe-human-vs-computer-20260920#message-5',
    },
    {
      beat: 'implementation_evidence',
      label: 'IMPLEMENTATION EVIDENCE',
      author: 'raphael_tictactoe_executor',
      author_role: 'executor',
      excerpt: 'IMPLEMENTATION COMPLETE; passes remains false pending review…',
      is_excerpt: true,
      excerpt_note: 'Excerpt — the original message is 1152 characters',
      sequence_num: 6,
      message_url: '/rooms/tictactoe-human-vs-computer-20260920#message-6',
    },
    {
      beat: 'final_review',
      label: 'FINAL REVIEW',
      author: 'raphael_tictactoe_planner',
      author_role: 'planner',
      excerpt: 'PLANNER CLOSED — final state independently revalidated.',
      is_excerpt: false,
      sequence_num: 9,
      message_url: '/rooms/tictactoe-human-vs-computer-20260920#message-9',
    },
  ],
  connect_url: '/connect?preset=planner-executor',
  connect_label: 'Try this workflow',
};

const ILLUSTRATIVE: APICollaborationExample = {
  kind: 'illustrative',
  state: 'illustrative',
  label: 'ILLUSTRATIVE WORKFLOW',
  headline: 'How a planner and an executor work together',
  summary:
    'No public example is available right now, so this is an illustrative workflow, not a recorded conversation.',
  live_agent_count: 0,
  participants: [],
  steps: REAL.steps.map((s) => ({
    beat: s.beat,
    label: s.label,
    author_role: s.author_role,
    excerpt: `Illustrative description of ${s.beat}.`,
    is_excerpt: false,
  })),
  connect_url: '/connect?preset=planner-executor',
  connect_label: 'Try this workflow',
};

const read = (file: string) => readFileSync(join(process.cwd(), file), 'utf8');
const squish = (s: string | null) => (s ?? '').replace(/\s+/g, ' ').trim();

beforeEach(() => {
  vi.clearAllMocks();
  mockUseExample.mockReturnValue({ example: REAL, loading: false, error: null });
});

describe('CollaborationExample — the real transcript', () => {
  it('shows the five beats in the order the API returned them', () => {
    render(<CollaborationExample />);
    const beats = screen
      .getAllByTestId('collab-step')
      .map((el) => squish(within(el).getByTestId('collab-step-label').textContent));
    expect(beats).toEqual([
      'PLANNER DIRECTIVE',
      'EXECUTOR PLAN',
      'PLANNER FEEDBACK',
      'IMPLEMENTATION EVIDENCE',
      'FINAL REVIEW',
    ]);
  });

  it('keeps the transcript authors accurate, with the role each one played', () => {
    render(<CollaborationExample />);
    const steps = screen.getAllByTestId('collab-step');
    expect(squish(within(steps[0]).getByTestId('collab-step-author').textContent)).toContain(
      'raphael_tictactoe_planner',
    );
    expect(squish(within(steps[0]).getByTestId('collab-step-author').textContent)).toContain(
      'planner',
    );
    expect(squish(within(steps[1]).getByTestId('collab-step-author').textContent)).toContain(
      'raphael_tictactoe_executor',
    );
  });

  it('renders each excerpt exactly as the API worded it', () => {
    render(<CollaborationExample />);
    for (const step of REAL.steps) {
      expect(screen.getByText(step.excerpt)).toBeInTheDocument();
    }
  });

  it('labels shortened messages as excerpts and leaves whole messages unlabelled', () => {
    render(<CollaborationExample />);
    const steps = screen.getAllByTestId('collab-step');

    expect(squish(within(steps[0]).getByTestId('collab-step-excerpt-note').textContent)).toBe(
      'Excerpt — the original message is 1643 characters',
    );
    expect(within(steps[2]).queryByTestId('collab-step-excerpt-note')).toBeNull();
  });

  it('links every beat to the original message it was taken from', () => {
    render(<CollaborationExample />);
    const links = screen
      .getAllByTestId('collab-step')
      .map((el) => within(el).getByTestId('collab-step-link').getAttribute('href'));
    expect(links).toEqual(REAL.steps.map((s) => s.message_url));
  });

  it('labels the example as completed, using the API wording', () => {
    render(<CollaborationExample />);
    expect(screen.getByTestId('collab-state-label').textContent).toContain(
      'COMPLETED COLLABORATION',
    );
    expect(screen.getByText(REAL.summary)).toBeInTheDocument();
  });

  it('calls it live only when the API says the room is live', () => {
    mockUseExample.mockReturnValue({
      example: {
        ...REAL,
        state: 'live',
        label: 'LIVE COLLABORATION',
        live_agent_count: 2,
        summary: '2 agents are in this public room right now.',
      },
      loading: false,
      error: null,
    });
    render(<CollaborationExample />);
    expect(screen.getByTestId('collab-state-label').textContent).toContain('LIVE COLLABORATION');
  });

  it('opens the original room with the API-supplied link', () => {
    render(<CollaborationExample />);
    expect(screen.getByTestId('collab-room-link').getAttribute('href')).toBe(REAL.room_url);
  });

  it('is the section the hero\'s Watch an example jumps to', () => {
    const { container } = render(<CollaborationExample />);
    const section = container.querySelector('section#example');
    expect(section).not.toBeNull();
    // Clear of the fixed header when the page scrolls to it.
    expect(section!.className).toContain('scroll-mt-24');
  });

  it('offers Try this workflow pointing at the planner/executor preset', () => {
    render(<CollaborationExample />);
    const cta = screen.getByTestId('collab-connect-link');
    expect(cta.getAttribute('href')).toBe('/connect?preset=planner-executor');
    expect(squish(cta.textContent)).toBe('Try this workflow');
  });
});

describe('CollaborationExample — when the room is gone', () => {
  beforeEach(() => {
    mockUseExample.mockReturnValue({ example: ILLUSTRATIVE, loading: false, error: null });
  });

  it('says plainly that the workflow is illustrative', () => {
    render(<CollaborationExample />);
    expect(screen.getByTestId('collab-state-label').textContent).toContain(
      'ILLUSTRATIVE WORKFLOW',
    );
    expect(screen.getByText(ILLUSTRATIVE.summary)).toBeInTheDocument();
  });

  it('shows no transcript author, no message link and no room link', () => {
    render(<CollaborationExample />);
    expect(screen.queryByTestId('collab-step-author')).toBeNull();
    expect(screen.queryByTestId('collab-step-link')).toBeNull();
    expect(screen.queryByTestId('collab-room-link')).toBeNull();
    expect(screen.queryByText(/raphael_tictactoe/)).toBeNull();
  });

  it('is still there for Watch an example to land on', () => {
    const { container } = render(<CollaborationExample />);
    expect(container.querySelector('section#example')).not.toBeNull();
  });

  it('still explains all five beats', () => {
    render(<CollaborationExample />);
    expect(screen.getAllByTestId('collab-step')).toHaveLength(5);
  });

  it('keeps a working Connect agents action', () => {
    render(<CollaborationExample />);
    expect(screen.getByTestId('collab-connect-link').getAttribute('href')).toBe(
      '/connect?preset=planner-executor',
    );
  });
});

describe('CollaborationExample — loading and failure', () => {
  it('shows a loading state instead of inventing a transcript', () => {
    mockUseExample.mockReturnValue({ example: null, loading: true, error: null });
    render(<CollaborationExample />);
    expect(screen.getByTestId('collab-loading')).toBeInTheDocument();
    expect(screen.queryByTestId('collab-step')).toBeNull();
  });

  it('keeps a working Connect action when the API call fails', () => {
    mockUseExample.mockReturnValue({ example: null, loading: false, error: 'network down' });
    render(<CollaborationExample />);
    expect(screen.queryByTestId('collab-step')).toBeNull();
    expect(screen.getByTestId('collab-connect-link').getAttribute('href')).toContain('/connect');
  });
});

describe('CollaborationExample — the client stays dumb', () => {
  const source = () => read('components/collaboration-example.tsx');

  it('hard-codes no transcript, no room slug and no beat copy', () => {
    const src = source();
    expect(src).not.toContain('tictactoe');
    expect(src).not.toContain('PLANNER DIRECTIVE');
    expect(src).not.toContain('raphael_');
    // The excerpt limit is the API's decision, not a client-side substring.
    expect(src).not.toMatch(/\.slice\(0,\s*\d+\)/);
    expect(src).not.toMatch(/substring\(/);
  });

  it('decides neither the completed/live wording nor the excerpt labels', () => {
    const src = source();
    expect(src).not.toContain('COMPLETED COLLABORATION');
    expect(src).not.toContain('LIVE COLLABORATION');
    expect(src).not.toContain('ILLUSTRATIVE WORKFLOW');
    expect(src).not.toMatch(/['"`]Excerpt/);
  });

  // Tightened from 'py-24 lg:py-32' in v1.3.4 (see design-system.test.tsx).
  it('keeps the shared homepage section rhythm', () => {
    expect(source()).toContain('px-4 sm:px-6 lg:px-12 py-12 lg:py-16');
  });
});
