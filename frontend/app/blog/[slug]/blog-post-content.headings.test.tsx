import { render } from '@testing-library/react';
import { describe, it, expect, vi } from 'vitest';
import { BlogPostContent } from './blog-post-content';

// The blog page renders its body with the real Markdown renderer here (page.test.tsx mocks it):
// a body that starts with "#" must not give the page a second <h1>.

vi.mock('next/navigation', () => ({
  useParams: () => ({ slug: 'test-blog-post' }),
  notFound: vi.fn(),
}));
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
  slug: 'test-blog-post',
  title: 'Test Blog Post Title',
  excerpt: 'A test excerpt',
  body: '# Hello World\n\nThis is the body.\n\n## A section',
  tags: ['golang'],
  author: { name: 'Alice Developer', type: 'human' as const },
  readTime: '5 min read',
  publishedAt: 'Feb 15, 2026',
  voteScore: 1,
  viewCount: 2,
  userVote: null as 'up' | 'down' | null,
};

describe('BlogPostContent headings', () => {
  it('keeps the post title as the page\'s only h1 when the body starts with "#"', () => {
    const { container } = render(<BlogPostContent post={post} />);
    const h1s = container.querySelectorAll('h1');
    expect(h1s).toHaveLength(1);
    expect(h1s[0]).toHaveTextContent('Test Blog Post Title');
    expect(container.querySelector('h2')).toHaveTextContent('Hello World');
  });
});
