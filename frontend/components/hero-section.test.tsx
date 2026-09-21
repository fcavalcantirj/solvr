import { render, screen, within } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import { HeroSection } from './hero-section';

// The hero owns the product proposition. Every string it publishes is pinned
// here so a copy edit is a deliberate act, not a drift.
const HEADLINE = 'Connect your agents. Let them work together.';
const SUPPORTING =
  'Two agents or a whole team. Paste a prompt into each. They share a Solvr room to plan, build, and review. No human signup or installation needed.';
const WORKFLOW = [
  '1. Give your planner a prompt.',
  '2. Paste its invite into your executor.',
  '3. Watch them work.',
];
const EXAMPLE_ROOM = '/rooms/tictactoe-human-vs-computer-20260920';

const mockUseStats = vi.fn();
vi.mock('@/hooks/use-stats', () => ({
  useStats: () => mockUseStats(),
}));

// The hero must not be gated on the auth state: task 2 shipped a header whose
// primary action was invisible until useAuth resolved. Mocked here so that a
// regression re-introducing the dependency is caught rather than crashing.
const mockUseAuth = vi.fn();
vi.mock('@/hooks/use-auth', () => ({
  useAuth: () => mockUseAuth(),
}));

const STATS = {
  active_posts: 400,
  total_agents: 1200,
  solved_today: 3,
  posted_today: 9,
  problems_solved: 301,
  questions_answered: 10,
  humans_count: 1050,
  total_posts: 2098,
  total_contributions: 3327,
  crystallized_posts: 50,
};

const read = (file: string) => readFileSync(join(process.cwd(), file), 'utf8');
const squish = (s: string | null) => (s ?? '').replace(/\s+/g, ' ').trim();

beforeEach(() => {
  vi.clearAllMocks();
  mockUseStats.mockReturnValue({ stats: STATS, loading: false });
  mockUseAuth.mockReturnValue({ isAuthenticated: false, isLoading: true, user: null });
});

describe('HeroSection proposition', () => {
  it('states the agent-connection proposition as the headline', () => {
    render(<HeroSection />);
    const heading = screen.getByRole('heading', { level: 1 });
    expect(squish(heading.textContent)).toBe(HEADLINE);
  });

  it('explains the promise in the supporting copy', () => {
    render(<HeroSection />);
    expect(screen.getByText(SUPPORTING)).toBeInTheDocument();
  });

  it('drops the old collective-knowledge hero copy', () => {
    const { container } = render(<HeroSection />);
    const text = squish(container.textContent);
    expect(text).not.toContain('Several brains');
    expect(text).not.toContain('COLLECTIVE INTELLIGENCE');
    expect(text).not.toContain('same knowledge base');
    expect(text).not.toContain('ALL SYSTEMS OPERATIONAL');
  });
});

describe('HeroSection calls to action', () => {
  it('makes Connect agents now the primary, filled action', () => {
    render(<HeroSection />);
    const cta = screen.getByRole('link', { name: 'Connect agents now' });
    expect(cta).toHaveAttribute('href', '/connect');
    expect(cta.className).toContain('bg-foreground');
    expect(cta.className).toContain('text-background');
  });

  it('makes Watch an example the secondary action on the public example room', () => {
    render(<HeroSection />);
    const cta = screen.getByRole('link', { name: 'Watch an example' });
    expect(cta).toHaveAttribute('href', EXAMPLE_ROOM);
    // Quiet outline, not a second filled button competing with Connect.
    expect(cta.className).toContain('border');
    // hover:bg-foreground is fine; an unconditional fill is not.
    expect(cta.className).not.toMatch(/(^|\s)bg-foreground(\s|$)/);
  });

  it('replaces the registration-first CTA pair', () => {
    const { container } = render(<HeroSection />);
    const hrefs = Array.from(container.querySelectorAll('a')).map((a) => a.getAttribute('href'));
    expect(hrefs).not.toContain('/join');
    expect(hrefs).not.toContain('/connect/agent');
    expect(hrefs).not.toContain('/new?type=problem');
    expect(hrefs).not.toContain('/feed');
    const text = squish(container.textContent);
    expect(text).not.toContain('JOIN AS HUMAN');
    expect(text).not.toContain('CONNECT AI AGENT');
    expect(text).not.toContain('POST A PROBLEM');
  });

  it('shows the same two actions whatever the auth state does', () => {
    for (const auth of [
      { isAuthenticated: false, isLoading: true, user: null },
      { isAuthenticated: false, isLoading: false, user: null },
      { isAuthenticated: true, isLoading: false, user: { id: '1', type: 'human' } },
    ]) {
      mockUseAuth.mockReturnValue(auth);
      const { container, unmount } = render(<HeroSection />);
      const names = Array.from(container.querySelectorAll('a')).map((a) => squish(a.textContent));
      expect(names).toEqual(['Connect agents now', 'Watch an example']);
      unmount();
    }
  });

  it('does not read the auth state at all', () => {
    expect(read('components/hero-section.tsx')).not.toContain('use-auth');
  });
});

describe('HeroSection workflow', () => {
  it('lists the three steps in order, beside the connection control', () => {
    render(<HeroSection />);
    const list = screen.getByRole('list', { name: /how it starts/i });
    const items = within(list)
      .getAllByRole('listitem')
      .map((li) => squish(li.textContent));
    expect(items).toEqual(WORKFLOW);
  });

  it('puts the workflow after the connection control and the numbers after both', () => {
    const { container } = render(<HeroSection />);
    const cta = screen.getByRole('link', { name: 'Connect agents now' });
    const list = screen.getByRole('list', { name: /how it starts/i });
    const stats = container.querySelector('[data-testid="hero-stats"]');
    expect(stats).not.toBeNull();
    // DOCUMENT_POSITION_FOLLOWING = 4
    expect(cta.compareDocumentPosition(list) & 4).toBeTruthy();
    expect(list.compareDocumentPosition(stats as Node) & 4).toBeTruthy();
  });
});

describe('HeroSection activity numbers', () => {
  it('renders real numbers straight from the API', () => {
    render(<HeroSection />);
    const stats = screen.getByTestId('hero-stats');
    expect(within(stats).getByText('301')).toBeInTheDocument();
    expect(within(stats).getByText('3.3K')).toBeInTheDocument();
    expect(within(stats).getByText('1.2K')).toBeInTheDocument();
    expect(within(stats).getByText('1.1K')).toBeInTheDocument();
  });

  it('shows a placeholder instead of a zero while the API answers', () => {
    mockUseStats.mockReturnValue({ stats: null, loading: true });
    render(<HeroSection />);
    const stats = screen.getByTestId('hero-stats');
    expect(within(stats).getAllByText('--')).toHaveLength(4);
    expect(within(stats).queryByText('0')).not.toBeInTheDocument();
  });

  it('shows a placeholder rather than a zero when the stats call failed', () => {
    mockUseStats.mockReturnValue({ stats: null, loading: false, error: 'boom' });
    render(<HeroSection />);
    const stats = screen.getByTestId('hero-stats');
    expect(within(stats).getAllByText('--')).toHaveLength(4);
    expect(within(stats).queryByText('0')).not.toBeInTheDocument();
  });

  it('never invents a number the API did not send', () => {
    const { rerender } = render(<HeroSection />);
    const first = squish(screen.getByTestId('hero-stats').textContent);
    mockUseStats.mockReturnValue({
      stats: { ...STATS, problems_solved: 7, total_contributions: 8, total_agents: 9, humans_count: 11 },
      loading: false,
    });
    rerender(<HeroSection />);
    const second = squish(screen.getByTestId('hero-stats').textContent);
    expect(second).not.toBe(first);
    expect(second).toContain('7');
    expect(second).toContain('11');
    expect(second).not.toContain('301');
  });
});

describe('HeroSection is compact', () => {
  it('no longer reserves a whole viewport for itself', () => {
    const source = read('components/hero-section.tsx');
    expect(source).not.toContain('min-h-screen');
    // Preserved from the design system: shared gutters, clears the fixed header.
    expect(source).toContain('px-4 sm:px-6 lg:px-12');
    expect(source).toContain('pt-24');
  });
});

describe('homepage speed claims', () => {
  // Speed is the design objective, not an advertised number. A comparative or
  // numeric setup-time claim may only ship once a benchmark backs it.
  const HOMEPAGE = [
    'app/page.tsx',
    'components/hero-section.tsx',
    'components/how-it-works.tsx',
    'components/features-section.tsx',
    'components/collaboration-showcase.tsx',
    'components/api-section.tsx',
    'components/cta-section.tsx',
  ];
  const UNSUPPORTED =
    /fastest|quickest|\bbest\b|faster than|world'?s |#1\b|\bin under\b|\b\d+ ?(?:x|seconds|secs|minutes|mins)\b/i;

  it.each(HOMEPAGE)('%s publishes no unbenchmarked speed claim', (file) => {
    const offending = read(file)
      .split('\n')
      .map((line, i) => [i + 1, line] as const)
      .filter(([, line]) => UNSUPPORTED.test(line));
    expect(offending.map(([n, l]) => `${n}: ${l.trim()}`)).toEqual([]);
  });
});
