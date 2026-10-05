import { render, screen, within } from '@testing-library/react';
import { renderToStaticMarkup } from 'react-dom/server';
import { describe, it, expect, vi } from 'vitest';
import { readFileSync } from 'node:fs';
import { join } from 'node:path';

import ConnectPage, { metadata } from './page';

// /connect stays a real, directly linkable page. It renders the SAME panel the
// index opens inline — same component, same GET /v1/connect contract — so the
// two surfaces cannot drift apart.

vi.mock('@/components/header', () => ({
  Header: () => <header data-testid="page-header" />,
}));

vi.mock('@/components/connect/connect-panel', () => ({
  ConnectPanel: ({ variant }: { variant?: string }) => (
    <div data-testid="page-connect-panel" data-variant={variant} />
  ),
}));

// The signed-in-only direct-create panel reads auth, so mock it here the same
// way the header is mocked — the page test never mounts an AuthProvider.
vi.mock('@/components/connect/direct-create-panel', () => ({
  DirectCreatePanel: () => <div data-testid="page-direct-create-panel" />,
}));

const read = (file: string) => readFileSync(join(process.cwd(), file), 'utf8');

describe('the /connect page', () => {
  it('renders the shared connection panel as the page', () => {
    render(<ConnectPage />);
    const panel = screen.getByTestId('page-connect-panel');
    expect(panel).toHaveAttribute('data-variant', 'page');
  });

  it('keeps the header above it and the footer below it', () => {
    render(<ConnectPage />);
    const header = screen.getByTestId('page-header');
    const panel = screen.getByTestId('page-connect-panel');
    const footer = screen.getByRole('navigation', { name: /footer/i });
    // DOCUMENT_POSITION_FOLLOWING = 4
    expect(header.compareDocumentPosition(panel) & 4).toBeTruthy();
    expect(panel.compareDocumentPosition(footer) & 4).toBeTruthy();
  });

  it('offers the signed-in direct-create path below the prompt-first panel', () => {
    render(<ConnectPage />);
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
  const serverHtml = () => renderToStaticMarkup(<ConnectPage />);
  const text = (html: string) =>
    html
      .replace(/<[^>]+>/g, ' ')
      .replace(/&amp;/g, '&')
      .replace(/&#x27;|&apos;/g, "'")
      .replace(/\s+/g, ' ')
      .trim();
  // What the page itself says: the header and the footer are other components' words.
  const copy = () => {
    const { container } = render(<ConnectPage />);
    const main = container.querySelector('main')!.cloneNode(true) as HTMLElement;
    main.querySelectorAll('header, footer, [data-testid^="page-"]').forEach((node) => node.remove());
    // Tags become spaces, so the words of two elements never run together.
    return text(main.innerHTML);
  };

  it('has one <h1>, "Connect two agents", above the panel', () => {
    const html = serverHtml();
    expect(html.match(/<h1\b/g)).toHaveLength(1);
    expect(html).toMatch(/<h1\b[^>]*>Connect two agents<\/h1>/);

    render(<ConnectPage />);
    const heading = screen.getByRole('heading', { level: 1 });
    expect(heading.compareDocumentPosition(screen.getByTestId('page-connect-panel')) & 4).toBeTruthy();
  });

  it('explains how it works in three short steps, under the panel', () => {
    render(<ConnectPage />);
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

  it('links the workflow guides and the public rooms, marked for the click listener', () => {
    render(<ConnectPage />);
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
    expect(serverHtml()).toMatch(/<a\b[^>]*href="\/docs\/guides"/);
    expect(serverHtml()).toMatch(/<a\b[^>]*href="\/rooms"/);
  });

  // The paragraph that names the agents links each name to its guide (SPEC.md 27.5): one
  // link per name, the words unchanged.
  it('links each agent it names to that agent\'s guide, once', () => {
    render(<ConnectPage />);
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
    expect(serverHtml()).toMatch(/<a\b[^>]*href="\/docs\/guides\/claude-code"[^>]*>Claude Code<\/a>/);
  });

  // Every statement is one skill/SKILL.md or the code makes: the first agent creates the
  // room and answers with the sentence for the second; plain HTTPS, no install, no human
  // signup; any agent that can make HTTPS requests; a public room is a page in a browser.
  it('says what the skill says, and names the agents', () => {
    const words = copy();
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

  it('is 120 to 220 words, so the panel stays the page', () => {
    const count = copy().split(' ').filter(Boolean).length;
    expect(count).toBeGreaterThanOrEqual(120);
    expect(count).toBeLessThanOrEqual(220);
  });

  it('uses each of the searcher\'s two phrases once, in sentence case, and no endpoint', () => {
    const words = copy();
    expect(words.match(/connect two agents/gi)).toHaveLength(1);
    expect(words.match(/connect your agents/gi)).toHaveLength(1);
    for (const forbidden of ['/v1/', 'api.solvr.dev', 'GET ', 'POST ', 'endpoint', 'token', 'handshake', 'JSON']) {
      expect(words, forbidden).not.toContain(forbidden);
    }
    render(<ConnectPage />);
    for (const heading of screen.getAllByRole('heading')) {
      const title = heading.textContent ?? '';
      expect(title, title).toBe(title.charAt(0).toUpperCase() + title.slice(1));
      expect(title, title).not.toMatch(/ [A-Z][a-z]+ [A-Z][a-z]/);
    }
  });

  it('never reads the sentence on the server: the page stays cacheable and mints no flow', () => {
    const page = read('app/connect/page.tsx');
    const explainer = read('components/connect/connect-explainer.tsx');
    for (const source of [page, explainer]) {
      expect(source).not.toMatch(/\bfetch\(/);
      expect(source).not.toContain('getConnectStart');
      expect(source).not.toContain('readForPage');
      expect(source).not.toMatch(/^\s*export const (dynamic|revalidate)\b/m);
      expect(source).not.toMatch(/\basync function\b/);
    }
    expect(text(serverHtml())).not.toMatch(/Learn Solvr from/);
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
