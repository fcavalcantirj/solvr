import { describe, it, expect, vi } from 'vitest';
import { render } from '@testing-library/react';
import { renderToStaticMarkup } from 'react-dom/server';

// SPEC.md 27.2: the post archive and the profile pages list posts as plain links in server
// HTML, so every post the sitemap lists has an inbound link a crawler can follow without
// running a script. A row is the post's title as an ordinary <a>, its date, and (on the
// archive) a link to its author's profile.

// The mock keeps what the list asks of the router: whether a link may be prefetched.
vi.mock('next/link', () => ({
  default: ({ children, href, prefetch, ...props }: { children: React.ReactNode; href: string; prefetch?: boolean; [key: string]: unknown }) => (
    <a href={href} data-prefetch={String(prefetch)} {...props}>{children}</a>
  ),
}));

import { PostLinkList, type LinkedPost } from './post-link-list';

const posts: LinkedPost[] = [
  { id: 'p1', title: 'Hand a plan to an executor', created_at: '2026-09-14T01:30:00Z', author: { id: 'agent_planner', type: 'agent', display_name: 'Planner' } },
  { id: 'p2', title: 'Pool exhausted under <load> & "retries"', created_at: '2026-09-02T08:00:00Z', author: { id: 'u-7', type: 'human', display_name: 'Dev Nine' } },
  { id: 'p3', title: 'A post the moderator wrote', created_at: '2026-08-30T00:00:00Z', author: { id: 'system', type: 'system', display_name: 'Solvr' } },
];

describe('PostLinkList', () => {
  it('server-renders each post as a plain link with its title', () => {
    const html = renderToStaticMarkup(<PostLinkList posts={posts} showAuthor />);
    expect(html).toMatch(/<a\b[^>]*href="\/posts\/p1"[^>]*>Hand a plan to an executor<\/a>/);
    expect(html).toMatch(/<a\b[^>]*href="\/posts\/p2"[^>]*>Pool exhausted under &lt;load&gt; &amp; &quot;retries&quot;<\/a>/);
    expect(html).not.toContain('<script');
  });

  it('keeps the order it was given, one row per post', () => {
    const { container } = render(<PostLinkList posts={posts} showAuthor />);
    const rows = [...container.querySelectorAll('li')];
    expect(rows).toHaveLength(3);
    expect(rows.map((row) => row.querySelector('a')?.getAttribute('href'))).toEqual(['/posts/p1', '/posts/p2', '/posts/p3']);
  });

  it('dates each post by its UTC calendar day, the same on the server and in every browser', () => {
    const original = process.env.TZ;
    process.env.TZ = 'America/Sao_Paulo';
    try {
      const { container } = render(<PostLinkList posts={posts} showAuthor />);
      const times = [...container.querySelectorAll('time')];
      expect(times.map((t) => t.getAttribute('datetime'))).toEqual(posts.map((p) => p.created_at));
      expect(times.map((t) => t.textContent)).toEqual(['2026-09-14', '2026-09-02', '2026-08-30']);
    } finally {
      process.env.TZ = original;
    }
  });

  it('links each author to the profile of its kind, and names an author without one as text', () => {
    const { container } = render(<PostLinkList posts={posts} showAuthor />);
    const rows = [...container.querySelectorAll('li')];
    const authorLink = (row: Element) => [...row.querySelectorAll('a')].find((a) => !a.getAttribute('href')?.startsWith('/posts/'));
    expect(authorLink(rows[0])?.getAttribute('href')).toBe('/agents/agent_planner');
    expect(authorLink(rows[0])?.textContent).toBe('Planner');
    expect(authorLink(rows[1])?.getAttribute('href')).toBe('/users/u-7');
    expect(authorLink(rows[2])).toBeUndefined();
    expect(rows[2].textContent).toContain('Solvr');
  });

  // A post outlives its author's account, and the API then sends the author with no display
  // name. The row names no author, rather than a raw id that links a profile which is gone.
  it('names no author for a post whose author has no account any more', () => {
    const orphan: LinkedPost = { id: 'p9', title: 'Kept after its author left', created_at: '2026-08-01T00:00:00Z', author: { id: 'agent_gone', type: 'agent', display_name: '' } };
    const { container } = render(<PostLinkList posts={[orphan]} showAuthor />);
    expect([...container.querySelectorAll('a')].map((a) => a.getAttribute('href'))).toEqual(['/posts/p9']);
    expect(container.textContent).not.toContain('agent_gone');
    expect(container.querySelector('time')?.textContent).toBe('2026-08-01');
  });

  it('leaves the author out on a profile, where every post is that author\'s', () => {
    const { container } = render(<PostLinkList posts={posts} />);
    const hrefs = [...container.querySelectorAll('a')].map((a) => a.getAttribute('href'));
    expect(hrefs).toEqual(['/posts/p1', '/posts/p2', '/posts/p3']);
    expect(container.textContent).not.toContain('Planner');
  });

  it('marks every link for the click listener, with what was pressed and never which post', () => {
    const { container } = render(<PostLinkList posts={posts} showAuthor />);
    const marks = [...container.querySelectorAll('a')].map((a) => [
      a.getAttribute('data-track'),
      a.getAttribute('data-track-item'),
      a.getAttribute('data-track-location'),
    ]);
    expect(marks).toEqual([
      ['nav', 'post', 'page'],
      ['nav', 'post_author', 'page'],
      ['nav', 'post', 'page'],
      ['nav', 'post_author', 'page'],
      ['nav', 'post', 'page'],
    ]);
  });

  // A row's target is a page rendered on demand, with no loading state: the router has
  // nothing to prefetch for it but would still ask the server once per link in view. A page
  // of fifty rows is a hundred such requests for every reader, and for a crawler that
  // renders the page. The links stay ordinary links; they are followed on click.
  it('asks the router not to prefetch its links', () => {
    const { container } = render(<PostLinkList posts={posts} showAuthor />);
    const links = [...container.querySelectorAll('a')];
    expect(links).toHaveLength(5);
    for (const link of links) expect(link.getAttribute('data-prefetch'), link.outerHTML).toBe('false');
  });

  it('renders nothing for no posts', () => {
    const { container } = render(<PostLinkList posts={[]} showAuthor />);
    expect(container.innerHTML).toBe('');
  });
});
