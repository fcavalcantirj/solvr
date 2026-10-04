import { render, screen, within, fireEvent } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import { HeroSection } from './hero-section';
import { OVERVIEW } from './homepage/overview-fixture';

// The hero owns the product proposition. Every string it publishes is pinned
// here so a copy edit is a deliberate act, not a drift.
const HEADLINE = 'Connect your agents. Let them work together.';
const SUPPORTING =
  'Two agents or a whole team. Paste a prompt into each. They share a Solvr room to plan, build, and review. No human signup or installation needed.';
// The hero's right column: the numbers GET /v1/overview chose, rendered as sent.
const HERO_NUMBERS = OVERVIEW.hero_numbers;

// The hero must not be gated on the auth state: task 2 shipped a header whose
// primary action was invisible until useAuth resolved. Mocked here so that a
// regression re-introducing the dependency is caught rather than crashing.
const mockUseAuth = vi.fn();
vi.mock('@/hooks/use-auth', () => ({
  useAuth: () => mockUseAuth(),
}));

// The hero opens the SHARED connection panel — the same component /connect
// renders full-page. Its contents are proven in its own test; here only the
// opening matters.
vi.mock('@/components/connect/connect-panel', () => ({
  ConnectPanel: ({ variant }: { variant?: string }) => (
    <div data-testid="connect-panel" data-variant={variant} />
  ),
}));

const read = (file: string) => readFileSync(join(process.cwd(), file), 'utf8');
const squish = (s: string | null) => (s ?? '').replace(/\s+/g, ' ').trim();

beforeEach(() => {
  vi.clearAllMocks();
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
    const cta = screen.getByRole('button', { name: /Connect agents now/i });
    expect(cta.className).toContain('bg-foreground');
    expect(cta.className).toContain('text-background');
  });

  it('opens the connection panel on the index itself, without leaving the page', () => {
    render(<HeroSection />);
    expect(screen.queryByTestId('connect-panel')).not.toBeInTheDocument();

    const cta = screen.getByRole('button', { name: /Connect agents now/i });
    expect(cta).toHaveAttribute('aria-expanded', 'false');
    fireEvent.click(cta);

    const panel = screen.getByTestId('connect-panel');
    expect(panel).toHaveAttribute('data-variant', 'panel');
    expect(screen.getByRole('button', { name: /Connect agents now/i })).toHaveAttribute(
      'aria-expanded',
      'true',
    );
  });

  it('keeps the panel inline under the proposition rather than covering the page', () => {
    render(<HeroSection />);
    fireEvent.click(screen.getByRole('button', { name: /Connect agents now/i }));
    const panel = screen.getByTestId('connect-panel');
    // An overlay would take the page away from a visitor reading the index.
    expect(panel.closest('[role="dialog"]')).toBeNull();
    expect(panel.closest('.fixed')).toBeNull();
    const heading = screen.getByRole('heading', { level: 1 });
    // DOCUMENT_POSITION_FOLLOWING = 4
    expect(heading.compareDocumentPosition(panel) & 4).toBeTruthy();
  });

  it('makes Watch an example the secondary action, jumping to the example on this page', () => {
    render(<HeroSection />);
    const cta = screen.getByRole('link', { name: 'Watch an example' });
    // The homepage example section (GET /v1/homepage/example) is the proof and
    // carries its own room link when a live room exists; a slug in the client
    // 404s the day that room goes.
    expect(cta).toHaveAttribute('href', '#example');
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
      const links = Array.from(container.querySelectorAll('a')).map((a) => squish(a.textContent));
      expect(links).toEqual(['Watch an example']);
      expect(screen.getByRole('button', { name: /Connect agents now/i })).toBeInTheDocument();
      unmount();
    }
  });

  it('names no room of its own', () => {
    const src = read('components/hero-section.tsx');
    expect(src).not.toContain('tictactoe');
    expect(src).not.toContain('/rooms/');
  });

  it('does not read the auth state at all', () => {
    expect(read('components/hero-section.tsx')).not.toContain('use-auth');
  });
});

describe('HeroSection numbers', () => {
  // Replaces 'lists the three steps in order, beside the connection control':
  // the "HOW IT STARTS" steps gave way to the API's hero numbers (v1.3.4).
  it('renders the hero numbers as the API sends them: display, label, window, in order', () => {
    render(<HeroSection heroNumbers={HERO_NUMBERS} />);
    const list = screen.getByRole('list', { name: /solvr in numbers/i });
    const items = within(list)
      .getAllByRole('listitem')
      .map((li) => squish(li.textContent));
    expect(items).toEqual(HERO_NUMBERS.map((n) => `${n.display} ${n.label} ${n.window}`));
  });

  // Replaces 'puts the workflow after the connection control'.
  it('puts the numbers after the connection control', () => {
    render(<HeroSection heroNumbers={HERO_NUMBERS} />);
    const cta = screen.getByRole('button', { name: /Connect agents now/i });
    const list = screen.getByRole('list', { name: /solvr in numbers/i });
    // DOCUMENT_POSITION_FOLLOWING = 4
    expect(cta.compareDocumentPosition(list) & 4).toBeTruthy();
  });

  it('no longer carries the "HOW IT STARTS" steps', () => {
    render(<HeroSection heroNumbers={HERO_NUMBERS} />);
    expect(screen.queryByText(/how it starts/i)).not.toBeInTheDocument();
    expect(screen.queryByText('3. Watch them work.')).not.toBeInTheDocument();
  });

  it('computes nothing: no formatting, sorting or filtering of its own', () => {
    const source = read('components/hero-section.tsx');
    expect(source).not.toMatch(/\.(toFixed|sort|filter|toLocaleString)\(/);
    expect(source).not.toContain('Intl.');
  });
});

describe('HeroSection no longer carries the four-counter strip', () => {
  // The strip showed four bare totals with no window and no definition. The
  // live overview below the hero replaces it with room, search and community
  // statistics that each state what they count and over what period.
  // Replaces 'renders no statistics strip at all': the hero now shows the
  // numbers the API chose (hero_numbers), and nothing of its own without them.
  it('renders no numbers of its own: without hero_numbers the column is absent', () => {
    const { container, unmount } = render(<HeroSection />);
    expect(container.querySelector('[data-testid="hero-stats"]')).toBeNull();
    expect(screen.queryByRole('list', { name: /solvr in numbers/i })).not.toBeInTheDocument();
    unmount();

    render(<HeroSection heroNumbers={[]} />);
    expect(screen.queryByRole('list', { name: /solvr in numbers/i })).not.toBeInTheDocument();
  });

  it('publishes none of the old counter labels', () => {
    render(<HeroSection />);
    for (const label of [
      'PROBLEMS SOLVED',
      'CONTRIBUTIONS',
      'AI AGENTS ACTIVE',
      'HUMANS PARTICIPATING',
    ]) {
      expect(screen.queryByText(label)).not.toBeInTheDocument();
    }
  });

  it('no longer reads the stats endpoint at all', () => {
    const source = read('components/hero-section.tsx');
    expect(source).not.toContain('use-stats');
    expect(source).not.toContain('formatCount');
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
    'components/collaboration-example.tsx',
    'components/homepage/live-overview.tsx',
    'components/homepage/home-overview.tsx',
    'components/homepage/use-cases-section.tsx',
    'lib/docs/use-cases.ts',
    'components/homepage/metric.tsx',
    'components/homepage/room-stats-section.tsx',
    'components/homepage/room-activity-section.tsx',
    'components/homepage/room-previews-section.tsx',
    'components/homepage/api-usage-section.tsx',
    'components/homepage/search-stats-section.tsx',
    'components/homepage/community-totals-section.tsx',
    'components/homepage/reusable-posts-section.tsx',
    'components/homepage/closing-section.tsx',
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

describe('HeroSection — the panel it opens comes into view', () => {
  // Measured on the build: the panel opened 816px down a 900px window (and
  // fully below the fold on a phone), so a click looked like nothing happened.
  const scrollIntoView = vi.fn();
  const original = Element.prototype.scrollIntoView;

  beforeEach(() => {
    scrollIntoView.mockClear();
    Element.prototype.scrollIntoView = scrollIntoView;
    window.matchMedia = vi.fn().mockReturnValue({ matches: false }) as unknown as typeof window.matchMedia;
  });

  afterEach(() => {
    Element.prototype.scrollIntoView = original;
  });

  it('scrolls the opened panel to the top of the window, clear of the fixed header', () => {
    render(<HeroSection />);
    fireEvent.click(screen.getByRole('button', { name: /Connect agents now/i }));
    const panel = document.getElementById('hero-connect-panel');
    expect(panel).not.toBeNull();
    expect(panel!.className).toContain('scroll-mt-24');
    expect(scrollIntoView).toHaveBeenCalledTimes(1);
    expect(scrollIntoView.mock.contexts[0]).toBe(panel);
    expect(scrollIntoView).toHaveBeenCalledWith({ block: 'start', behavior: 'smooth' });
  });

  it('moves focus to the panel without a second jump', () => {
    render(<HeroSection />);
    fireEvent.click(screen.getByRole('button', { name: /Connect agents now/i }));
    expect(document.activeElement).toBe(document.getElementById('hero-connect-panel'));
  });

  it('jumps without animation when the visitor asks for reduced motion', () => {
    window.matchMedia = vi.fn().mockReturnValue({ matches: true }) as unknown as typeof window.matchMedia;
    render(<HeroSection />);
    fireEvent.click(screen.getByRole('button', { name: /Connect agents now/i }));
    expect(scrollIntoView).toHaveBeenCalledWith({ block: 'start', behavior: 'auto' });
  });

  it('does not scroll when the panel closes', () => {
    render(<HeroSection />);
    const cta = screen.getByRole('button', { name: /Connect agents now/i });
    fireEvent.click(cta);
    fireEvent.click(cta);
    expect(scrollIntoView).toHaveBeenCalledTimes(1);
  });
});
