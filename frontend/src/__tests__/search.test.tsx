/**
 * Tests for Search Page component
 * TDD approach: RED -> GREEN -> REFACTOR
 * Requirements from PRD lines 460-465:
 * - Create /search page
 * - Search: input field (bound to URL query param)
 * - Search: filters (type, status, sort dropdowns)
 * - Search: results list (PostCards)
 * - Search: pagination controls
 * - Search: loading/empty states
 */

import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import { act } from 'react';
import userEvent from '@testing-library/user-event';

// Mock next/navigation
const mockPush = jest.fn();
const mockSearchParams = new URLSearchParams();
jest.mock('next/navigation', () => ({
  useRouter: () => ({ push: mockPush, replace: jest.fn(), back: jest.fn() }),
  useSearchParams: () => mockSearchParams,
  usePathname: () => '/search',
}));

// Mock next/link
jest.mock('next/link', () => {
  return function MockLink({ children, href }: { children: React.ReactNode; href: string }) {
    return <a href={href}>{children}</a>;
  };
});

// Mock the API module
const mockApiGet = jest.fn();
jest.mock('@/lib/api', () => ({
  api: { get: (...args: unknown[]) => mockApiGet(...args) },
  __esModule: true,
}));

// Import component after mocks
import SearchPage from '../app/search/page';

// Test data
const mockSearchResults = {
  data: [
    {
      id: 'post-1',
      type: 'problem',
      title: 'Race condition in async/await with PostgreSQL',
      snippet: '...encountering a <mark>race condition</mark> when multiple <mark>async</mark>...',
      tags: ['postgresql', 'async', 'concurrency'],
      status: 'solved',
      author: { id: 'user-1', type: 'human', display_name: 'John Doe' },
      score: 0.95,
      votes: 42,
      created_at: '2026-01-15T10:00:00Z',
    },
    {
      id: 'post-2',
      type: 'question',
      title: 'How to handle async errors in Go?',
      snippet: 'What is the best practice for handling <mark>async</mark> errors...',
      tags: ['go', 'error-handling'],
      status: 'answered',
      author: { id: 'agent-1', type: 'agent', display_name: 'Claude' },
      score: 0.88,
      votes: 28,
      created_at: '2026-01-14T15:00:00Z',
    },
    {
      id: 'post-3',
      type: 'idea',
      title: 'Idea: Better async debugging tools',
      snippet: 'I think we need better <mark>async</mark> debugging tools...',
      tags: ['debugging', 'tooling'],
      status: 'active',
      author: { id: 'user-2', type: 'human', display_name: 'Jane Smith' },
      score: 0.75,
      votes: 15,
      created_at: '2026-01-13T09:00:00Z',
    },
  ],
  meta: { query: 'async', total: 127, page: 1, per_page: 20, has_more: true, took_ms: 23 },
};

const emptySearchResults = {
  data: [],
  meta: { query: 'nonexistent', total: 0, page: 1, per_page: 20, has_more: false, took_ms: 5 },
};

// Helper to set up URL search params
function setupSearchParams(params: Record<string, string>) {
  Array.from(mockSearchParams.keys()).forEach((key) => mockSearchParams.delete(key));
  Object.entries(params).forEach(([key, value]) => mockSearchParams.set(key, value));
}

describe('SearchPage', () => {
  beforeEach(() => {
    jest.clearAllMocks();
    setupSearchParams({});
  });

  describe('Basic structure', () => {
    it('renders main container and heading', async () => {
      mockApiGet.mockResolvedValue({ data: [], meta: {} });
      await act(async () => {
        render(<SearchPage />);
      });
      expect(screen.getByRole('main')).toBeInTheDocument();
      await waitFor(() => {
        expect(screen.getByRole('heading', { level: 1 })).toBeInTheDocument();
      });
    });
  });

  describe('Search input', () => {
    it('renders search input and binds to URL query param', async () => {
      setupSearchParams({ q: 'async' });
      mockApiGet.mockResolvedValue(mockSearchResults);
      await act(async () => {
        render(<SearchPage />);
      });
      await waitFor(() => {
        const searchInput = screen.getByRole('searchbox') as HTMLInputElement;
        expect(searchInput).toBeInTheDocument();
        expect(searchInput.value).toBe('async');
      });
    });

    it('updates URL when search is submitted', async () => {
      mockApiGet.mockResolvedValue({ data: [], meta: {} });
      const user = userEvent.setup();
      await act(async () => {
        render(<SearchPage />);
      });
      await waitFor(() => {
        expect(screen.getByRole('searchbox')).toBeInTheDocument();
      });
      const searchInput = screen.getByRole('searchbox');
      await user.clear(searchInput);
      await user.type(searchInput, 'postgres');
      const form = searchInput.closest('form');
      if (form) fireEvent.submit(form);
      await waitFor(() => {
        expect(mockPush).toHaveBeenCalledWith(expect.stringContaining('q=postgres'));
      });
    });

    it('has search button and clear button when query exists', async () => {
      setupSearchParams({ q: 'test query' });
      mockApiGet.mockResolvedValue(mockSearchResults);
      await act(async () => {
        render(<SearchPage />);
      });
      await waitFor(() => {
        expect(screen.getByRole('button', { name: /^search$/i })).toBeInTheDocument();
        expect(screen.getByRole('button', { name: /clear search/i })).toBeInTheDocument();
      });
    });
  });

  describe('Filters', () => {
    it('renders type, status, and sort filter dropdowns with correct options', async () => {
      mockApiGet.mockResolvedValue({ data: [], meta: {} });
      await act(async () => {
        render(<SearchPage />);
      });
      await waitFor(() => {
        const typeFilter = screen.getByLabelText(/type/i) as HTMLSelectElement;
        const statusFilter = screen.getByLabelText(/status/i) as HTMLSelectElement;
        const sortFilter = screen.getByLabelText(/sort/i) as HTMLSelectElement;

        expect(typeFilter).toBeInTheDocument();
        expect(statusFilter).toBeInTheDocument();
        expect(sortFilter).toBeInTheDocument();

        const typeOptions = Array.from(typeFilter.options).map((opt) => opt.value);
        expect(typeOptions).toContain('all');
        expect(typeOptions).toContain('problem');
        expect(typeOptions).toContain('question');
        expect(typeOptions).toContain('idea');

        const statusOptions = Array.from(statusFilter.options).map((opt) => opt.value);
        expect(statusOptions).toContain('all');
        expect(statusOptions).toContain('open');
        expect(statusOptions).toContain('solved');

        const sortOptions = Array.from(sortFilter.options).map((opt) => opt.value);
        expect(sortOptions).toContain('relevance');
        expect(sortOptions).toContain('newest');
        expect(sortOptions).toContain('votes');
      });
    });

    it('filter changes update URL params', async () => {
      setupSearchParams({ q: 'async' });
      mockApiGet.mockResolvedValue(mockSearchResults);
      const user = userEvent.setup();
      await act(async () => {
        render(<SearchPage />);
      });
      await waitFor(() => {
        expect(screen.getByLabelText(/type/i)).toBeInTheDocument();
      });
      const typeFilter = screen.getByLabelText(/type/i);
      await user.selectOptions(typeFilter, 'problem');
      await waitFor(() => {
        expect(mockPush).toHaveBeenCalledWith(expect.stringContaining('type=problem'));
      });
    });

    it('filters are initialized from URL params', async () => {
      setupSearchParams({ q: 'async', type: 'question', sort: 'newest' });
      mockApiGet.mockResolvedValue(mockSearchResults);
      await act(async () => {
        render(<SearchPage />);
      });
      await waitFor(() => {
        const typeFilter = screen.getByLabelText(/type/i) as HTMLSelectElement;
        const sortFilter = screen.getByLabelText(/sort/i) as HTMLSelectElement;
        expect(typeFilter.value).toBe('question');
        expect(sortFilter.value).toBe('newest');
      });
    });
  });

  describe('Search results list', () => {
    it('fetches and displays search results from /v1/search', async () => {
      setupSearchParams({ q: 'async' });
      mockApiGet.mockResolvedValue(mockSearchResults);
      await act(async () => {
        render(<SearchPage />);
      });
      await waitFor(() => {
        expect(mockApiGet).toHaveBeenCalledWith(
          '/v1/search',
          expect.objectContaining({ q: 'async' }),
          expect.any(Object)
        );
        expect(
          screen.getByText('Race condition in async/await with PostgreSQL')
        ).toBeInTheDocument();
        expect(screen.getByText('How to handle async errors in Go?')).toBeInTheDocument();
        expect(screen.getByText(/127 results/i)).toBeInTheDocument();
      });
    });

    it('displays type badges, author info, and vote counts', async () => {
      setupSearchParams({ q: 'async' });
      mockApiGet.mockResolvedValue(mockSearchResults);
      await act(async () => {
        render(<SearchPage />);
      });
      await waitFor(() => {
        expect(screen.getByText('problem')).toBeInTheDocument();
        expect(screen.getByText('question')).toBeInTheDocument();
        expect(screen.getByText('idea')).toBeInTheDocument();
        expect(screen.getByText('John Doe')).toBeInTheDocument();
        expect(screen.getByText('Claude')).toBeInTheDocument();
        expect(screen.getByText(/42/)).toBeInTheDocument();
        expect(screen.getByText(/28/)).toBeInTheDocument();
      });
    });

    it('links results to post detail pages and displays tags', async () => {
      setupSearchParams({ q: 'async' });
      mockApiGet.mockResolvedValue(mockSearchResults);
      await act(async () => {
        render(<SearchPage />);
      });
      await waitFor(() => {
        const postLink = screen.getByRole('link', { name: /race condition/i });
        expect(postLink).toHaveAttribute('href', '/posts/post-1');
        expect(screen.getByText('postgresql')).toBeInTheDocument();
        expect(screen.getByText('concurrency')).toBeInTheDocument();
      });
    });

    it('displays snippet with highlighted search terms', async () => {
      setupSearchParams({ q: 'async' });
      mockApiGet.mockResolvedValue(mockSearchResults);
      await act(async () => {
        render(<SearchPage />);
      });
      await waitFor(() => {
        const snippetElements = document.querySelectorAll('.line-clamp-2');
        expect(snippetElements.length).toBeGreaterThan(0);
        const hasRaceCondition = Array.from(snippetElements).some((el) =>
          el.textContent?.toLowerCase().includes('race condition')
        );
        expect(hasRaceCondition).toBe(true);
      });
    });
  });

  describe('Pagination', () => {
    it('displays page controls when there are multiple pages', async () => {
      setupSearchParams({ q: 'async' });
      mockApiGet.mockResolvedValue(mockSearchResults);
      await act(async () => {
        render(<SearchPage />);
      });
      await waitFor(() => {
        expect(screen.getByRole('navigation', { name: /pagination/i })).toBeInTheDocument();
      });
    });

    it('displays current page number', async () => {
      setupSearchParams({ q: 'async', page: '2' });
      mockApiGet.mockResolvedValue({
        ...mockSearchResults,
        meta: { ...mockSearchResults.meta, page: 2 },
      });
      await act(async () => {
        render(<SearchPage />);
      });
      await waitFor(() => {
        expect(screen.getByText(/page 2/i)).toBeInTheDocument();
      });
    });

    it('clicking next page updates URL', async () => {
      setupSearchParams({ q: 'async' });
      mockApiGet.mockResolvedValue(mockSearchResults);
      const user = userEvent.setup();
      await act(async () => {
        render(<SearchPage />);
      });
      await waitFor(() => {
        expect(screen.getByRole('button', { name: /next/i })).toBeInTheDocument();
      });
      await user.click(screen.getByRole('button', { name: /next/i }));
      await waitFor(() => {
        expect(mockPush).toHaveBeenCalledWith(expect.stringContaining('page=2'));
      });
    });

    it('clicking previous page updates URL', async () => {
      setupSearchParams({ q: 'async', page: '2' });
      mockApiGet.mockResolvedValue({
        ...mockSearchResults,
        meta: { ...mockSearchResults.meta, page: 2, has_more: true },
      });
      const user = userEvent.setup();
      await act(async () => {
        render(<SearchPage />);
      });
      await waitFor(() => {
        const prevButton = screen.getByRole('button', { name: /previous/i });
        expect(prevButton).not.toBeDisabled();
      });
      await user.click(screen.getByRole('button', { name: /previous/i }));
      await waitFor(() => {
        expect(mockPush).toHaveBeenCalledWith(expect.stringContaining('q=async'));
      });
    });

    it('disables prev on first page and next on last page', async () => {
      setupSearchParams({ q: 'async' });
      mockApiGet.mockResolvedValue({
        ...mockSearchResults,
        meta: { ...mockSearchResults.meta, has_more: false },
      });
      await act(async () => {
        render(<SearchPage />);
      });
      await waitFor(() => {
        expect(screen.getByRole('button', { name: /previous/i })).toBeDisabled();
        expect(screen.getByRole('button', { name: /next/i })).toBeDisabled();
      });
    });

    it('hides pagination when no results', async () => {
      setupSearchParams({ q: 'nonexistent' });
      mockApiGet.mockResolvedValue(emptySearchResults);
      await act(async () => {
        render(<SearchPage />);
      });
      await waitFor(() => {
        expect(screen.queryByRole('navigation', { name: /pagination/i })).not.toBeInTheDocument();
      });
    });
  });

  describe('Loading state', () => {
    it('shows loading skeleton while fetching results', async () => {
      setupSearchParams({ q: 'async' });
      mockApiGet.mockImplementation(() => new Promise(() => {}));
      render(<SearchPage />);
      expect(screen.getByTestId('search-loading')).toBeInTheDocument();
      expect(screen.getAllByTestId('result-skeleton').length).toBeGreaterThan(0);
    });

    it('removes loading state after results load', async () => {
      setupSearchParams({ q: 'async' });
      mockApiGet.mockResolvedValue(mockSearchResults);
      await act(async () => {
        render(<SearchPage />);
      });
      await waitFor(() => {
        expect(screen.queryByTestId('search-loading')).not.toBeInTheDocument();
      });
    });
  });

  describe('Empty state', () => {
    it('shows empty state when no query', async () => {
      mockApiGet.mockResolvedValue({ data: [], meta: {} });
      await act(async () => {
        render(<SearchPage />);
      });
      await waitFor(() => {
        expect(screen.getByText(/enter a search term/i)).toBeInTheDocument();
      });
    });

    it('shows no results message with query and suggestions', async () => {
      setupSearchParams({ q: 'nonexistent' });
      mockApiGet.mockResolvedValue(emptySearchResults);
      await act(async () => {
        render(<SearchPage />);
      });
      await waitFor(() => {
        expect(screen.getByText(/no results found/i)).toBeInTheDocument();
        expect(document.body.textContent).toContain('nonexistent');
        expect(
          screen.getByText(/try different|try another|modify your search/i)
        ).toBeInTheDocument();
      });
    });
  });

  describe('Error state', () => {
    it('shows error message and retry button when fetch fails', async () => {
      setupSearchParams({ q: 'async' });
      mockApiGet.mockRejectedValue(new Error('Network error'));
      await act(async () => {
        render(<SearchPage />);
      });
      await waitFor(() => {
        expect(screen.getByText(/error|failed|unable/i)).toBeInTheDocument();
        expect(screen.getByRole('button', { name: /retry|try again/i })).toBeInTheDocument();
      });
    });

    it('retries fetch when retry button is clicked', async () => {
      setupSearchParams({ q: 'async' });
      mockApiGet
        .mockRejectedValueOnce(new Error('Network error'))
        .mockResolvedValueOnce(mockSearchResults);
      const user = userEvent.setup();
      await act(async () => {
        render(<SearchPage />);
      });
      await waitFor(() => {
        expect(screen.getByRole('button', { name: /retry|try again/i })).toBeInTheDocument();
      });
      await user.click(screen.getByRole('button', { name: /retry|try again/i }));
      await waitFor(() => {
        expect(mockApiGet).toHaveBeenCalledTimes(2);
      });
    });
  });

  describe('Accessibility', () => {
    it('has proper heading, accessible labels, and semantic structure', async () => {
      setupSearchParams({ q: 'async' });
      mockApiGet.mockResolvedValue(mockSearchResults);
      await act(async () => {
        render(<SearchPage />);
      });
      await waitFor(() => {
        expect(screen.getByRole('heading', { level: 1 })).toBeInTheDocument();
        expect(screen.getByRole('searchbox')).toHaveAccessibleName();
        expect(screen.getByLabelText(/type/i)).toBeInTheDocument();
        expect(screen.getByLabelText(/status/i)).toBeInTheDocument();
        expect(screen.getByLabelText(/sort/i)).toBeInTheDocument();
        expect(screen.getByRole('navigation', { name: /pagination/i })).toBeInTheDocument();
        expect(screen.getByRole('list')).toBeInTheDocument();
        expect(screen.getAllByRole('listitem').length).toBe(3);
      });
    });
  });

  describe('Search query display', () => {
    it('displays search query in heading and search time', async () => {
      setupSearchParams({ q: 'async' });
      mockApiGet.mockResolvedValue(mockSearchResults);
      await act(async () => {
        render(<SearchPage />);
      });
      await waitFor(() => {
        expect(screen.getByText(/results for.*async/i)).toBeInTheDocument();
        expect(screen.getByText(/23\s*ms/i)).toBeInTheDocument();
      });
    });
  });

  describe('Analytics: Track search events', () => {
    // Mock window.plausible to track analytics calls
    let mockPlausible: jest.Mock;

    beforeEach(() => {
      mockPlausible = jest.fn();
      (window as { plausible?: unknown }).plausible = mockPlausible;
    });

    afterEach(() => {
      delete (window as { plausible?: unknown }).plausible;
    });

    it('fires Search event on search submit', async () => {
      setupSearchParams({});
      mockApiGet.mockResolvedValue(mockSearchResults);
      const user = userEvent.setup();

      await act(async () => {
        render(<SearchPage />);
      });
      await waitFor(() => {
        expect(screen.getByRole('searchbox')).toBeInTheDocument();
      });

      const searchInput = screen.getByRole('searchbox');
      await user.clear(searchInput);
      await user.type(searchInput, 'postgres query');
      const form = searchInput.closest('form');
      if (form) fireEvent.submit(form);

      await waitFor(() => {
        expect(mockPlausible).toHaveBeenCalledWith(
          'Search',
          expect.objectContaining({
            props: expect.objectContaining({
              query_length: expect.any(Number),
            }),
          })
        );
      });
    });

    it('includes query length (not query content) in Search event', async () => {
      setupSearchParams({});
      mockApiGet.mockResolvedValue(mockSearchResults);
      const user = userEvent.setup();

      await act(async () => {
        render(<SearchPage />);
      });
      await waitFor(() => {
        expect(screen.getByRole('searchbox')).toBeInTheDocument();
      });

      const searchInput = screen.getByRole('searchbox');
      await user.clear(searchInput);
      await user.type(searchInput, 'async postgres'); // 14 characters
      const form = searchInput.closest('form');
      if (form) fireEvent.submit(form);

      await waitFor(() => {
        const calls = mockPlausible.mock.calls;
        const searchCall = calls.find((call: unknown[]) => call[0] === 'Search');
        expect(searchCall).toBeDefined();
        if (searchCall) {
          const props = searchCall[1]?.props;
          expect(props).toHaveProperty('query_length', 14);
          // Should NOT have query content
          expect(props).not.toHaveProperty('query');
          expect(props).not.toHaveProperty('q');
        }
      });
    });

    it('includes results count in Search event after results load', async () => {
      setupSearchParams({ q: 'async' });
      mockApiGet.mockResolvedValue(mockSearchResults);

      await act(async () => {
        render(<SearchPage />);
      });

      await waitFor(() => {
        expect(
          screen.getByText('Race condition in async/await with PostgreSQL')
        ).toBeInTheDocument();
      });

      // Give time for the analytics event to fire after results load
      await waitFor(() => {
        const calls = mockPlausible.mock.calls;
        const searchCall = calls.find((call: unknown[]) => call[0] === 'Search');
        expect(searchCall).toBeDefined();
        if (searchCall) {
          const props = searchCall[1]?.props;
          expect(props).toHaveProperty('results_count', 127);
        }
      });
    });
  });
});
