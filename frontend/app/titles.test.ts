import { describe, it, expect, vi } from 'vitest';

// Task idx 82: the root template appends " | Solvr" to every page title, so no page
// names the brand itself (no "X - Solvr | Solvr"). The home page carries the concise
// agent-connection title in full, the same proposition as the site defaults.

vi.mock('react', async (importOriginal) => ({
  ...(await importOriginal<typeof import('react')>()),
  cache: <T,>(fn: T) => fn,
}));
vi.mock('@/components/header', () => ({ Header: () => null }));
vi.mock('@/components/footer', () => ({ Footer: () => null }));
vi.mock('@/components/hero-section', () => ({ HeroSection: () => null }));
vi.mock('@/components/homepage/live-overview', () => ({ LiveOverview: () => null }));
vi.mock('@/components/connect/connect-panel', () => ({ ConnectPanel: () => null }));
vi.mock('@/components/connect/direct-create-panel', () => ({ DirectCreatePanel: () => null }));
vi.mock('@/components/rooms/rooms-browser', () => ({ RoomsBrowser: () => null }));
vi.mock('@/components/rooms/recently-viewed-rooms', () => ({ RecentlyViewedRooms: () => null }));
vi.mock('@/components/rooms/create-room-dialog', () => ({ CreateRoomDialog: () => null }));
vi.mock('next/font/google', () => ({ Inter: () => ({}), JetBrains_Mono: () => ({}) }));
vi.mock('@next/third-parties/google', () => ({ GoogleAnalytics: () => null }));

import { metadata as layout } from './layout';
import { metadata as home } from './page';
import { metadata as connect } from './connect/page';
import { metadata as protocol } from './docs/protocol/page';
import { metadata as data } from './data/layout';
import { generateMetadata as rooms } from './rooms/page';

const text = (t: unknown) =>
  typeof t === 'string' ? t : ((t as { absolute?: string; default?: string })?.absolute ?? '');

describe('page titles', () => {
  it('the home page carries the agent-connection title and description in full', () => {
    const defaults = layout.title as { default: string };
    expect(home.title).toEqual({ absolute: defaults.default });
    expect(home.description).toBe(layout.description);
  });

  it('no other page names the brand the template already appends', async () => {
    const titles = {
      connect: connect.title,
      protocol: protocol.title,
      data: data.title,
      dataOpenGraph: data.openGraph?.title,
      rooms: (await rooms({ searchParams: Promise.resolve({}) })).title,
    };
    for (const [page, title] of Object.entries(titles)) {
      expect(text(title), page).not.toMatch(/solvr/i);
      expect(text(title).length, page).toBeGreaterThan(3);
    }
  });
});
