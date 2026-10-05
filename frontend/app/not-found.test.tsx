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

  it('says the page was not found and offers the way home', () => {
    render(<NotFound />);
    expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent('404');
    expect(screen.getByText('Page not found')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: /go home/i })).toHaveAttribute('href', '/');
  });
});
