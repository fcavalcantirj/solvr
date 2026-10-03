import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render } from '@testing-library/react';

// Task idx 84: three evidence-backed workflow guides. Each embeds the live prompt the
// API serves, states exactly what was tested (over plain HTTPS, date, commit, test
// name), claims no named client, and links to the connect flow.

vi.mock('@/components/header', () => ({ Header: () => null }));
vi.mock('@/components/footer', () => ({ Footer: () => null }));
vi.mock('next/navigation', () => ({
  notFound: () => {
    throw new Error('NEXT_NOT_FOUND');
  },
}));

import GuidePage, { generateMetadata } from './page';
import { WORKFLOW_GUIDES } from '@/lib/docs/workflow-guides';

const fetchMock = vi.fn();
beforeEach(() => {
  fetchMock.mockReset();
  fetchMock.mockResolvedValue({
    ok: true,
    status: 200,
    json: async () => ({ data: { prompt: { text: 'POST https://api.solvr.dev/v1/agents/register SEEDED PROMPT' } } }),
  });
  vi.stubGlobal('fetch', fetchMock);
});
afterEach(() => vi.unstubAllGlobals());

const params = (slug: string) => ({ params: Promise.resolve({ slug }) });

describe('workflow guides', () => {
  it('publishes exactly the three evidence-backed guides', () => {
    expect(WORKFLOW_GUIDES.map((g) => g.slug)).toEqual([
      'connect-planner-executor',
      'connect-builder-reviewer',
      'resume-across-two-clis',
    ]);
  });

  for (const guide of WORKFLOW_GUIDES) {
    it(`${guide.slug}: renders its steps, the live prompt and what was tested`, async () => {
      const { container } = render(await GuidePage(params(guide.slug)));
      const text = container.textContent ?? '';
      expect(container.querySelector('h1')?.textContent).toBe(guide.title);
      expect(text).toContain('SEEDED PROMPT');
      expect(text).toContain('Tested over plain HTTPS');
      expect(text).toContain(guide.tested.date);
      expect(text).toContain(guide.tested.commit);
      expect(text).toContain(guide.tested.test);
      for (const step of guide.steps) expect(text).toContain(step);
      const hrefs = [...container.querySelectorAll('a')].map((a) => a.getAttribute('href'));
      expect(hrefs).toContain('/connect');
      expect(text).not.toMatch(/claude code|codex|cursor|gemini/i);
      const [url, init] = fetchMock.mock.calls[0];
      expect(String(url)).toContain(`/v1/connect?preset=${guide.preset}`);
      expect(init).toMatchObject({ cache: 'no-store' });
    });

    it(`${guide.slug}: has its own canonical and breadcrumbs`, async () => {
      const metadata = await generateMetadata(params(guide.slug));
      expect(metadata.alternates?.canonical).toBe(`/docs/guides/${guide.slug}`);
      expect(metadata.title).toBe(guide.title);
      const { container } = render(await GuidePage(params(guide.slug)));
      const ld = [...container.querySelectorAll('script[type="application/ld+json"]')].map((s) => JSON.parse(s.innerHTML));
      expect(ld[0].itemListElement.map((i: { item: string }) => i.item)).toEqual([
        'https://solvr.dev/', 'https://solvr.dev/docs', 'https://solvr.dev/docs/guides', `https://solvr.dev/docs/guides/${guide.slug}`,
      ]);
    });
  }

  it('still teaches the workflow when the live prompt cannot be read', async () => {
    fetchMock.mockRejectedValue(new TypeError('fetch failed'));
    const { container } = render(await GuidePage(params(WORKFLOW_GUIDES[0].slug)));
    expect(container.textContent).toContain(WORKFLOW_GUIDES[0].steps[0]);
    expect([...container.querySelectorAll('a')].map((a) => a.getAttribute('href'))).toContain('/connect');
  });

  it('answers a real 404 for a guide that does not exist', async () => {
    await expect(GuidePage(params('connect-everything'))).rejects.toThrow('NEXT_NOT_FOUND');
  });
});
