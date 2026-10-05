import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, fireEvent, waitFor, screen } from '@testing-library/react';
import { renderToStaticMarkup } from 'react-dom/server';

// SPEC.md 27.5 (owner, 2026-10-05): every guide is a full how-to in its server HTML. It
// says what it is for, shows the API's sentence (never one typed here), the steps, what
// the agents did in a real run where one exists, what to do when something goes wrong,
// and a run record naming the agents that were run and every one that was not. It links
// the other guides, Connect and the rooms.

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
import { WORKFLOW_GUIDES, guideTexts, workflowGuide } from '@/lib/docs/workflow-guides';
import { AGENT_VERSIONS, RUNS, RUNS_BUILD, RUNS_DATE } from '@/lib/docs/guide-runs';
import { CONNECT_EXAMPLES, CONNECT_START } from '@/components/connect/connect-fixture';

// The resume guide reads GET /v1/connect?flow=none: the sentence as the API serves it now.
const RESUME_SENTENCE = CONNECT_START.presets[0].prompt.text;

const fetchMock = vi.fn();
beforeEach(() => {
  fetchMock.mockReset();
  fetchMock.mockImplementation(async (url: string) => ({
    ok: true,
    status: 200,
    json: async () =>
      String(url).includes('/v1/connect/examples')
        ? { data: { instruction_version: '2.1', presets: CONNECT_EXAMPLES } }
        : { data: CONNECT_START },
  }));
  vi.stubGlobal('fetch', fetchMock);
});
afterEach(() => vi.unstubAllGlobals());

const params = (slug: string) => ({ params: Promise.resolve({ slug }) });

// Tags become spaces, so the words of two elements never run together.
const words = (html: string) =>
  html
    .replace(/<[^>]+>/g, ' ')
    .replace(/&amp;/g, '&')
    .replace(/&#x27;|&apos;/g, "'")
    .replace(/&quot;/g, '"')
    .replace(/&lt;/g, '<')
    .replace(/&gt;/g, '>')
    .replace(/\s+/g, ' ')
    .trim();

// Inline code and links sit right beside punctuation, so text is compared without spaces.
const unspaced = (text: string) => text.replace(/\s+/g, '');

async function serverHtml(slug: string): Promise<string> {
  return renderToStaticMarkup(await GuidePage(params(slug)));
}

// The words a guide says itself: its main text without the API's sentence and without the
// list of other guides at its end.
async function ownWords(slug: string): Promise<number> {
  const { container } = render(await GuidePage(params(slug)));
  const main = container.querySelector('main')!.cloneNode(true) as HTMLElement;
  main.querySelectorAll('[data-testid="guide-prompt"], [data-testid="more-guides"]').forEach((node) => node.remove());
  return words(main.innerHTML).split(' ').filter(Boolean).length;
}

const HOW_TOS = WORKFLOW_GUIDES.filter((g) => g.kind !== 'resume');

describe('every guide says all it says in the server HTML', () => {
  for (const guide of WORKFLOW_GUIDES) {
    it(`${guide.slug}: title, first paragraph, every section and the run record`, async () => {
      const html = await serverHtml(guide.slug);
      const text = words(html);
      expect(html.match(/<h1\b/g)).toHaveLength(1);
      expect(html).toContain(`>${guide.title}</h1>`);
      for (const piece of guideTexts(guide)) expect(unspaced(text), piece).toContain(unspaced(piece));
      expect(html).toContain('data-testid="run-record"');
      expect(html).toContain('data-testid="guide-intro"');
    });
  }

  for (const guide of HOW_TOS) {
    it(`${guide.slug}: a full how-to, at least 350 words of its own and no wall`, async () => {
      const count = await ownWords(guide.slug);
      expect(count).toBeGreaterThanOrEqual(350);
      expect(count).toBeLessThanOrEqual(700);
    });
  }

  it('names no endpoint and no API host on a guide page', async () => {
    for (const guide of WORKFLOW_GUIDES) {
      const text = words(await serverHtml(guide.slug));
      for (const forbidden of ['api.solvr.dev', '/v1/', 'curl ', 'handshake', 'Bearer']) {
        expect(text, `${guide.slug} ${forbidden}`).not.toContain(forbidden);
      }
    }
  });
});

describe('the sentence on a guide is the API\'s', () => {
  for (const guide of WORKFLOW_GUIDES.filter((g) => g.kind !== 'resume')) {
    it(`${guide.slug} shows the example sentence of its use case, as served`, async () => {
      const { container } = render(await GuidePage(params(guide.slug)));
      const example = CONNECT_EXAMPLES.find((p) => p.value === guide.preset)!;
      expect(container.querySelector('[data-testid="prompt-sentence"]')?.textContent).toBe(example.prompt.text);
      expect(container.textContent).toContain(example.next);
    });
  }

  it('reads the examples from the API', async () => {
    render(await GuidePage(params('claude-code')));
    expect(String(fetchMock.mock.calls[0][0])).toContain('/v1/connect/examples');
  });

  it('points at Connect when the sentence cannot be read, and keeps the rest of the guide', async () => {
    fetchMock.mockResolvedValue({ ok: false, status: 503, json: async () => ({}) });
    const { container } = render(await GuidePage(params('share-context-between-agents')));
    expect(container.querySelector('[data-testid="prompt-sentence"]')).toBeNull();
    expect(container.querySelector('a[href="/connect?preset=collaborate"]')).not.toBeNull();
    expect(container.querySelector('[data-testid="run-record"]')).not.toBeNull();
  });

  // The page is rendered on the server: its HTML can be cached and shared, and no browser
  // step stands behind a flow code minted for it. The resume guide reads its sentence with
  // flow=none, so the API mints nothing (SPEC.md 25.6).
  it('the resume guide reads its sentence without starting a flow, and shows it as served', async () => {
    const { container } = render(await GuidePage(params('resume-across-two-clis')));
    const reads = fetchMock.mock.calls
      .map((call) => new URL(String(call[0])))
      .filter((url) => url.pathname === '/v1/connect');
    expect(reads).toHaveLength(1);
    expect(reads[0].searchParams.get('flow')).toBe('none');
    expect(reads[0].searchParams.get('preset')).toBe('plan-and-build');
    expect(reads[0].searchParams.get('visibility')).toBe('public');
    expect(container.querySelector('[data-testid="prompt-sentence"]')?.textContent).toBe(RESUME_SENTENCE);
  });
});

describe('the run record', () => {
  for (const guide of WORKFLOW_GUIDES) {
    it(`${guide.slug}: date, build, the agents run with their versions, and those not run`, async () => {
      const { container } = render(await GuidePage(params(guide.slug)));
      const record = container.querySelector('[data-testid="run-record"]')!;
      const text = words(record.innerHTML);
      expect(text).toContain(RUNS_DATE);
      expect(text).toContain(words(RUNS_BUILD));
      for (const id of guide.record.runs) {
        for (const side of [RUNS[id].first, RUNS[id].second]) {
          expect(text).toContain(AGENT_VERSIONS[side.agent as keyof typeof AGENT_VERSIONS]);
          expect(text).toContain(side.role);
        }
      }
      if (guide.record.runs.length === 0) expect(text).toContain('No named agent was run for this guide.');
      expect(text).toContain('Not run for this guide');
      for (const agent of Object.keys(guide.record.notRun)) expect(text).toContain(agent);
    });
  }

  it.each([
    ['hermes', 'Hermes was not run for this guide'],
    ['openclaw', 'OpenClaw was not run for this guide'],
  ])('%s says plainly, first, that it was not run', async (slug, sentence) => {
    const { container } = render(await GuidePage(params(slug)));
    expect(container.querySelector('[data-testid="guide-intro"]')?.textContent).toContain(sentence);
  });

  it('shows a room excerpt for a use case that was run, and says so where none was', async () => {
    const run = render(await GuidePage(params('connect-planner-executor')));
    const excerpt = run.container.querySelector('[data-testid="room-excerpt"]')!;
    expect(excerpt.querySelectorAll('li').length).toBe(RUNS.p5.excerpt!.length);
    expect(excerpt.textContent).toContain('Quiet Lantern');
    run.unmount();

    const none = render(await GuidePage(params('share-context-between-agents')));
    expect(none.container.querySelector('[data-testid="room-excerpt"]')).toBeNull();
    expect(none.container.textContent).toContain('No run of this use case with a named agent is recorded yet');
  });

  it('says what the private sentence says: the second agent gives you its id, you pass it to the first', async () => {
    const text = words(await serverHtml('share-context-between-agents'));
    expect(text).toContain('the expert gives you its agent id');
    expect(text).toContain('Pass the id to the learner');
  });
});

describe('links from a guide', () => {
  for (const guide of WORKFLOW_GUIDES) {
    it(`${guide.slug} links at least two other guides, Connect and the rooms, each a real guide`, async () => {
      const html = await serverHtml(guide.slug);
      const hrefs = [...html.matchAll(/<a\b[^>]*href="([^"]+)"/g)].map((m) => m[1].replace(/&amp;/g, '&'));
      const guides = hrefs.filter((href) => href.startsWith('/docs/guides/'));
      const others = new Set(guides.filter((href) => href !== `/docs/guides/${guide.slug}`));
      expect(others.size).toBeGreaterThanOrEqual(2);
      for (const href of guides) expect(workflowGuide(href.slice('/docs/guides/'.length)), href).toBeDefined();
      expect(hrefs).toContain('/connect');
      expect(hrefs).toContain('/rooms');
    });
  }

  it('links every per-agent guide from each use-case guide', async () => {
    for (const guide of WORKFLOW_GUIDES.filter((g) => g.kind === 'use-case')) {
      const html = await serverHtml(guide.slug);
      for (const slug of ['claude-code', 'codex', 'kimi-code', 'hermes', 'openclaw']) {
        expect(html, `${guide.slug} -> ${slug}`).toContain(`href="/docs/guides/${slug}"`);
      }
    }
  });
});

// A guide's Copy prompt reports the browser funnel step starter_prompt_copied for the
// guide's own use case, only after the copy succeeded.
describe('a guide\'s copy is reported', () => {
  beforeEach(() => {
    vi.mocked(api.postFunnelEvent).mockReset();
    vi.mocked(api.postFunnelEvent).mockResolvedValue(undefined);
    Object.assign(navigator, { clipboard: { writeText: vi.fn().mockResolvedValue(undefined) } });
  });

  it.each([
    ['connect-planner-executor', 'plan-and-build', 'planner'],
    ['share-context-between-agents', 'collaborate', 'learner'],
    ['connect-builder-reviewer', 'build-and-review', 'builder'],
    ['claude-code', 'plan-and-build', 'planner'],
    ['codex', 'build-and-review', 'builder'],
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
    render(await GuidePage(params('connect-planner-executor')));

    fireEvent.click(screen.getByRole('button', { name: /copy prompt/i }));

    expect(await screen.findByRole('alert')).toBeInTheDocument();
    expect(api.postFunnelEvent).not.toHaveBeenCalled();
  });
});

describe('guide metadata and missing guides', () => {
  it('titles, describes and canonicalizes each guide, with its own preview', async () => {
    for (const guide of WORKFLOW_GUIDES) {
      const meta = await generateMetadata(params(guide.slug));
      expect(meta.title).toBe(guide.title);
      expect(meta.description).toBe(guide.description);
      expect(meta.alternates?.canonical).toBe(`/docs/guides/${guide.slug}`);
      expect(meta.openGraph).toMatchObject({ title: `${guide.title} | Solvr`, url: `https://solvr.dev/docs/guides/${guide.slug}` });
    }
  });

  it('answers 404 for an unknown guide', async () => {
    await expect(GuidePage(params('no-such-guide'))).rejects.toThrow('NEXT_NOT_FOUND');
  });
});
