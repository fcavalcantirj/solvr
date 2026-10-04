import { render, screen, within } from '@testing-library/react';
import { describe, it, expect } from 'vitest';
import { readFileSync } from 'node:fs';
import { join } from 'node:path';

import { OVERVIEW } from './overview-fixture';
import { RoomPreviewsSection } from './room-previews-section';
import { CommunityTotalsSection } from './community-totals-section';
import { ReusablePostsSection } from './reusable-posts-section';
import { ClosingSection } from './closing-section';

// The index is a live overview served whole by GET /v1/homepage/overview. Every
// heading, window, definition, label, excerpt and relative time in these
// sections is an API string. These tests hold each section to rendering that
// answer and computing nothing.

const read = (file: string) => readFileSync(join(process.cwd(), file), 'utf8');
const squish = (s: string | null) => (s ?? '').replace(/\s+/g, ' ').trim();

const SECTION_FILES = [
  'components/homepage/room-stats-section.tsx',
  'components/homepage/room-activity-section.tsx',
  'components/homepage/room-previews-section.tsx',
  'components/homepage/api-usage-section.tsx',
  'components/homepage/search-stats-section.tsx',
  'components/homepage/community-totals-section.tsx',
  'components/homepage/reusable-posts-section.tsx',
  'components/homepage/closing-section.tsx',
  'components/homepage/use-cases-section.tsx',
];

describe('RoomPreviewsSection', () => {
  // The operator's featured rooms (SPEC Part 26): each one is quoted by what it
  // set out to do and what came out of it, one full-width row per room.
  const PREVIEWS = OVERVIEW.previews!;

  it('renders each featured room as a full-width row: name, purpose, participants, ask and outcome', () => {
    render(<RoomPreviewsSection data={PREVIEWS} />);
    const rows = screen.getAllByTestId('featured-room');
    expect(rows).toHaveLength(PREVIEWS.rooms.length);

    PREVIEWS.rooms.forEach((room, i) => {
      const row = within(rows[i]);
      expect(row.getByRole('link', { name: new RegExp(room.display_name) })).toHaveAttribute('href', room.url);
      expect(row.getByText(room.purpose)).toBeInTheDocument();
      expect(row.getByText(room.message_count_label)).toBeInTheDocument();
      expect(row.getByText(room.last_activity_label)).toBeInTheDocument();
      for (const p of room.participants) {
        expect(row.getByText(p.name)).toBeInTheDocument();
      }
      expect(row.getByText(room.ask!.excerpt)).toBeInTheDocument();
      expect(row.getByText(room.outcome!.excerpt)).toBeInTheDocument();
    });
  });

  it('captions the two quotes with the words the API sends', () => {
    render(<RoomPreviewsSection data={PREVIEWS} />);
    expect(screen.getAllByText(PREVIEWS.ask_label)).toHaveLength(PREVIEWS.rooms.length);
    expect(screen.getAllByText(PREVIEWS.outcome_label)).toHaveLength(PREVIEWS.rooms.length);
    expect(screen.getByText(PREVIEWS.note)).toBeInTheDocument();
  });

  it('labels an excerpt with the API note and links each quote back to its message', () => {
    render(<RoomPreviewsSection data={PREVIEWS} />);
    const outcome = PREVIEWS.rooms[0].outcome!;
    expect(screen.getByText(outcome.excerpt_note!)).toBeInTheDocument();
    const links = screen.getAllByRole('link').map((a) => a.getAttribute('href'));
    expect(links).toContain(outcome.message_url);
    expect(links).toContain(PREVIEWS.rooms[0].ask!.message_url);
  });

  it('shows only the ask for a room that has no outcome yet', () => {
    const single = { ...PREVIEWS, rooms: [{ ...PREVIEWS.rooms[0], outcome: undefined }] };
    render(<RoomPreviewsSection data={single} />);
    expect(screen.getByText(PREVIEWS.ask_label)).toBeInTheDocument();
    expect(screen.queryByText(PREVIEWS.outcome_label)).not.toBeInTheDocument();
  });

  it('degrades cleanly when a featured room has no purpose, participants or quotes', () => {
    const bare = {
      ...PREVIEWS,
      rooms: [{ ...PREVIEWS.rooms[0], purpose: '', participants: [], ask: undefined, outcome: undefined }],
    };
    render(<RoomPreviewsSection data={bare} />);
    expect(screen.getByRole('link', { name: new RegExp(bare.rooms[0].display_name) })).toBeInTheDocument();
    expect(screen.getByText(bare.rooms[0].message_count_label)).toBeInTheDocument();
    // Nothing is invented to fill the gaps.
    expect(screen.queryByText(PREVIEWS.rooms[0].purpose)).not.toBeInTheDocument();
    expect(screen.queryByText(PREVIEWS.ask_label)).not.toBeInTheDocument();
  });

  it('omits the message link when the API sent no anchor for it', () => {
    const anchorless = {
      ...PREVIEWS,
      rooms: [
        {
          ...PREVIEWS.rooms[0],
          ask: { author: 'planner', author_role: 'agent', excerpt: 'no anchor for this one', is_excerpt: false },
          outcome: undefined,
        },
      ],
    };
    render(<RoomPreviewsSection data={anchorless} />);
    expect(screen.getByText('no anchor for this one')).toBeInTheDocument();
    expect(screen.queryByRole('link', { name: /Open the original message/ })).not.toBeInTheDocument();
  });

  it('says how many participants the bounded name list leaves out, as the API words it', () => {
    const crowded = {
      ...PREVIEWS,
      rooms: [{ ...PREVIEWS.rooms[0], participant_count: 20, more_participants_label: '+18 more participants' }],
    };
    render(<RoomPreviewsSection data={crowded} />);
    expect(screen.getByText('+18 more participants')).toBeInTheDocument();
  });
});

describe('CommunityTotalsSection', () => {
  it('renders the all-time totals with the window spelled out', () => {
    render(<CommunityTotalsSection data={OVERVIEW.community} />);
    expect(screen.getByText('2,098')).toBeInTheDocument();
    expect(screen.getByText('1,003')).toBeInTheDocument();
    const metrics = screen.getAllByTestId('overview-metric');
    metrics.forEach((node) => expect(squish(node.textContent)).toContain('all time'));
  });

  // The four totals the section exists to publish: the scale of Solvr, with
  // the two account totals named as REGISTRATIONS rather than as an audience.
  it('names the public rooms, published posts and registered accounts', () => {
    render(<CommunityTotalsSection data={OVERVIEW.community} />);
    expect(screen.getByText(OVERVIEW.community.heading)).toBeInTheDocument();

    for (const metric of OVERVIEW.community.metrics) {
      expect(screen.getByText(metric.label)).toBeInTheDocument();
      expect(screen.getByText(metric.display)).toBeInTheDocument();
    }

    const labels = OVERVIEW.community.metrics.map((m) => m.label);
    expect(labels).toEqual(
      expect.arrayContaining([
        'PUBLIC ROOMS',
        'PUBLISHED POSTS',
        'REGISTERED AGENTS',
        'REGISTERED HUMANS',
      ]),
    );
  });

  it('carries the API caveat that a registration is not a measure of use', () => {
    render(<CommunityTotalsSection data={OVERVIEW.community} />);
    const registration = OVERVIEW.community.metrics.find(
      (m) => m.key === 'registered_agents',
    )!;
    expect(registration.qualifier).toBeTruthy();
    expect(screen.getAllByText(registration.qualifier!).length).toBeGreaterThan(0);
  });

  // v1.3.7: a figure reads first; its window opens the caveat, one click away.
  it('keeps the caveat behind the window it qualifies', () => {
    render(<CommunityTotalsSection data={OVERVIEW.community} />);
    const registration = OVERVIEW.community.metrics.find(
      (m) => m.key === 'registered_agents',
    )!;
    const details = screen.getAllByText(registration.qualifier!)[0].closest('details');
    expect(details).not.toBeNull();
    expect(details!.querySelector('summary')).toHaveTextContent(registration.window);
  });

  it('never calls a registered account an active one', () => {
    render(<CommunityTotalsSection data={OVERVIEW.community} />);
    const rendered = squish(
      screen.getByTestId('overview-section-community').textContent,
    ).toLowerCase();
    expect(rendered).not.toContain('active agents');
    expect(rendered).not.toContain('active humans');
    expect(rendered).not.toContain('active users');
  });

  it('renders an unread counter exactly as the API sent it, never as a zero', () => {
    const unread = {
      ...OVERVIEW.community,
      metrics: OVERVIEW.community.metrics.map((m) => ({ ...m, display: '—' })),
    };
    render(<CommunityTotalsSection data={unread} />);
    expect(screen.getAllByText('—')).toHaveLength(unread.metrics.length);
    expect(screen.queryByText('0')).not.toBeInTheDocument();
  });
});

describe('ReusablePostsSection', () => {
  it('renders each post with the URL, counts and wording the API chose', () => {
    render(<ReusablePostsSection data={OVERVIEW.posts!} />);
    const item = OVERVIEW.posts!.items[0];
    const link = screen.getByRole('link', { name: new RegExp(item.title) });
    expect(link).toHaveAttribute('href', item.url);
    expect(screen.getByText(item.contribution_label)).toBeInTheDocument();
    expect(screen.getByText(item.last_activity_label)).toBeInTheDocument();
    for (const tag of item.tags) {
      expect(screen.getByText(tag)).toBeInTheDocument();
    }
  });

  it('states what makes a post reusable', () => {
    render(<ReusablePostsSection data={OVERVIEW.posts!} />);
    expect(screen.getByText(OVERVIEW.posts!.definition)).toBeInTheDocument();
  });

  it('links on to all posts', () => {
    render(<ReusablePostsSection data={OVERVIEW.posts!} />);
    expect(
      screen.getByRole('link', { name: OVERVIEW.posts!.browse_label }),
    ).toHaveAttribute('href', '/posts');
  });
});

describe('ClosingSection', () => {
  it('closes the page on Connect agents now', () => {
    render(<ClosingSection data={OVERVIEW.closing} />);
    const cta = screen.getByRole('link', { name: OVERVIEW.closing.connect_label });
    expect(cta).toHaveAttribute('href', '/connect');
    expect(cta.className).toContain('bg-foreground');
    expect(screen.getByRole('heading', { level: 2 })).toHaveTextContent(
      OVERVIEW.closing.heading,
    );
  });
});

describe('the homepage sections decide nothing', () => {
  // The API owns every judgement. A section that starts slicing text, counting
  // rows, formatting a percentage or sorting by volume has taken business logic
  // out of the API, which is the one thing this project forbids.
  const FORBIDDEN = [
    /\.slice\(\s*0\s*,/,
    /\.substring\(/,
    /\.toFixed\(/,
    /\.sort\(/,
    /Math\.(round|floor|ceil|max|min)\(/,
    /\*\s*100\b(?![%])/,
    /\/\s*\d+\s*\)/,
  ];

  it.each(SECTION_FILES)('%s computes nothing of its own', (file) => {
    const source = read(file);
    const offenders = source
      .split('\n')
      .map((line, i) => [i + 1, line] as const)
      .filter(([, line]) => FORBIDDEN.some((re) => re.test(line)))
      .map(([n, line]) => `${n}: ${line.trim()}`);
    expect(offenders).toEqual([]);
  });

  it.each(SECTION_FILES)('%s hard-codes no statistic, slug or window', (file) => {
    const source = read(file);
    // Any bare number of three digits or more would be a hard-coded total.
    expect(source).not.toMatch(/(?<![\w-])\d{3,}(?![\w%-])/);
    expect(source).not.toContain('tictactoe');
    expect(source).not.toMatch(/last \d+ (hours|days)/);
  });

  // Tightened from 'py-24 lg:py-32' in v1.3.4 (see design-system.test.tsx).
  it.each(SECTION_FILES)('%s keeps the shared section rhythm', (file) => {
    expect(read(file)).toContain('px-4 sm:px-6 lg:px-12 py-12 lg:py-16');
  });
});
