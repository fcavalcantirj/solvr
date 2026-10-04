import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, within } from '@testing-library/react';
import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import GuidesPage from './page';
import { LISTED_GUIDES } from '@/lib/docs/workflow-guides';
import { CONNECT_EXAMPLES } from '@/components/connect/connect-fixture';

// /docs/guides (v1.3.5): the same sentence three ways — the API's example sentences
// stacked and aligned, so the words that change read at a glance — then a card per use
// case and where to go next. No room steps, no curl, no endpoints on the page.

vi.mock('@/components/header', () => ({ Header: () => <div data-testid="header">Header</div> }));
vi.mock('@/components/footer', () => ({ Footer: () => <div data-testid="footer">Footer</div> }));
vi.mock('next/link', () => ({
  default: ({ children, href, ...props }: { children: React.ReactNode; href: string; [key: string]: unknown }) => (
    <a href={href} {...props}>{children}</a>
  ),
}));

const fetchMock = vi.fn();
beforeEach(() => {
  fetchMock.mockReset();
  fetchMock.mockResolvedValue({
    ok: true,
    status: 200,
    json: async () => ({ data: { instruction_version: '2.0', presets: CONNECT_EXAMPLES } }),
  });
  vi.stubGlobal('fetch', fetchMock);
});
afterEach(() => vi.unstubAllGlobals());

const squish = (s: string | null) => (s ?? '').replace(/\s+/g, ' ').trim();
const hrefs = (container: HTMLElement) => [...container.querySelectorAll('a')].map((a) => a.getAttribute('href'));

describe('GuidesPage', () => {
  it('says in one line that every guide is the same sentence', async () => {
    const { getByRole, getByTestId } = render(await GuidesPage());
    expect(getByRole('heading', { level: 1, name: 'Guides' })).toBeInTheDocument();
    expect(getByTestId('guides-intro')).toHaveTextContent(/same sentence/i);
  });

  it('stacks the three example sentences, one per use case, exactly as served', async () => {
    const { getByTestId } = render(await GuidesPage());
    const rows = within(getByTestId('prompt-stack')).getAllByRole('listitem');
    expect(rows.map((r) => r.querySelector('p')?.textContent)).toEqual(CONNECT_EXAMPLES.map((p) => p.label));
    expect(String(fetchMock.mock.calls[0][0])).toContain('/v1/connect/examples');
  });

  it('lists the three use-case guides as cards, in order, each linking its page; the resume guide is not listed', async () => {
    const { getAllByTestId, container } = render(await GuidesPage());
    const cards = getAllByTestId('guide-card');
    expect(cards.map((c) => squish(within(c).getByRole('heading', { level: 3 }).textContent))).toEqual(
      LISTED_GUIDES.map((g) => g.title),
    );
    expect(cards.map((c) => within(c).getByRole('link').getAttribute('href'))).toEqual(
      LISTED_GUIDES.map((g) => `/docs/guides/${g.slug}`),
    );
    expect(hrefs(container)).not.toContain('/docs/guides/resume-across-two-clis');
  });

  it('shows no endpoint, no curl and no room steps', async () => {
    const { container } = render(await GuidesPage());
    const text = container.textContent ?? '';
    for (const forbidden of ['api.solvr.dev', '/v1/', 'curl', 'Handshake', 'HOW A ROOM WORKS']) {
      expect(text).not.toContain(forbidden);
    }
    expect(container.querySelector('pre')).toBeNull();
  });

  it('keeps its cards when the sentences cannot be read', async () => {
    fetchMock.mockResolvedValue({ ok: false, status: 503, json: async () => ({}) });
    const { queryByTestId, getAllByTestId } = render(await GuidesPage());
    expect(queryByTestId('prompt-stack')).toBeNull();
    expect(getAllByTestId('guide-card')).toHaveLength(3);
  });

  it('links where to go next: /connect, the skill, llms.txt and the API docs', async () => {
    const { container } = render(await GuidesPage());
    for (const href of ['/connect', '/skill.md', '/llms.txt', '/api-docs']) {
      expect(hrefs(container)).toContain(href);
    }
  });

  it('drops the carousel, the OpenClaw section and the search-first pattern', async () => {
    const { container } = render(await GuidesPage());
    const text = container.textContent ?? '';
    expect(text).not.toMatch(/openclaw/i);
    expect(text).not.toContain('Search Before You Solve');
    expect(container.querySelector('.animate-carousel')).toBeNull();
  });

  it('is a server-rendered page: no client-only code', () => {
    const source = readFileSync(join(process.cwd(), 'app/docs/guides/page.tsx'), 'utf8');
    expect(source).not.toMatch(/^["']use client["']/m);
  });
});
