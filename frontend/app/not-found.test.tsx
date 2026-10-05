import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';

vi.mock('next/link', () => ({
  default: ({ children, href, ...props }: { children: React.ReactNode; href: string; [key: string]: unknown }) => (
    <a href={href} {...props}>{children}</a>
  ),
}));

import NotFound, { metadata } from './not-found';
import { NOINDEX } from '@/lib/seo/route-policy';

// A page that does not exist was served, and counted in analytics, under the home page's
// title, so a 404 view could not be told from a home view. It now names itself, and
// tells search engines to keep it out of the index.
describe('not-found page', () => {
  it('carries its own title, which the root template turns into "Page not found | Solvr"', () => {
    expect(metadata.title).toBe('Page not found');
  });

  it('is never indexed', () => {
    expect(metadata.robots).toEqual(NOINDEX);
    expect(metadata.alternates?.canonical).toBeUndefined();
  });

  // A missing page under a route layout (/docs/guides/no-such-guide) is rendered with
  // that layout's metadata below it. Without a link preview of its own it showed the
  // layout's: the title "Guides | Solvr" and the address of /docs/guides, on a 404.
  it('states its own link preview, with no address', () => {
    expect(metadata.openGraph).toMatchObject({ title: 'Page not found | Solvr', siteName: 'Solvr' });
    expect(metadata.openGraph).not.toHaveProperty('url');
    expect(metadata.twitter).toMatchObject({ card: 'summary_large_image', title: 'Page not found | Solvr' });
  });

  it('says the page was not found and offers the way home', () => {
    render(<NotFound />);
    expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent('404');
    expect(screen.getByText('Page not found')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: /go home/i })).toHaveAttribute('href', '/');
  });
});
