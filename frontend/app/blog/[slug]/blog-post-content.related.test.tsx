import { render, screen, within } from '@testing-library/react';
import { describe, it, expect, vi } from 'vitest';
import { BlogPostContent } from './blog-post-content';

// The related blog posts the page read are plain links after the body (SEO audit 2026-11-08).

vi.mock('next/navigation', () => ({ useParams: () => ({ slug: 'a-post' }), notFound: vi.fn() }));
vi.mock('@/components/header', () => ({ Header: () => null }));
vi.mock('@/components/footer', () => ({ Footer: () => null }));
vi.mock('next/link', () => ({
  default: ({ children, href, ...props }: { children: React.ReactNode; href: string; [key: string]: unknown }) => (
    <a href={href} {...props}>{children}</a>
  ),
}));
vi.mock('@/hooks/use-auth', () => ({
  useAuth: () => ({ user: null, isAuthenticated: false, isLoading: false, setShowAuthModal: vi.fn() }),
}));
vi.mock('@/lib/api', () => ({
  api: { voteBlogPost: vi.fn(), recordBlogView: vi.fn().mockResolvedValue(undefined) },
}));

const post = {
  slug: 'a-post', title: 'A post', excerpt: 'E', body: 'Body text.', tags: ['agents'],
  author: { name: 'Helper Bot', type: 'ai' as const }, readTime: '1 min read', publishedAt: '2 days ago',
  voteScore: 1, viewCount: 2,
};

describe('BlogPostContent related posts', () => {
  it('links each related post by its title', () => {
    render(<BlogPostContent post={post} related={[{ slug: 'b', title: 'Post B' }, { slug: 'c', title: 'Post C' }]} />);
    const section = screen.getByRole('heading', { name: /more from the blog/i }).closest('section')!;
    expect(within(section).getAllByRole('link').map((a) => [a.textContent, a.getAttribute('href')])).toEqual([
      ['Post B', '/blog/b'],
      ['Post C', '/blog/c'],
    ]);
  });

  it('shows no such section without related posts', () => {
    render(<BlogPostContent post={post} />);
    expect(screen.queryByRole('heading', { name: /more from the blog/i })).toBeNull();
  });
});
