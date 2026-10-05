import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, within } from '@testing-library/react';
import { renderToStaticMarkup } from 'react-dom/server';
import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import GuidesPage from './page';
import { GUIDE_GROUPS, WORKFLOW_GUIDES } from '@/lib/docs/workflow-guides';
import { CONNECT_EXAMPLES } from '@/components/connect/connect-fixture';

// /docs/guides (SPEC.md 27.5): the same sentence three ways — the API's example sentences
// stacked and aligned, so the words that change read at a glance — then every guide, in
// two groups (by use case, by agent), one line each, in plain server HTML. No room steps,
// no curl, no endpoints on the page.

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

  it('lists all nine guides in two groups, by use case and by agent, one line each', async () => {
    const { getAllByTestId, getByRole } = render(await GuidesPage());
    for (const group of GUIDE_GROUPS) {
      const section = getByRole('region', { name: group.heading });
      const entries = within(section).getAllByTestId('guide-entry');
      expect(entries.map((e) => squish(within(e).getByRole('link').textContent))).toEqual(group.guides.map((g) => g.title));
      expect(entries.map((e) => within(e).getByRole('link').getAttribute('href'))).toEqual(
        group.guides.map((g) => `/docs/guides/${g.slug}`),
      );
      expect(entries.map((e) => squish(e.querySelector('p')?.textContent ?? ''))).toEqual(group.guides.map((g) => g.description));
    }
    expect(getAllByTestId('guide-entry')).toHaveLength(9);
  });

  it('lists the resume guide with the others', async () => {
    const { container } = render(await GuidesPage());
    expect(hrefs(container)).toContain('/docs/guides/resume-across-two-clis');
  });

  it('puts every guide and its line in the HTML the server sends', async () => {
    const html = renderToStaticMarkup(await GuidesPage());
    for (const guide of WORKFLOW_GUIDES) {
      expect(html).toMatch(new RegExp(`<a\\b[^>]*href="/docs/guides/${guide.slug}"`));
      expect(html).toContain(guide.title);
    }
  });

  it('shows no endpoint, no curl and no room steps', async () => {
    const { container } = render(await GuidesPage());
    const text = container.textContent ?? '';
    for (const forbidden of ['api.solvr.dev', '/v1/', 'curl', 'Handshake', 'HOW A ROOM WORKS']) {
      expect(text).not.toContain(forbidden);
    }
    expect(container.querySelector('pre')).toBeNull();
  });

  it('keeps its list when the sentences cannot be read', async () => {
    fetchMock.mockResolvedValue({ ok: false, status: 503, json: async () => ({}) });
    const { queryByTestId, getAllByTestId } = render(await GuidesPage());
    expect(queryByTestId('prompt-stack')).toBeNull();
    expect(getAllByTestId('guide-entry')).toHaveLength(9);
  });

  it('links where to go next: /connect, the skill, llms.txt and the API docs', async () => {
    const { container } = render(await GuidesPage());
    for (const href of ['/connect', '/skill.md', '/llms.txt', '/api-docs']) {
      expect(hrefs(container)).toContain(href);
    }
  });

  it('drops the carousel and the search-first pattern', async () => {
    const { container } = render(await GuidesPage());
    const text = container.textContent ?? '';
    expect(text).not.toContain('Search Before You Solve');
    expect(container.querySelector('.animate-carousel')).toBeNull();
  });

  it('is a server-rendered page: no client-only code', () => {
    const source = readFileSync(join(process.cwd(), 'app/docs/guides/page.tsx'), 'utf8');
    expect(source).not.toMatch(/^["']use client["']/m);
  });
});
