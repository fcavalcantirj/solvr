import { render, screen } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { readFileSync } from 'node:fs';
import { join } from 'node:path';

import { OVERVIEW } from './overview-fixture';
import { LiveOverview } from './live-overview';

// LiveOverview is the one component that fetches the overview and lays the
// whole index out. The order it renders in IS the specification of the page.

const mockUseOverview = vi.fn();
vi.mock('@/hooks/use-homepage-overview', () => ({
  useHomepageOverview: () => mockUseOverview(),
}));

vi.mock('@/components/collaboration-example', () => ({
  CollaborationExample: () => <section data-testid="collab-example" />,
}));

const read = (file: string) => readFileSync(join(process.cwd(), file), 'utf8');

beforeEach(() => {
  vi.clearAllMocks();
  mockUseOverview.mockReturnValue({ overview: OVERVIEW, loading: false, error: null });
});

describe('LiveOverview layout', () => {
  it('lays the index out in the specified order', () => {
    render(<LiveOverview />);
    const order = screen
      .getAllByTestId(/^overview-section-|^collab-example$/)
      .map((node) => node.getAttribute('data-testid'));

    expect(order).toEqual([
      'overview-section-rooms',
      'overview-section-activity',
      'overview-section-previews',
      'overview-section-api',
      'overview-section-search',
      'overview-section-community',
      'collab-example',
      'overview-section-posts',
      'overview-section-closing',
    ]);
  });

  it('ends on the connection control', () => {
    render(<LiveOverview />);
    const sections = screen.getAllByTestId(/^overview-section-/);
    expect(sections[sections.length - 1]).toHaveAttribute(
      'data-testid',
      'overview-section-closing',
    );
    expect(
      screen.getByRole('link', { name: OVERVIEW.closing.connect_label }),
    ).toBeInTheDocument();
  });

  it('carries the public statistics itself, so the Data page is not a detour', () => {
    render(<LiveOverview />);
    // The three tables /data exists to show: trending terms, recent terms and
    // the type-filter breakdown, plus the agent/human search split.
    expect(screen.getByTestId('overview-table-TRENDING QUERIES')).toBeInTheDocument();
    expect(screen.getByTestId('overview-table-RECENT QUERIES')).toBeInTheDocument();
    expect(
      screen.getByTestId('overview-table-SEARCHES BY TYPE FILTER'),
    ).toBeInTheDocument();
  });

  it('shows a loading state instead of empty numbers while the API answers', () => {
    mockUseOverview.mockReturnValue({ overview: null, loading: true, error: null });
    render(<LiveOverview />);
    expect(screen.getByTestId('overview-loading')).toBeInTheDocument();
    expect(screen.queryByTestId('overview-section-rooms')).not.toBeInTheDocument();
  });

  it('reports a failed read and still offers the connection control', () => {
    mockUseOverview.mockReturnValue({ overview: null, loading: false, error: 'boom' });
    render(<LiveOverview />);
    expect(screen.getByRole('alert')).toHaveTextContent('boom');
    expect(screen.getByRole('link', { name: /connect agents/i })).toHaveAttribute(
      'href',
      '/connect',
    );
  });

  it('never invents a number of its own', () => {
    const source = read('components/homepage/live-overview.tsx');
    expect(source).not.toMatch(/(?<![\w-])\d{3,}(?![\w%-])/);
    expect(source).not.toMatch(/\.(toFixed|sort)\(/);
  });
});
