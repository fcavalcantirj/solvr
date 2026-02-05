/**
 * Tests for Homepage component
 * TDD approach: RED -> GREEN -> REFACTOR
 * Requirements from PRD lines 454-459:
 * - Create homepage /
 * - Homepage: recent activity (fetch /v1/feed)
 * - Homepage: quick stats
 * - Homepage: CTA (Get Started button)
 * - Homepage: loading state
 * - Homepage: error state
 */

import { render, screen, waitFor } from '@testing-library/react';
import { act } from 'react';

// Mock next/link
jest.mock('next/link', () => {
  return function MockLink({
    children,
    href,
  }: {
    children: React.ReactNode;
    href: string;
  }) {
    return <a href={href}>{children}</a>;
  };
});

// Mock the API module
const mockApiGet = jest.fn();
jest.mock('@/lib/api', () => ({
  api: {
    get: (...args: unknown[]) => mockApiGet(...args),
  },
  __esModule: true,
}));

// Import component after mocks
import HomePage from '../app/page';

// Test data
const mockFeedData = {
  data: [
    {
      id: 'post-1',
      type: 'problem',
      title: 'How to fix async race condition',
      snippet: 'I have a race condition in my async code...',
      author: { id: 'user-1', type: 'human', display_name: 'John Doe' },
      tags: ['javascript', 'async'],
      status: 'open',
      votes: 5,
      created_at: '2026-01-30T10:00:00Z',
    },
    {
      id: 'post-2',
      type: 'question',
      title: 'What is the best way to handle errors in Go?',
      snippet: 'Looking for error handling best practices...',
      author: { id: 'agent-1', type: 'agent', display_name: 'Claude' },
      tags: ['go', 'error-handling'],
      status: 'answered',
      votes: 12,
      created_at: '2026-01-29T15:00:00Z',
    },
    {
      id: 'post-3',
      type: 'idea',
      title: 'Proposal for new search algorithm',
      snippet: 'I think we could improve search by...',
      author: { id: 'user-2', type: 'human', display_name: 'Jane Smith' },
      tags: ['search', 'algorithms'],
      status: 'active',
      votes: 8,
      created_at: '2026-01-28T09:00:00Z',
    },
  ],
  meta: {
    total: 150,
    page: 1,
    per_page: 5,
    has_more: true,
  },
};

const mockStatsData = {
  problems: 45,
  questions: 128,
  ideas: 32,
  agents: 89,
  users: 256,
};

// Helper to mock API responses for both feed and stats
function mockSuccessfulApiCalls() {
  mockApiGet.mockImplementation((path: string) => {
    if (path.includes('/feed')) {
      return Promise.resolve(mockFeedData);
    }
    if (path.includes('/stats')) {
      return Promise.resolve(mockStatsData);
    }
    return Promise.resolve({ data: [], meta: {} });
  });
}

describe('HomePage', () => {
  beforeEach(() => {
    jest.clearAllMocks();
  });

  describe('Basic structure', () => {
    it('renders the page container', async () => {
      mockApiGet.mockResolvedValue({ data: [], meta: {} });

      await act(async () => {
        render(<HomePage />);
      });

      // Should have a main element
      expect(screen.getByRole('main')).toBeInTheDocument();
    });

    it('displays the Solvr branding/tagline', async () => {
      mockApiGet.mockResolvedValue({ data: [], meta: {} });

      await act(async () => {
        render(<HomePage />);
      });

      // Wait for content to load
      await waitFor(() => {
        // Should have a headline about Solvr
        const heading = screen.getByRole('heading', { level: 1 });
        expect(heading).toBeInTheDocument();
        // Per SPEC.md Part 4.3: "The Knowledge Base for Humans and AI Agents"
        expect(heading.textContent).toMatch(/knowledge base/i);
      });
    });

    it('displays the hero section', async () => {
      mockApiGet.mockResolvedValue({ data: [], meta: {} });

      await act(async () => {
        render(<HomePage />);
      });

      await waitFor(() => {
        // Per SPEC.md Part 4.3: Hero section with subheadline
        expect(screen.getByText(/developers.*ai.*collaborate/i)).toBeInTheDocument();
      });
    });
  });

  describe('CTA (Call to Action)', () => {
    it('renders Get Started button', async () => {
      mockApiGet.mockResolvedValue({ data: [], meta: {} });

      await act(async () => {
        render(<HomePage />);
      });

      await waitFor(() => {
        const ctaButton = screen.getByRole('link', { name: /get started/i });
        expect(ctaButton).toBeInTheDocument();
      });
    });

    it('Get Started links to /login for unauthenticated users', async () => {
      mockApiGet.mockResolvedValue({ data: [], meta: {} });

      await act(async () => {
        render(<HomePage />);
      });

      await waitFor(() => {
        const ctaButton = screen.getByRole('link', { name: /get started/i });
        expect(ctaButton).toHaveAttribute('href', '/login');
      });
    });

    it('renders secondary CTA for AI agents', async () => {
      mockApiGet.mockResolvedValue({ data: [], meta: {} });

      await act(async () => {
        render(<HomePage />);
      });

      await waitFor(() => {
        // Per SPEC.md Part 4.3: "Connect Your AI Agent" or "API Docs"
        const agentCta = screen.getByRole('link', { name: /api docs/i });
        expect(agentCta).toBeInTheDocument();
      });
    });
  });

  describe('Quick stats', () => {
    it('displays stats section', async () => {
      mockSuccessfulApiCalls();

      await act(async () => {
        render(<HomePage />);
      });

      // Should have a stats section
      await waitFor(() => {
        expect(screen.getByTestId('stats-section')).toBeInTheDocument();
      });
    });

    it('displays problems count', async () => {
      mockSuccessfulApiCalls();

      await act(async () => {
        render(<HomePage />);
      });

      await waitFor(() => {
        const statsSection = screen.getByTestId('stats-section');
        expect(statsSection.textContent).toMatch(/problems/i);
        expect(screen.getByText('45')).toBeInTheDocument();
      });
    });

    it('displays questions count', async () => {
      mockSuccessfulApiCalls();

      await act(async () => {
        render(<HomePage />);
      });

      await waitFor(() => {
        const statsSection = screen.getByTestId('stats-section');
        expect(statsSection.textContent).toMatch(/questions/i);
        expect(screen.getByText('128')).toBeInTheDocument();
      });
    });

    it('displays agents count', async () => {
      mockSuccessfulApiCalls();

      await act(async () => {
        render(<HomePage />);
      });

      await waitFor(() => {
        const statsSection = screen.getByTestId('stats-section');
        expect(statsSection.textContent).toMatch(/agents/i);
        expect(screen.getByText('89')).toBeInTheDocument();
      });
    });
  });

  describe('Recent activity', () => {
    it('fetches recent activity from /v1/feed', async () => {
      mockSuccessfulApiCalls();

      await act(async () => {
        render(<HomePage />);
      });

      await waitFor(() => {
        expect(mockApiGet).toHaveBeenCalledWith(
          '/v1/feed',
          expect.any(Object),
          expect.any(Object)
        );
      });
    });

    it('displays recent activity section', async () => {
      mockSuccessfulApiCalls();

      await act(async () => {
        render(<HomePage />);
      });

      await waitFor(() => {
        expect(screen.getByText(/recent activity/i)).toBeInTheDocument();
      });
    });

    it('displays post titles from feed', async () => {
      mockSuccessfulApiCalls();

      await act(async () => {
        render(<HomePage />);
      });

      await waitFor(() => {
        expect(screen.getByText('How to fix async race condition')).toBeInTheDocument();
        expect(
          screen.getByText('What is the best way to handle errors in Go?')
        ).toBeInTheDocument();
      });
    });

    it('displays post type badges', async () => {
      mockSuccessfulApiCalls();

      await act(async () => {
        render(<HomePage />);
      });

      await waitFor(() => {
        // Should show type badges for each post
        expect(screen.getByText('problem')).toBeInTheDocument();
        expect(screen.getByText('question')).toBeInTheDocument();
        expect(screen.getByText('idea')).toBeInTheDocument();
      });
    });

    it('displays author info for posts', async () => {
      mockSuccessfulApiCalls();

      await act(async () => {
        render(<HomePage />);
      });

      await waitFor(() => {
        expect(screen.getByText('John Doe')).toBeInTheDocument();
        expect(screen.getByText('Claude')).toBeInTheDocument();
      });
    });

    it('links posts to detail pages', async () => {
      mockSuccessfulApiCalls();

      await act(async () => {
        render(<HomePage />);
      });

      await waitFor(() => {
        const postLink = screen.getByRole('link', { name: /async race condition/i });
        expect(postLink).toHaveAttribute('href', '/posts/post-1');
      });
    });

    it('renders View All link to /feed', async () => {
      mockSuccessfulApiCalls();

      await act(async () => {
        render(<HomePage />);
      });

      await waitFor(() => {
        const viewAllLink = screen.getByRole('link', { name: /view all/i });
        expect(viewAllLink).toHaveAttribute('href', '/feed');
      });
    });
  });

  describe('Loading state', () => {
    it('shows loading skeleton while fetching data', async () => {
      // Make the API call hang
      mockApiGet.mockImplementation(() => new Promise(() => {}));

      render(<HomePage />);

      // Should show loading state
      expect(screen.getByTestId('loading-skeleton')).toBeInTheDocument();
    });

    it('shows skeleton elements for stats', async () => {
      mockApiGet.mockImplementation(() => new Promise(() => {}));

      render(<HomePage />);

      // Should show skeleton for stats
      expect(screen.getAllByTestId('stat-skeleton').length).toBeGreaterThan(0);
    });

    it('shows skeleton elements for activity feed', async () => {
      mockApiGet.mockImplementation(() => new Promise(() => {}));

      render(<HomePage />);

      // Should show skeletons for activity items
      expect(screen.getAllByTestId('activity-skeleton').length).toBeGreaterThan(0);
    });

    it('removes loading state after data loads', async () => {
      mockSuccessfulApiCalls();

      await act(async () => {
        render(<HomePage />);
      });

      await waitFor(() => {
        expect(screen.queryByTestId('loading-skeleton')).not.toBeInTheDocument();
      });
    });
  });

  describe('Error state', () => {
    it('shows error message when fetch fails', async () => {
      mockApiGet.mockRejectedValue(new Error('Network error'));

      await act(async () => {
        render(<HomePage />);
      });

      await waitFor(() => {
        expect(screen.getByText(/unable|error|failed/i)).toBeInTheDocument();
      });
    });

    it('shows error message for activity section', async () => {
      mockApiGet.mockRejectedValue(new Error('Failed to load feed'));

      await act(async () => {
        render(<HomePage />);
      });

      await waitFor(() => {
        expect(screen.getByTestId('activity-error')).toBeInTheDocument();
      });
    });

    it('shows retry button on error', async () => {
      mockApiGet.mockRejectedValue(new Error('Network error'));

      await act(async () => {
        render(<HomePage />);
      });

      await waitFor(() => {
        const retryButton = screen.getByRole('button', { name: /retry|try again/i });
        expect(retryButton).toBeInTheDocument();
      });
    });

    it('does not break the page on API error', async () => {
      mockApiGet.mockRejectedValue(new Error('Server error'));

      await act(async () => {
        render(<HomePage />);
      });

      await waitFor(() => {
        // Should still show the hero section and CTA
        expect(screen.getByRole('main')).toBeInTheDocument();
        expect(screen.getByRole('link', { name: /get started/i })).toBeInTheDocument();
      });
    });
  });

  describe('How it works section', () => {
    it('displays how it works section', async () => {
      mockApiGet.mockResolvedValue({ data: [], meta: {} });

      await act(async () => {
        render(<HomePage />);
      });

      await waitFor(() => {
        // Per SPEC.md Part 4.3: How it works
        expect(screen.getByText(/how it works/i)).toBeInTheDocument();
      });
    });

    it('shows step 1: Post problems/questions/ideas', async () => {
      mockApiGet.mockResolvedValue({ data: [], meta: {} });

      await act(async () => {
        render(<HomePage />);
      });

      await waitFor(() => {
        expect(screen.getByText(/post problems|post questions|post ideas/i)).toBeInTheDocument();
      });
    });

    it('shows step 2: Humans and AI collaborate', async () => {
      mockApiGet.mockResolvedValue({ data: [], meta: {} });

      await act(async () => {
        render(<HomePage />);
      });

      await waitFor(() => {
        expect(screen.getByText(/humans and ai collaborate/i)).toBeInTheDocument();
      });
    });

    it('shows step 3: Knowledge accumulates', async () => {
      mockApiGet.mockResolvedValue({ data: [], meta: {} });

      await act(async () => {
        render(<HomePage />);
      });

      await waitFor(() => {
        expect(screen.getByText(/knowledge accumulates/i)).toBeInTheDocument();
      });
    });
  });

  describe('For AI Agents section', () => {
    it('displays For AI Agents section', async () => {
      mockApiGet.mockResolvedValue({ data: [], meta: {} });

      await act(async () => {
        render(<HomePage />);
      });

      await waitFor(() => {
        // Per SPEC.md Part 4.3: "For AI Agents" section
        expect(screen.getByText(/for ai agents/i)).toBeInTheDocument();
      });
    });

    it('links to API documentation', async () => {
      mockApiGet.mockResolvedValue({ data: [], meta: {} });

      await act(async () => {
        render(<HomePage />);
      });

      await waitFor(() => {
        // Find the link containing "API Documentation" text
        const apiLinks = screen.getAllByRole('link');
        const apiDocLink = apiLinks.find(
          (link) =>
            link.textContent?.toLowerCase().includes('api') &&
            link.textContent?.toLowerCase().includes('doc')
        );
        expect(apiDocLink).toBeTruthy();
        expect(apiDocLink).toHaveAttribute('href', expect.stringContaining('/docs'));
      });
    });
  });

  describe('Accessibility', () => {
    it('has proper heading hierarchy', async () => {
      mockApiGet.mockResolvedValue({ data: [], meta: {} });

      await act(async () => {
        render(<HomePage />);
      });

      await waitFor(() => {
        // Should have h1
        const h1 = screen.getByRole('heading', { level: 1 });
        expect(h1).toBeInTheDocument();
      });
    });

    it('has descriptive link text', async () => {
      mockSuccessfulApiCalls();

      await act(async () => {
        render(<HomePage />);
      });

      await waitFor(() => {
        // Links should have meaningful text, not just "click here"
        const links = screen.getAllByRole('link');
        links.forEach((link) => {
          expect(link.textContent).not.toBe('click here');
        });
      });
    });

    it('stat cards have accessible names', async () => {
      mockSuccessfulApiCalls();

      await act(async () => {
        render(<HomePage />);
      });

      await waitFor(() => {
        const statsSection = screen.getByTestId('stats-section');
        expect(statsSection).toHaveAttribute('aria-label');
      });
    });
  });

  describe('Responsive design', () => {
    it('uses responsive container classes', async () => {
      mockApiGet.mockResolvedValue({ data: [], meta: {} });

      await act(async () => {
        render(<HomePage />);
      });

      await waitFor(() => {
        const main = screen.getByRole('main');
        // Should have max-width constraint class
        expect(main.className).toMatch(/max-w-|container/);
      });
    });
  });
});
