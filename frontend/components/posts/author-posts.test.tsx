import { describe, it, expect, vi } from 'vitest';
import { render } from '@testing-library/react';
import { renderToStaticMarkup } from 'react-dom/server';

// SPEC.md 27.2: a profile lists its author's indexable posts as plain links in server HTML.
// The profile's own posts load in the browser, which a crawler does not run, so the profile
// linked none of them.

vi.mock('next/link', () => ({
  default: ({ children, href, ...props }: { children: React.ReactNode; href: string; [key: string]: unknown }) => {
    // What a link asks of the router (prefetch) is not an attribute of the element.
    const attributes = { ...props };
    delete attributes.prefetch;
    return <a href={href} {...attributes}>{children}</a>;
  },
}));

import { AuthorPosts } from './author-posts';

const author = { id: 'agent_planner', type: 'agent', display_name: 'Planner' };
const posts = (count: number) =>
  Array.from({ length: count }, (_, i) => ({ id: `p${i + 1}`, title: `Post ${i + 1}`, created_at: '2026-09-01T00:00:00Z', author }));

describe('AuthorPosts', () => {
  it('server-renders each of the author\'s posts as a plain link, under a heading that names the author', () => {
    const html = renderToStaticMarkup(<AuthorPosts name="Planner" posts={posts(3)} total={3} />);
    for (const id of ['p1', 'p2', 'p3']) expect(html).toContain(`href="/posts/${id}"`);
    expect(html).toMatch(/<h2\b[^>]*>Posts by Planner<\/h2>/);
  });

  it('lists the posts and nothing else: no author link on the author\'s own page', () => {
    const { container } = render(<AuthorPosts name="Planner" posts={posts(3)} total={3} />);
    expect([...container.querySelectorAll('a')].map((a) => a.getAttribute('href'))).toEqual(['/posts/p1', '/posts/p2', '/posts/p3']);
  });

  it('says how many posts there are', () => {
    expect(render(<AuthorPosts name="Planner" posts={posts(1)} total={1} />).container.textContent).toContain('1 post.');
    expect(render(<AuthorPosts name="Planner" posts={posts(3)} total={3} />).container.textContent).toContain('3 posts.');
  });

  it('says so when it shows only the newest of more', () => {
    const { container } = render(<AuthorPosts name="Planner" posts={posts(50)} total={123} />);
    expect(container.querySelectorAll('li')).toHaveLength(50);
    expect(container.textContent).toContain('The newest 50 of 123 posts.');
  });

  it('renders nothing for an author with no post to list', () => {
    const { container } = render(<AuthorPosts name="Planner" posts={[]} total={0} />);
    expect(container.innerHTML).toBe('');
  });
});
