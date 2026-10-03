import { describe, it, expect } from 'vitest';
import { parseHead, extractLinks, isNoindex, checkPage, relLink, messageAnchors } from './seo-verify.mjs';

const page = `<!DOCTYPE html><html><head>
<title>Kestrel build | Solvr</title>
<meta name="description" content="Plan &amp; ship"/>
<meta name="robots" content="noindex, follow"/>
<link rel="canonical" href="https://solvr.dev/rooms/kestrel"/>
<script type="application/ld+json">{"@type":"WebPage"}</script>
</head><body>
<a href="/rooms">Rooms</a><a href="/rooms/kestrel/history/1">History 1</a>
<a href="https://example.com/x">out</a><a href="//cdn.example.com">cdn</a><a href="/rooms">dup</a>
<script>document.write('<a href="/hidden">x</a>')</script>
</body></html>`;

describe('seo-verify', () => {
  it('reads title, description, robots, canonical and JSON-LD from server HTML', () => {
    const head = parseHead(page);
    expect(head.title).toBe('Kestrel build | Solvr');
    expect(head.description).toBe('Plan & ship');
    expect(head.robots).toBe('noindex, follow');
    expect(head.canonical).toBe('https://solvr.dev/rooms/kestrel');
    expect(head.jsonLd).toEqual(['{"@type":"WebPage"}']);
  });

  it('extracts only same-site anchors a crawler without JavaScript can follow', () => {
    expect(extractLinks(page)).toEqual(['/rooms', '/rooms/kestrel/history/1']);
  });

  it('recognizes noindex', () => {
    expect(isNoindex('noindex, follow')).toBe(true);
    expect(isNoindex('index, follow')).toBe(false);
    expect(isNoindex(undefined)).toBe(false);
  });

  it('checks an indexable page against its canonical and robots', () => {
    const ok = `<link rel="canonical" href="https://solvr.dev/posts"/>`;
    expect(checkPage('/posts', 'index', { status: 200, html: ok }, 'https://solvr.dev')).toEqual([]);
    expect(checkPage('/posts?utm_source=x', 'index', { status: 200, html: ok }, 'https://solvr.dev')).toEqual([]);
    expect(checkPage('/posts', 'index', { status: 200, html: page }, 'https://solvr.dev')).toHaveLength(2);
    expect(checkPage('/', 'index', { status: 200, html: `<link rel="canonical" href="https://solvr.dev"/>` }, 'https://solvr.dev')).toEqual([]);
  });

  it('checks a noindex page', () => {
    expect(checkPage('/login', 'noindex', { status: 200, html: page }, 'https://solvr.dev')).toEqual([]);
    expect(checkPage('/login', 'noindex', { status: 200, html: '<title>x</title>' }, 'https://solvr.dev')).toEqual([
      '/login: robots "(none)", want noindex',
    ]);
  });
});

describe('seo-verify crawl helpers', () => {
  const page = `<a href="/rooms/x/history/1" rel="prev">Earlier</a><a rel="next" href="/rooms/x/history/3">Later</a>
<li id="message-101"></li><li id="message-102"></li><div id="messages-103"></div>`;

  it('finds the prev and next links', () => {
    expect(relLink(page, 'prev')).toBe('/rooms/x/history/1');
    expect(relLink(page, 'next')).toBe('/rooms/x/history/3');
    expect(relLink('<a href="/x">x</a>', 'next')).toBeUndefined();
  });

  it('counts rendered transcript messages', () => {
    expect(messageAnchors(page)).toEqual([101, 102]);
  });
});
