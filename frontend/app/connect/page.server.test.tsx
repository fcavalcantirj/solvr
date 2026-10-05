import { renderToStaticMarkup } from 'react-dom/server';
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';

import { CONNECT_FLOW_ID, CONNECT_START, CONNECT_START_NO_FLOW } from '@/components/connect/connect-fixture';
import ConnectPage from './page';

// /connect's server HTML carries the default sentence (SPEC.md 25.6). The page reads
// GET /v1/connect?flow=none on the server and the panel renders that answer at once, so the
// biggest thing on the page is painted with the HTML, and the explanation under it does not
// jump down when the browser's own read arrives. Here the panel is the real one.
//
// The stub answers like the API: a read without flow=none gets a flow code on every link,
// as a browser does, so the HTML shows which read the page made.

vi.mock('@/components/header', () => ({ Header: () => <header data-testid="page-header" /> }));
vi.mock('@/components/connect/direct-create-panel', () => ({ DirectCreatePanel: () => null }));
// The browser's read never runs in a server render, and nothing here may reach a network.
vi.mock('@/lib/api', () => ({ api: { getConnectStart: vi.fn(), postFunnelEvent: vi.fn() } }));

const fetchMock = vi.fn(async (input: unknown) => {
  const url = new URL(String(input));
  if (url.pathname !== '/v1/connect') return { ok: false, status: 404, json: async () => ({}) };
  const data = url.searchParams.get('flow') === 'none' ? CONNECT_START_NO_FLOW : CONNECT_START;
  return { ok: true, status: 200, json: async () => ({ data }) };
});
beforeEach(() => {
  fetchMock.mockClear();
  vi.stubGlobal('fetch', fetchMock);
});
afterEach(() => vi.unstubAllGlobals());

const serverHtml = async () => renderToStaticMarkup(await ConnectPage());
const parse = (html: string) => {
  const root = document.createElement('div');
  root.innerHTML = html;
  return root;
};

describe('the /connect server HTML', () => {
  it('carries the default sentence of GET /v1/connect?flow=none, word for word, and no flow code', async () => {
    const html = await serverHtml();

    expect(fetchMock).toHaveBeenCalledTimes(1);
    const read = new URL(String(fetchMock.mock.calls[0][0]));
    expect(read.pathname + read.search).toBe('/v1/connect?flow=none');
    expect(parse(html).querySelector('[data-testid="prompt-sentence"]')?.textContent).toBe(CONNECT_START_NO_FLOW.prompt.text);
    expect(html).toContain('href="https://solvr.dev/skill.md"');
    expect(html).not.toContain('skill.md?f=');
    expect(html).not.toContain(CONNECT_FLOW_ID);
  });

  // The code arrives with the browser's answer; the server HTML holds its width, empty and
  // hidden, in the link of the sentence shown and of the two held, so nothing moves then.
  it('holds the width of the code in every link, with no character in it', async () => {
    const html = await serverHtml();
    const links = [...parse(html).querySelectorAll('a[data-kind="link"]')];

    expect(links).toHaveLength(3);
    for (const link of links) {
      expect(link.textContent).toBe('https://solvr.dev/skill.md');
      const placeholder = link.lastElementChild!;
      expect(placeholder.outerHTML).toBe('<span aria-hidden="true" class="inline-block w-[11ch] text-[0.78em]"></span>');
    }
    expect(html.match(/skill\.md\?f=/g)).toBeNull();
  });

  it('carries the use cases, the line beside the sentence and the Copy button, and no loading state', async () => {
    const page = parse(await serverHtml());

    const radios = [...page.querySelectorAll<HTMLInputElement>('input[type="radio"]')];
    expect(radios.map((radio) => radio.value)).toEqual(CONNECT_START_NO_FLOW.presets.map((p) => p.value));
    expect(radios.filter((radio) => radio.hasAttribute('checked')).map((radio) => radio.value)).toEqual(['plan-and-build']);
    expect(page.textContent).toContain(CONNECT_START_NO_FLOW.next);
    expect([...page.querySelectorAll('button')].map((button) => button.textContent)).toContain('Copy prompt');
    expect(page.querySelector('[data-testid="connect-loading"]')).toBeNull();
  });

  it('keeps exactly one h1, the page\'s own: the panel renders none on /connect', async () => {
    const html = await serverHtml();
    expect(html.match(/<h1\b/g)).toHaveLength(1);
    expect(html).toMatch(/<h1\b[^>]*>Connect two agents<\/h1>/);
  });

  it('shows the panel\'s loading state, as before, when the server read fails', async () => {
    fetchMock.mockImplementationOnce(async () => ({ ok: false, status: 503, json: async () => ({}) }));
    const page = parse(await serverHtml());

    expect(page.querySelector('[data-testid="connect-loading"]')?.textContent).toBe('READING THE SENTENCE...');
    expect(page.querySelector('[data-testid="prompt-sentence"]')).toBeNull();
    // What the page says itself is still there.
    expect(page.querySelector('h1')?.textContent).toBe('Connect two agents');
  });
});
