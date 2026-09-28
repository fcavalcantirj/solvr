import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import { readFileSync, readdirSync, statSync } from 'node:fs';
import { join, relative, sep } from 'node:path';

// The GA tag reports the page address with every hit. A page whose address
// carries a secret (a claim token, a one-time login code, an unsubscribe token)
// must never load it, or the secret lands in the analytics property.

const mockPathname = vi.fn<() => string | null>();
vi.mock('next/navigation', () => ({
  usePathname: () => mockPathname(),
}));

vi.mock('@next/third-parties/google', () => ({
  GoogleAnalytics: ({ gaId }: { gaId: string }) => (
    <div data-testid="ga-tag" data-ga-id={gaId} />
  ),
}));

import { SiteAnalytics, isUntrackedPath } from './site-analytics';

describe('SiteAnalytics', () => {
  beforeEach(() => {
    mockPathname.mockReset();
  });

  it.each(['/', '/rooms/planner-room', '/feed', '/claimed', '/auth', '/email'])(
    'loads the GA tag on %s',
    (path) => {
      mockPathname.mockReturnValue(path);
      render(<SiteAnalytics gaId="G-TEST" />);
      expect(screen.getByTestId('ga-tag').getAttribute('data-ga-id')).toBe('G-TEST');
    },
  );

  it.each(['/claim', '/claim/', '/auth/callback', '/email/unsubscribe'])(
    'does not load the GA tag on %s, whose address carries a secret',
    (path) => {
      mockPathname.mockReturnValue(path);
      render(<SiteAnalytics gaId="G-TEST" />);
      expect(screen.queryByTestId('ga-tag')).toBeNull();
    },
  );

  it('does not load the GA tag when the page is unknown', () => {
    mockPathname.mockReturnValue(null);
    render(<SiteAnalytics gaId="G-TEST" />);
    expect(screen.queryByTestId('ga-tag')).toBeNull();
  });

  it('loads the GA tag once the visitor moves on from a claim link', () => {
    mockPathname.mockReturnValue('/claim');
    const { rerender } = render(<SiteAnalytics gaId="G-TEST" />);
    expect(screen.queryByTestId('ga-tag')).toBeNull();

    mockPathname.mockReturnValue('/dashboard');
    rerender(<SiteAnalytics gaId="G-TEST" />);
    expect(screen.getByTestId('ga-tag')).toBeTruthy();
  });
});

describe('root layout mounts GA only through SiteAnalytics', () => {
  it('does not render the GA component directly', () => {
    const layout = readFileSync(join(__dirname, '..', 'app', 'layout.tsx'), 'utf8');
    expect(layout).not.toMatch(/GoogleAnalytics/);
    expect(layout).toMatch(/<SiteAnalytics\b/);
  });
});

// Every page that reads a credential out of its own address must be untracked.
// A new page that starts reading ?token= fails here until it is listed.
describe('pages that read a secret from their address are untracked', () => {
  const SECRET_FROM_ADDRESS =
    /searchParams\??\.get\(\s*["'](token|code|key|secret|api_key|access_token)["']|location\.hash/;
  const appDir = join(__dirname, '..', 'app');

  const pages: string[] = [];
  const walk = (dir: string) => {
    for (const entry of readdirSync(dir)) {
      const full = join(dir, entry);
      if (statSync(full).isDirectory()) {
        walk(full);
      } else if (entry === 'page.tsx' || entry === 'page.ts') {
        pages.push(full);
      }
    }
  };
  walk(appDir);

  const routeOf = (file: string) =>
    '/' +
    relative(appDir, file)
      .split(sep)
      .slice(0, -1)
      .filter((segment) => !/^\(.*\)$/.test(segment))
      .join('/');

  const readers = pages.filter((file) => SECRET_FROM_ADDRESS.test(readFileSync(file, 'utf8')));

  it('finds the pages known to read one', () => {
    expect(readers.map(routeOf).sort()).toEqual(
      expect.arrayContaining(['/auth/callback', '/claim', '/email/unsubscribe']),
    );
  });

  it.each(readers.map((file) => [routeOf(file)]))('%s is untracked', (route) => {
    expect(isUntrackedPath(route)).toBe(true);
  });
});
