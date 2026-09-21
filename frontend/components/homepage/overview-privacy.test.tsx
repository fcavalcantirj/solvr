import { render } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { readFileSync, readdirSync } from 'node:fs';
import { join } from 'node:path';

import { OVERVIEW } from './overview-fixture';
import { LiveOverview } from './live-overview';
import { HeroSection } from '@/components/hero-section';
import { metadata } from '@/app/page';

// Solvr publishes product activity and keeps website traffic private.
//
// The API decides that — the allowlist and the excluded vocabulary live in
// backend/internal/api/handlers/public_overview_allowlist.go, and the API
// withholds anything that is not on it. These tests hold the other half of the
// line: whatever the page renders, the HTML a visitor receives carries no
// audience or growth idea either, and no internal planning asset is served
// from the site.
//
// The vocabulary below mirrors the Go list. It is duplicated on purpose: a
// frontend test that imported the rule from the thing it is checking would
// prove nothing.
const EXCLUDED = [
  /\bvisitors?\b/i,
  /\bvisits?\b/i,
  /\bsessions?\b/i,
  /\bpage[ _-]?views?\b/i,
  /\b(dau|wau|mau)\b/i,
  /\b(daily|weekly|monthly) active (users|people|accounts|audience)\b/i,
  /\bbounce\b/i,
  /\bacquisitions?\b/i,
  /\breferrers?\b/i,
  /\butm[ _-][a-z]+\b/i,
  /\bgeograph\w*\b/i,
  /\bconversions?\b/i,
  /\bfunnels?\b/i,
  /\bretention\b/i,
  /\bcampaigns?\b/i,
  /\bgrowth (target|targets|goal|goals)\b/i,
  /\bimpressions?\b/i,
  /\btraffic\b/i,
  /\baudience\b/i,
];

// A served file is prose written for agents, not a statistics payload, so it
// is checked for PUBLISHED AUDIENCE FIGURES rather than for single words: an
// instruction that mentions a work session is not a traffic metric, while
// "12,004 visitors" or a bounce rate is.
const EXCLUDED_IN_ASSETS = [
  /\d[\d,.]*\s*(unique\s+)?(visitors|visits|sessions|page[ _-]?views|impressions)\b/i,
  /\b(bounce|conversion|retention|click[ _-]?through)\s+rate\b/i,
  /\b(dau|wau|mau)\b/i,
  /\b(daily|weekly|monthly) active (users|people|accounts|audience)\b/i,
  /\butm[ _-][a-z]+\b/i,
  /\bgrowth (target|targets|goal|goals)\b/i,
  /\b(website|site|web) traffic\b/i,
  /\bacquisition (channel|channels|source|sources)\b/i,
];

const mockUseOverview = vi.fn();
vi.mock('@/hooks/use-homepage-overview', () => ({
  useHomepageOverview: () => mockUseOverview(),
}));

vi.mock('@/components/collaboration-example', () => ({
  CollaborationExample: () => <section data-testid="collab-example" />,
}));

vi.mock('@/lib/api', () => ({
  api: {
    getHomepageOverview: vi.fn(),
    getHomepageRooms: vi.fn(),
    getHomepageSearch: vi.fn(),
  },
}));

const expectNoAudienceVocabulary = (what: string, text: string) => {
  for (const pattern of EXCLUDED) {
    expect(pattern.test(text), `${what} publishes ${pattern}`).toBe(false);
  }
};

beforeEach(() => {
  vi.clearAllMocks();
  mockUseOverview.mockReturnValue({ overview: OVERVIEW, loading: false, error: null });
});

describe('the public homepage publishes product activity, never website traffic', () => {
  it('renders the whole live overview without an audience or growth figure', () => {
    const { container } = render(<LiveOverview />);
    expectNoAudienceVocabulary('the rendered overview', container.innerHTML);
  });

  it('states the proposition without claiming an audience', () => {
    const { container } = render(<HeroSection />);
    expectNoAudienceVocabulary('the hero', container.innerHTML);
  });

  it('keeps page metadata free of audience claims', () => {
    expectNoAudienceVocabulary(
      'the homepage metadata',
      `${metadata.title} ${metadata.description}`,
    );
  });

  it('labels the registration totals as product accounts, not active users', () => {
    const { container } = render(<LiveOverview />);
    const section = container.querySelector('[data-testid="overview-section-community"]');
    expect(section).not.toBeNull();

    const text = section!.textContent ?? '';
    expect(text).toContain('REGISTERED AGENTS');
    expect(text).toContain('REGISTERED HUMANS');
    expect(text).toContain('all time');
    expectNoAudienceVocabulary('the all-time section', text);
    expect(/\bactive (users|humans|agents)\b/i.test(text)).toBe(false);
  });
});

describe('the site serves no internal planning asset', () => {
  it('keeps the build ledger, the journal and the planning reports out of /public', () => {
    const served = readdirSync(join(process.cwd(), 'public'));

    for (const file of served) {
      expect(
        /report|spec|prd|progress|analytics|ledger/i.test(file),
        `public/${file} looks like an internal planning asset`,
      ).toBe(false);
    }
  });

  it('keeps audience figures out of the files it does serve', () => {
    const dir = join(process.cwd(), 'public');
    const readable = readdirSync(dir, { recursive: true, withFileTypes: true }).filter(
      (entry) => entry.isFile() && /\.(md|json|txt|sh)$/.test(entry.name),
    );
    expect(readable.length).toBeGreaterThan(0);

    for (const entry of readable) {
      const contents = readFileSync(join(entry.parentPath ?? dir, entry.name), 'utf8');
      for (const pattern of EXCLUDED_IN_ASSETS) {
        expect(
          pattern.test(contents),
          `public/${entry.name} publishes an audience figure matching ${pattern}`,
        ).toBe(false);
      }
    }
  });
});
