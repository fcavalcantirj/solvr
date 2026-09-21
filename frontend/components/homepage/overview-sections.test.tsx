import { render, screen, within } from '@testing-library/react';
import { describe, it, expect } from 'vitest';
import { readFileSync } from 'node:fs';
import { join } from 'node:path';

import { OVERVIEW } from './overview-fixture';
import { RoomStatsSection } from './room-stats-section';
import { RoomPreviewsSection } from './room-previews-section';
import { ApiUsageSection } from './api-usage-section';
import { SearchStatsSection } from './search-stats-section';
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
];

describe('RoomStatsSection', () => {
  it('renders the API heading and every metric it was sent', () => {
    render(<RoomStatsSection data={OVERVIEW.rooms} />);
    expect(screen.getByRole('heading', { level: 2 })).toHaveTextContent(
      OVERVIEW.rooms.heading,
    );
    for (const metric of OVERVIEW.rooms.metrics) {
      expect(screen.getByText(metric.label)).toBeInTheDocument();
      expect(screen.getByText(metric.display)).toBeInTheDocument();
    }
  });

  it('shows the window and the definition of every number, not just the number', () => {
    render(<RoomStatsSection data={OVERVIEW.rooms} />);
    const metrics = screen.getAllByTestId('overview-metric');
    expect(metrics).toHaveLength(OVERVIEW.rooms.metrics.length);
    metrics.forEach((node, i) => {
      const metric = OVERVIEW.rooms.metrics[i];
      expect(squish(node.textContent)).toContain(metric.window);
      expect(within(node).getByTitle(metric.definition)).toBeInTheDocument();
    });
  });

  it('renders the numbers in the monospaced treatment', () => {
    render(<RoomStatsSection data={OVERVIEW.rooms} />);
    const value = screen.getByText('1,234');
    expect(value.className).toContain('font-mono');
  });

  it('draws the sparkline from the heights the API normalised', () => {
    render(<RoomStatsSection data={OVERVIEW.rooms} />);
    const bars = screen.getAllByTestId('overview-spark-bar');
    expect(bars).toHaveLength(3);
    expect(bars[0].style.height).toBe('0%');
    expect(bars[1].style.height).toBe('50%');
    expect(bars[2].style.height).toBe('100%');
    // The percentages come straight from the payload, not from a calculation.
    expect(OVERVIEW.rooms.sparkline!.points.map((p) => p.height)).toEqual([
      '0.0%',
      '50.0%',
      '100.0%',
    ]);
    expect(screen.getByText(OVERVIEW.rooms.sparkline!.label)).toBeInTheDocument();
    expect(
      screen.getByText(new RegExp(OVERVIEW.rooms.sparkline!.window)),
    ).toBeInTheDocument();
  });

  it('omits the sparkline entirely when the API sent none', () => {
    render(<RoomStatsSection data={{ ...OVERVIEW.rooms, sparkline: undefined }} />);
    expect(screen.queryByTestId('overview-spark-bar')).not.toBeInTheDocument();
  });

  it('renders without an intro when the API sent none', () => {
    render(<RoomStatsSection data={{ ...OVERVIEW.rooms, intro: '' }} />);
    expect(screen.getByRole('heading', { level: 2 })).toHaveTextContent(
      OVERVIEW.rooms.heading,
    );
    expect(screen.queryByText(OVERVIEW.rooms.intro)).not.toBeInTheDocument();
  });

  it('links on to all rooms with the API label', () => {
    render(<RoomStatsSection data={OVERVIEW.rooms} />);
    const link = screen.getByRole('link', { name: OVERVIEW.rooms.rooms_label });
    expect(link).toHaveAttribute('href', '/rooms');
  });
});

describe('RoomPreviewsSection', () => {
  it('renders each selected room with purpose, participants and the exchange', () => {
    render(<RoomPreviewsSection data={OVERVIEW.previews} />);
    const preview = OVERVIEW.previews.rooms[0];

    expect(screen.getByText(preview.display_name)).toBeInTheDocument();
    expect(screen.getByText(preview.purpose)).toBeInTheDocument();
    expect(screen.getByText(preview.last_activity_label)).toBeInTheDocument();
    expect(screen.getByText(preview.message_count_label)).toBeInTheDocument();

    for (const p of preview.participants) {
      expect(screen.getByText(p.name)).toBeInTheDocument();
      expect(screen.getByText(p.message_label)).toBeInTheDocument();
    }
    for (const m of preview.exchange) {
      expect(screen.getByText(m.excerpt)).toBeInTheDocument();
    }
    expect(screen.getByRole('link', { name: /Tic-Tac-Toe/ })).toHaveAttribute(
      'href',
      preview.url,
    );
  });

  it('says why a room is here, so nobody reads it as a ranking', () => {
    render(<RoomPreviewsSection data={OVERVIEW.previews} />);
    expect(screen.getByText(OVERVIEW.previews.rooms[0].selected_reason)).toBeInTheDocument();
    expect(screen.getByText(OVERVIEW.previews.note)).toBeInTheDocument();
  });

  it('labels an excerpt with the API note and links back to the message', () => {
    render(<RoomPreviewsSection data={OVERVIEW.previews} />);
    const excerpted = OVERVIEW.previews.rooms[0].exchange[1];
    expect(screen.getByText(excerpted.excerpt_note!)).toBeInTheDocument();
    const links = screen
      .getAllByRole('link')
      .map((a) => a.getAttribute('href'));
    expect(links).toContain(excerpted.message_url);
  });

  it('renders the API empty note when no room is selected', () => {
    render(<RoomPreviewsSection data={{ ...OVERVIEW.previews, rooms: [] }} />);
    expect(screen.getByText(OVERVIEW.previews.empty_note)).toBeInTheDocument();
  });

  it('degrades cleanly when a selected room has no purpose, participants or exchange', () => {
    const bare = {
      ...OVERVIEW.previews,
      rooms: [
        {
          ...OVERVIEW.previews.rooms[0],
          purpose: '',
          participants: [],
          exchange: [],
        },
      ],
    };
    render(<RoomPreviewsSection data={bare} />);

    // The room is still named, still linked, and still says why it is here.
    expect(screen.getByText(bare.rooms[0].display_name)).toBeInTheDocument();
    expect(screen.getByText(bare.rooms[0].selected_reason)).toBeInTheDocument();
    expect(screen.getByText(bare.rooms[0].message_count_label)).toBeInTheDocument();
    // And nothing is invented to fill the gaps.
    expect(screen.queryByText(OVERVIEW.previews.rooms[0].purpose)).not.toBeInTheDocument();
  });

  it('omits the message link when the API sent no anchor for it', () => {
    const anchorless = {
      ...OVERVIEW.previews,
      rooms: [
        {
          ...OVERVIEW.previews.rooms[0],
          exchange: [
            {
              author: 'planner',
              author_role: 'agent',
              excerpt: 'no anchor for this one',
              is_excerpt: false,
            },
          ],
        },
      ],
    };
    render(<RoomPreviewsSection data={anchorless} />);
    expect(screen.getByText('no anchor for this one')).toBeInTheDocument();
    expect(
      screen.queryByRole('link', { name: /Open the original message/ }),
    ).not.toBeInTheDocument();
  });
});

describe('ApiUsageSection', () => {
  it('renders the measured call volumes with their windows', () => {
    render(<ApiUsageSection data={OVERVIEW.api_usage} />);
    for (const metric of OVERVIEW.api_usage.metrics) {
      expect(screen.getByText(metric.label)).toBeInTheDocument();
      expect(screen.getByText(metric.display)).toBeInTheDocument();
    }
    expect(screen.getAllByTestId('overview-metric')).toHaveLength(
      OVERVIEW.api_usage.metrics.length,
    );
  });

  it('lists the endpoints the API named, method and path verbatim', () => {
    render(<ApiUsageSection data={OVERVIEW.api_usage} />);
    const rows = screen.getAllByTestId('overview-endpoint');
    expect(rows).toHaveLength(OVERVIEW.api_usage.endpoints.length);
    rows.forEach((row, i) => {
      const endpoint = OVERVIEW.api_usage.endpoints[i];
      expect(squish(row.textContent)).toContain(endpoint.method);
      expect(squish(row.textContent)).toContain(endpoint.path);
      expect(squish(row.textContent)).toContain(endpoint.summary);
    });
  });

  it('links to the documentation the API pointed at', () => {
    render(<ApiUsageSection data={OVERVIEW.api_usage} />);
    expect(
      screen.getByRole('link', { name: OVERVIEW.api_usage.docs_label }),
    ).toHaveAttribute('href', '/api-docs');
  });
});

describe('SearchStatsSection', () => {
  it('renders the API-formatted rate rather than computing one', () => {
    render(<SearchStatsSection data={OVERVIEW.search} />);
    expect(screen.getByText('49%')).toBeInTheDocument();
  });

  it('renders all three compact tables with their windows and definitions', () => {
    render(<SearchStatsSection data={OVERVIEW.search} />);
    for (const table of [
      OVERVIEW.search.trending,
      OVERVIEW.search.recent,
      OVERVIEW.search.categories,
    ]) {
      const node = screen.getByTestId(`overview-table-${table.heading}`);
      expect(squish(node.textContent)).toContain(table.heading);
      expect(squish(node.textContent)).toContain(table.window);
      expect(within(node).getByTitle(table.definition)).toBeInTheDocument();
      for (const row of table.rows) {
        expect(within(node).getByText(row.label)).toBeInTheDocument();
        expect(within(node).getByText(row.count_label)).toBeInTheDocument();
      }
    }
  });

  it('shows the API time label on a recent query', () => {
    render(<SearchStatsSection data={OVERVIEW.search} />);
    expect(screen.getByText('2 minutes ago')).toBeInTheDocument();
  });

  it('falls back to the API empty note for an empty table', () => {
    const empty = {
      ...OVERVIEW.search,
      trending: { ...OVERVIEW.search.trending, rows: [] },
    };
    render(<SearchStatsSection data={empty} />);
    expect(screen.getByText(OVERVIEW.search.trending.empty_note)).toBeInTheDocument();
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
    render(<ReusablePostsSection data={OVERVIEW.posts} />);
    const item = OVERVIEW.posts.items[0];
    const link = screen.getByRole('link', { name: new RegExp(item.title) });
    expect(link).toHaveAttribute('href', item.url);
    expect(screen.getByText(item.contribution_label)).toBeInTheDocument();
    expect(screen.getByText(item.last_activity_label)).toBeInTheDocument();
    for (const tag of item.tags) {
      expect(screen.getByText(tag)).toBeInTheDocument();
    }
  });

  it('states what makes a post reusable', () => {
    render(<ReusablePostsSection data={OVERVIEW.posts} />);
    expect(screen.getByText(OVERVIEW.posts.definition)).toBeInTheDocument();
  });

  it('links on to all posts and shows the empty note when there are none', () => {
    render(<ReusablePostsSection data={OVERVIEW.posts} />);
    expect(
      screen.getByRole('link', { name: OVERVIEW.posts.browse_label }),
    ).toHaveAttribute('href', '/posts');

    render(<ReusablePostsSection data={{ ...OVERVIEW.posts, items: [] }} />);
    expect(screen.getByText(OVERVIEW.posts.empty_note)).toBeInTheDocument();
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

  it.each(SECTION_FILES)('%s keeps the shared section rhythm', (file) => {
    expect(read(file)).toContain('px-4 sm:px-6 lg:px-12 py-24 lg:py-32');
  });
});
