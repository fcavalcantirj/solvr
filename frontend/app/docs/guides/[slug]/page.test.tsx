import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, fireEvent, waitFor, screen } from '@testing-library/react';

// Task idx 84, v1.3.5: each use-case guide is a title, one line and the API's example
// sentence, big and read-only (GET /v1/connect/examples) — no step lists and no endpoints.
// The unlisted resume guide keeps its long-form content and its tested record.

vi.mock('@/components/header', () => ({ Header: () => null }));
vi.mock('@/components/footer', () => ({ Footer: () => null }));
vi.mock('next/navigation', () => ({
  notFound: () => {
    throw new Error('NEXT_NOT_FOUND');
  },
}));
vi.mock('@/lib/api', () => ({ api: { postFunnelEvent: vi.fn() } }));

import { api } from '@/lib/api';
import GuidePage, { generateMetadata } from './page';
import { LISTED_GUIDES, WORKFLOW_GUIDES } from '@/lib/docs/workflow-guides';
import { CONNECT_EXAMPLES } from '@/components/connect/connect-fixture';

const fetchMock = vi.fn();
beforeEach(() => {
  fetchMock.mockReset();
  fetchMock.mockImplementation(async (url: string) => ({
    ok: true,
    status: 200,
    json: async () =>
      String(url).includes('/v1/connect/examples')
        ? { data: { instruction_version: '2.0', presets: CONNECT_EXAMPLES } }
        : { data: { prompt: { text: 'Learn Solvr from https://solvr.dev/skill.md. SEEDED SENTENCE' } } },
  }));
  vi.stubGlobal('fetch', fetchMock);
});
afterEach(() => vi.unstubAllGlobals());

const params = (slug: string) => ({ params: Promise.resolve({ slug }) });

describe('use-case guides', () => {
  it('lists the three use cases, in the order of the sentences', () => {
    expect(LISTED_GUIDES.map((g) => [g.slug, g.preset])).toEqual([
      ['connect-planner-executor', 'plan-and-build'],
      ['share-context-between-agents', 'collaborate'],
      ['connect-builder-reviewer', 'build-and-review'],
    ]);
  });

  for (const guide of LISTED_GUIDES) {
    it(`${guide.slug}: a title, one line and the example sentence, nothing else`, async () => {
      const { container } = render(await GuidePage(params(guide.slug)));
      const text = container.textContent ?? '';
      expect(container.querySelector('h1')).toHaveTextContent(guide.title);
      expect(text).toContain(guide.description);
      const example = CONNECT_EXAMPLES.find((p) => p.value === guide.preset)!;
      expect(container.querySelector('[data-testid="prompt-sentence"]')?.textContent).toBe(example.prompt.text);
      expect(text).toContain(example.next);
      expect(container.querySelector('ol')).toBeNull();
      for (const forbidden of ['api.solvr.dev', '/v1/', 'curl', 'STEPS', 'WHAT WAS TESTED']) {
        expect(text).not.toContain(forbidden);
      }
      expect(text).not.toMatch(/claude code|codex|cursor|gemini/i);
    });
  }

  it('reads the examples from the API', async () => {
    render(await GuidePage(params(LISTED_GUIDES[0].slug)));
    expect(String(fetchMock.mock.calls[0][0])).toContain('/v1/connect/examples');
  });

  it('points at Connect when the sentence cannot be read', async () => {
    fetchMock.mockResolvedValue({ ok: false, status: 503, json: async () => ({}) });
    const { container } = render(await GuidePage(params(LISTED_GUIDES[1].slug)));
    expect(container.querySelector('[data-testid="prompt-sentence"]')).toBeNull();
    expect(container.querySelector('a[href="/connect?preset=collaborate"]')).not.toBeNull();
  });
});

// A guide's Copy prompt reported nothing. It now reports the browser funnel step
// starter_prompt_copied for the guide's own use case, only after the copy succeeded.
describe('use-case guides: the copy is reported', () => {
  beforeEach(() => {
    vi.mocked(api.postFunnelEvent).mockReset();
    vi.mocked(api.postFunnelEvent).mockResolvedValue(undefined);
    Object.assign(navigator, { clipboard: { writeText: vi.fn().mockResolvedValue(undefined) } });
  });

  it.each([
    ['connect-planner-executor', 'plan-and-build', 'planner'],
    ['share-context-between-agents', 'collaborate', 'learner'],
    ['connect-builder-reviewer', 'build-and-review', 'builder'],
  ])('%s reports one starter_prompt_copied after a successful copy', async (slug, preset, role) => {
    render(await GuidePage(params(slug)));
    expect(api.postFunnelEvent).not.toHaveBeenCalled();

    fireEvent.click(screen.getByRole('button', { name: /copy prompt/i }));

    const example = CONNECT_EXAMPLES.find((p) => p.value === preset)!;
    await waitFor(() => expect(navigator.clipboard.writeText).toHaveBeenCalledWith(example.prompt.text));
    await waitFor(() => expect(api.postFunnelEvent).toHaveBeenCalledTimes(1));
    expect(api.postFunnelEvent).toHaveBeenCalledWith({
      event: 'starter_prompt_copied',
      entry_surface: 'guide_page',
      preset,
      role,
    });
  });

  it('reports nothing when the browser blocks the clipboard', async () => {
    Object.assign(navigator, { clipboard: { writeText: vi.fn().mockRejectedValue(new Error('denied')) } });
    render(await GuidePage(params(LISTED_GUIDES[0].slug)));

    fireEvent.click(screen.getByRole('button', { name: /copy prompt/i }));

    expect(await screen.findByRole('alert')).toBeInTheDocument();
    expect(api.postFunnelEvent).not.toHaveBeenCalled();
  });
});

describe('the resume guide (unlisted)', () => {
  const resume = WORKFLOW_GUIDES.find((g) => g.slug === 'resume-across-two-clis')!;

  it('is not listed, and keeps its content', async () => {
    expect(resume.listed).toBe(false);
    const { container } = render(await GuidePage(params(resume.slug)));
    const text = container.textContent ?? '';
    for (const step of resume.steps ?? []) expect(text).toContain(step);
    expect(text).toContain(resume.tested!.test);
    expect(text).toContain('SEEDED SENTENCE');
  });
});

describe('guide metadata and missing guides', () => {
  it('titles each guide and canonicalizes its URL', async () => {
    for (const guide of WORKFLOW_GUIDES) {
      const meta = await generateMetadata(params(guide.slug));
      expect(meta.title).toBe(guide.title);
      expect(meta.alternates?.canonical).toBe(`/docs/guides/${guide.slug}`);
    }
  });

  it('answers 404 for an unknown guide', async () => {
    await expect(GuidePage(params('no-such-guide'))).rejects.toThrow('NEXT_NOT_FOUND');
  });
});
