import { render, screen, within } from '@testing-library/react';
import { renderToStaticMarkup } from 'react-dom/server';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { readFileSync } from 'node:fs';
import { join } from 'node:path';

import { CONNECT_START_NO_FLOW } from '@/components/connect/connect-fixture';
import type { APIConnectStart } from '@/lib/api-types';
import ConnectPage, { metadata } from './page';

// /connect stays a real, directly linkable page. It renders the SAME panel the
// index opens inline — same component, same GET /v1/connect contract — so the
// two surfaces cannot drift apart.

vi.mock('@/components/header', () => ({
  Header: () => <header data-testid="page-header" />,
}));

// The mocked panel names the sentence it was handed, as an attribute: the words the page
// itself says (counted below) stay the page's own.
vi.mock('@/components/connect/connect-panel', () => ({
  ConnectPanel: ({ variant, initial }: { variant?: string; initial?: APIConnectStart | null }) => (
    <div data-testid="page-connect-panel" data-variant={variant} data-initial={initial?.prompt.text ?? ''} />
  ),
}));

// The server read of the default sentence (page.server.test.tsx renders it with the real panel).
const getDefaultConnectStart = vi.hoisted(() => vi.fn());
vi.mock('@/lib/connect-start-server', () => ({ getDefaultConnectStart }));
beforeEach(() => {
  getDefaultConnectStart.mockReset();
  getDefaultConnectStart.mockResolvedValue(CONNECT_START_NO_FLOW);
});

// The signed-in-only direct-create panel reads auth, so mock it here the same
// way the header is mocked — the page test never mounts an AuthProvider.
vi.mock('@/components/connect/direct-create-panel', () => ({
  DirectCreatePanel: () => <div data-testid="page-direct-create-panel" />,
}));

const read = (file: string) => readFileSync(join(process.cwd(), file), 'utf8');

describe('the /connect page', () => {
  it('renders the shared connection panel as the page', async () => {
    render(await ConnectPage());
    const panel = screen.getByTestId('page-connect-panel');
    expect(panel).toHaveAttribute('data-variant', 'page');
  });

  // The default sentence, read on the server once, is the panel's first render (SPEC.md 25.6).
  it('hands the panel the default sentence it read on the server', async () => {
    render(await ConnectPage());
    expect(getDefaultConnectStart).toHaveBeenCalledTimes(1);
    expect(getDefaultConnectStart).toHaveBeenCalledWith();
    expect(screen.getByTestId('page-connect-panel')).toHaveAttribute('data-initial', CONNECT_START_NO_FLOW.prompt.text);
  });

  it('hands the panel nothing when that read failed, so it shows its loading state', async () => {
    getDefaultConnectStart.mockResolvedValue(null);
    render(await ConnectPage());
    expect(screen.getByTestId('page-connect-panel')).toHaveAttribute('data-initial', '');
  });

  it('keeps the header above it and the footer below it', async () => {
    render(await ConnectPage());
    const header = screen.getByTestId('page-header');
    const panel = screen.getByTestId('page-connect-panel');
    const footer = screen.getByRole('navigation', { name: /footer/i });
    // DOCUMENT_POSITION_FOLLOWING = 4
    expect(header.compareDocumentPosition(panel) & 4).toBeTruthy();
    expect(panel.compareDocumentPosition(footer) & 4).toBeTruthy();
  });

  it('offers the signed-in direct-create path below the prompt-first panel', async () => {
    render(await ConnectPage());
    const panel = screen.getByTestId('page-connect-panel');
    const directCreate = screen.getByTestId('page-direct-create-panel');
    // DOCUMENT_POSITION_FOLLOWING = 4 — the prompt-first panel comes first.
    expect(panel.compareDocumentPosition(directCreate) & 4).toBeTruthy();
  });

  it('is its own canonical URL, so it can be linked and shared directly', () => {
    expect(metadata.alternates?.canonical).toBe('/connect');
    expect(metadata.title).toBeTruthy();
  });

  it('reads no authentication state and sends nobody to a login', () => {
    const source = read('app/connect/page.tsx');
    expect(source).not.toContain('use-auth');
    expect(source).not.toContain('/login');
    expect(source).not.toContain('router.push');
  });

  it('renders the same panel component the index opens inline', () => {
    const page = read('app/connect/page.tsx');
    const hero = read('components/hero-section.tsx');
    for (const source of [page, hero]) {
      expect(source).toContain('@/components/connect/connect-panel');
    }
  });
});

// Recon finding F02: the server HTML of /connect held forty words and no <h1>; the heading
// existed only after the browser had read the sentence. The page now says what it is in the
// HTML the server sends: one heading, and under the panel how it works in three steps, then
// the guides and the public rooms. The panel is mocked away here, so everything these tests
// read is server-rendered by the page itself.
describe('/connect says what it is in the server HTML', () => {
  const serverHtml = async () => renderToStaticMarkup(await ConnectPage());
  const text = (html: string) =>
    html
      .replace(/<[^>]+>/g, ' ')
      .replace(/&amp;/g, '&')
      .replace(/&#x27;|&apos;/g, "'")
      .replace(/\s+/g, ' ')
      .trim();
  // What the page itself says: the header and the footer are other components' words.
  const copy = async () => {
    const { container } = render(await ConnectPage());
    const main = container.querySelector('main')!.cloneNode(true) as HTMLElement;
    main.querySelectorAll('header, footer, [data-testid^="page-"]').forEach((node) => node.remove());
    // Tags become spaces, so the words of two elements never run together.
    return text(main.innerHTML);
  };

  it('has one <h1>, "Connect two agents", above the panel', async () => {
    const html = await serverHtml();
    expect(html.match(/<h1\b/g)).toHaveLength(1);
    expect(html).toMatch(/<h1\b[^>]*>Connect two agents<\/h1>/);

    render(await ConnectPage());
    const heading = screen.getByRole('heading', { level: 1 });
    expect(heading.compareDocumentPosition(screen.getByTestId('page-connect-panel')) & 4).toBeTruthy();
  });

  it('explains how it works in three short steps, under the panel', async () => {
    render(await ConnectPage());
    const how = screen.getByRole('region', { name: 'How it works' });
    expect(screen.getByTestId('page-connect-panel').compareDocumentPosition(how) & 4).toBeTruthy();
    expect(within(how).getByRole('heading', { level: 2, name: 'How it works' })).toBeInTheDocument();
    const steps = within(how).getAllByRole('listitem').filter((item) => item.closest('ol'));
    expect(steps).toHaveLength(3);
    expect(steps.map((step) => within(step).getByRole('heading', { level: 3 }).textContent)).toEqual([
      'Copy the sentence',
      'Your agent creates a room',
      'Paste that into the second agent',
    ]);
  });

  it('links the workflow guides and the public rooms, marked for the click listener', async () => {
    render(await ConnectPage());
    const how = screen.getByRole('region', { name: 'How it works' });
    const links = Object.fromEntries(
      within(how).getAllByRole('link').map((a) => [
        a.getAttribute('href'),
        [a.getAttribute('data-track'), a.getAttribute('data-track-item'), a.getAttribute('data-track-location')],
      ]),
    );
    expect(links).toEqual({
      '/docs/guides': ['cta', 'workflow_guides', 'page'],
      '/rooms': ['cta', 'public_rooms', 'page'],
      '/docs/guides/claude-code': ['cta', 'claude_code', 'page'],
      '/docs/guides/codex': ['cta', 'codex', 'page'],
      '/docs/guides/kimi-code': ['cta', 'kimi_code', 'page'],
      '/docs/guides/hermes': ['cta', 'hermes', 'page'],
      '/docs/guides/openclaw': ['cta', 'openclaw', 'page'],
    });
    // Plain links in the HTML the server sends.
    expect(await serverHtml()).toMatch(/<a\b[^>]*href="\/docs\/guides"/);
    expect(await serverHtml()).toMatch(/<a\b[^>]*href="\/rooms"/);
  });

  // The paragraph that names the agents links each name to its guide (SPEC.md 27.5): one
  // link per name, the words unchanged.
  it('links each agent it names to that agent\'s guide, once', async () => {
    render(await ConnectPage());
    const how = screen.getByRole('region', { name: 'How it works' });
    for (const [name, slug] of [
      ['Claude Code', 'claude-code'],
      ['Codex', 'codex'],
      ['Kimi Code', 'kimi-code'],
      ['Hermes', 'hermes'],
      ['OpenClaw', 'openclaw'],
    ]) {
      const named = within(how).getAllByRole('link', { name });
      expect(named, name).toHaveLength(1);
      expect(named[0]).toHaveAttribute('href', `/docs/guides/${slug}`);
    }
    expect(await serverHtml()).toMatch(/<a\b[^>]*href="\/docs\/guides\/claude-code"[^>]*>Claude Code<\/a>/);
  });

  // Every statement is one skill/SKILL.md or the code makes: the first agent creates the
  // room and answers with the sentence for the second; plain HTTPS, no install, no human
  // signup; any agent that can make HTTPS requests; a public room is a page in a browser.
  it('says what the skill says, and names the agents', async () => {
    const words = await copy();
    expect(words).toContain('Paste it into an agent you already run.');
    expect(words).toContain('It answers with a second sentence, written for the other agent.');
    expect(words).toContain('over plain HTTPS, with no install and no human signup');
    expect(words).toContain('all agents that can make an HTTPS request');
    for (const agent of ['Claude Code', 'Codex', 'Kimi Code', 'Hermes', 'OpenClaw']) {
      expect(words, agent).toContain(agent);
    }
    expect(words).toContain('A public room is a web page: open it in a browser');

    const skill = read('../skill/SKILL.md');
    expect(skill).toContain('it creates the room and answers with the sentence for the second agent');
    expect(skill).toContain('Any agent that can make HTTPS requests connects to Solvr');
    expect(skill).toContain('no human signup or installation needed');
  });

  it('is 120 to 220 words, so the panel stays the page', async () => {
    const count = (await copy()).split(' ').filter(Boolean).length;
    expect(count).toBeGreaterThanOrEqual(120);
    expect(count).toBeLessThanOrEqual(220);
  });

  it('uses each of the searcher\'s two phrases once, in sentence case, and no endpoint', async () => {
    const words = await copy();
    expect(words.match(/connect two agents/gi)).toHaveLength(1);
    expect(words.match(/connect your agents/gi)).toHaveLength(1);
    for (const forbidden of ['/v1/', 'api.solvr.dev', 'GET ', 'POST ', 'endpoint', 'token', 'handshake', 'JSON']) {
      expect(words, forbidden).not.toContain(forbidden);
    }
    render(await ConnectPage());
    for (const heading of screen.getAllByRole('heading')) {
      const title = heading.textContent ?? '';
      expect(title, title).toBe(title.charAt(0).toUpperCase() + title.slice(1));
      expect(title, title).not.toMatch(/ [A-Z][a-z]+ [A-Z][a-z]/);
    }
  });

  // The page reads its default sentence on the server through one reader, which asks the API
  // for no flow (lib/connect-start-server.ts, SPEC.md 25.6): the HTML is cached and shared, so
  // it never carries a flow code. The page fetches nothing itself and stays static, the
  // explanation reads nothing at all, and every word of the sentence is the API's.
  it('reads its sentence on the server only through the no-flow reader, and stays static', async () => {
    const page = read('app/connect/page.tsx');
    const explainer = read('components/connect/connect-explainer.tsx');
    expect(page).toContain("from '@/lib/connect-start-server'");
    expect(read('lib/connect-start-server.ts')).toContain('/v1/connect?flow=none`');
    for (const source of [page, explainer]) {
      expect(source).not.toMatch(/\bfetch\(/);
      expect(source).not.toContain('getConnectStart');
      expect(source).not.toMatch(/^\s*export const (dynamic|revalidate)\b/m);
    }
    expect(explainer).not.toMatch(/\basync function\b/);
    expect(text(await serverHtml())).not.toMatch(/Learn Solvr from/);
  });
});

describe('/connect title and description', () => {
  it('say the same thing in the searcher\'s words', () => {
    expect(metadata.title).toEqual({ default: 'Connect two agents in a shared room', template: '%s | Solvr' });
    const description = String(metadata.description);
    expect(description).toContain('Connect your agents with one sentence');
    expect(description).toContain('talk to each other');
    for (const agent of ['Claude Code', 'Codex']) expect(description).toContain(agent);
    expect(description).toContain('No install');
    expect(description.length).toBeGreaterThan(110);
    expect(description.length).toBeLessThanOrEqual(160);
  });

  it('previews under the same title and description', () => {
    expect(metadata.openGraph).toMatchObject({
      title: 'Connect two agents in a shared room | Solvr',
      description: metadata.description,
      url: 'https://solvr.dev/connect',
    });
  });
});
