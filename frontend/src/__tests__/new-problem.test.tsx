/**
 * Tests for New Problem Page
 * TDD approach: Tests written FIRST per CLAUDE.md Golden Rules
 * Per PRD requirements (lines 479-482):
 *   - Create /new/problem page
 *   - New problem: form fields (title, description, tags, success_criteria)
 *   - New problem: preview (markdown)
 *   - New problem: submit (POST to /v1/problems, redirect)
 */

import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import userEvent from '@testing-library/user-event';

// Mock marked for markdown rendering
jest.mock('marked', () => ({
  marked: jest.fn((text: string) => `<p>${text}</p>`),
}));

// Mock next/navigation
const mockPush = jest.fn();
const mockRouterBack = jest.fn();
jest.mock('next/navigation', () => ({
  useRouter: () => ({
    push: mockPush,
    replace: jest.fn(),
    back: mockRouterBack,
    prefetch: jest.fn(),
  }),
}));

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

// Mock useAuth hook
const mockUser = { id: 'user-1', username: 'testuser', display_name: 'Test User' };
let mockIsAuthenticated = true;
let mockIsLoading = false;

jest.mock('@/hooks/useAuth', () => ({
  useAuth: () => ({
    user: mockIsAuthenticated ? mockUser : null,
    isLoading: mockIsLoading,
    login: jest.fn(),
    logout: jest.fn(),
  }),
}));

// Mock the API module
const mockApiPost = jest.fn();
jest.mock('@/lib/api', () => ({
  api: {
    get: jest.fn(),
    post: (...args: unknown[]) => mockApiPost(...args),
  },
  ApiError: class MockApiError extends Error {
    constructor(
      public status: number,
      public code: string,
      message: string,
      public details?: Record<string, unknown>
    ) {
      super(message);
    }
  },
  __esModule: true,
}));

// Import the ApiError to use in tests
import { ApiError } from '@/lib/api';

// Import component after mocks
import NewProblemPage from '../app/new/problem/page';

describe('New Problem Page', () => {
  beforeEach(() => {
    jest.clearAllMocks();
    mockIsAuthenticated = true;
    mockIsLoading = false;
    mockApiPost.mockResolvedValue({
      id: 'new-problem-123',
      type: 'problem',
      title: 'Test Problem',
    });
  });

  describe('Basic Structure', () => {
    it('renders the page', () => {
      render(<NewProblemPage />);
      expect(screen.getByRole('main')).toBeInTheDocument();
    });

    it('displays page heading', () => {
      render(<NewProblemPage />);
      expect(
        screen.getByRole('heading', { name: /new problem/i })
      ).toBeInTheDocument();
    });

    it('has form element', () => {
      render(<NewProblemPage />);
      expect(screen.getByRole('form')).toBeInTheDocument();
    });
  });

  describe('Authentication', () => {
    it('shows loading state while checking auth', () => {
      mockIsLoading = true;
      render(<NewProblemPage />);
      expect(screen.getByText(/loading/i)).toBeInTheDocument();
    });

    it('redirects to login when not authenticated', async () => {
      mockIsAuthenticated = false;
      mockIsLoading = false;
      render(<NewProblemPage />);

      await waitFor(() => {
        expect(mockPush).toHaveBeenCalledWith('/login?redirect=/new/problem');
      });
    });

    it('renders form when authenticated', () => {
      render(<NewProblemPage />);
      expect(screen.getByRole('form')).toBeInTheDocument();
    });
  });

  describe('Form Fields', () => {
    it('has title input', () => {
      render(<NewProblemPage />);
      expect(screen.getByLabelText(/title/i)).toBeInTheDocument();
    });

    it('title input is required', () => {
      render(<NewProblemPage />);
      const titleInput = screen.getByLabelText(/title/i);
      expect(titleInput).toBeRequired();
    });

    it('title input has max length hint', () => {
      render(<NewProblemPage />);
      expect(screen.getByText(/200 characters/i)).toBeInTheDocument();
    });

    it('has description textarea', () => {
      render(<NewProblemPage />);
      expect(screen.getByLabelText(/description/i)).toBeInTheDocument();
    });

    it('description is required', () => {
      render(<NewProblemPage />);
      const descInput = screen.getByLabelText(/description/i);
      expect(descInput).toBeRequired();
    });

    it('has tags input', () => {
      render(<NewProblemPage />);
      expect(screen.getByLabelText(/tags/i)).toBeInTheDocument();
    });

    it('tags input shows max hint', () => {
      render(<NewProblemPage />);
      expect(screen.getByText(/up to 5 tags/i)).toBeInTheDocument();
    });

    it('has success criteria input', () => {
      render(<NewProblemPage />);
      expect(screen.getByLabelText(/success criteria/i)).toBeInTheDocument();
    });

    it('has difficulty/weight select', () => {
      render(<NewProblemPage />);
      expect(screen.getByLabelText(/difficulty/i)).toBeInTheDocument();
    });

    it('difficulty select has options 1-5', () => {
      render(<NewProblemPage />);
      const select = screen.getByLabelText(/difficulty/i);
      expect(select).toBeInTheDocument();

      // Check for difficulty level options
      expect(screen.getByRole('option', { name: /1/i })).toBeInTheDocument();
      expect(screen.getByRole('option', { name: /5/i })).toBeInTheDocument();
    });
  });

  describe('Form Interactions', () => {
    it('allows typing in title field', async () => {
      const user = userEvent.setup();
      render(<NewProblemPage />);

      const titleInput = screen.getByLabelText(/title/i);
      await user.type(titleInput, 'Test Problem Title');

      expect(titleInput).toHaveValue('Test Problem Title');
    });

    it('allows typing in description field', async () => {
      const user = userEvent.setup();
      render(<NewProblemPage />);

      const descInput = screen.getByLabelText(/description/i);
      await user.type(descInput, 'This is a test description');

      expect(descInput).toHaveValue('This is a test description');
    });

    it('allows entering tags', async () => {
      const user = userEvent.setup();
      render(<NewProblemPage />);

      const tagsInput = screen.getByLabelText(/tags/i);
      await user.type(tagsInput, 'golang, postgresql');

      expect(tagsInput).toHaveValue('golang, postgresql');
    });

    it('allows adding success criteria', async () => {
      const user = userEvent.setup();
      render(<NewProblemPage />);

      const criteriaInput = screen.getByLabelText(/success criteria/i);
      await user.type(criteriaInput, 'Tests pass consistently');

      expect(criteriaInput).toHaveValue('Tests pass consistently');
    });
  });

  describe('Preview Tab', () => {
    it('has preview tab button', () => {
      render(<NewProblemPage />);
      expect(
        screen.getByRole('tab', { name: /preview/i })
      ).toBeInTheDocument();
    });

    it('has write/edit tab button', () => {
      render(<NewProblemPage />);
      expect(
        screen.getByRole('tab', { name: /write|edit/i })
      ).toBeInTheDocument();
    });

    it('shows form by default (Write tab active)', () => {
      render(<NewProblemPage />);
      const writeTab = screen.getByRole('tab', { name: /write|edit/i });
      expect(writeTab).toHaveAttribute('aria-selected', 'true');
    });

    it('switches to preview when Preview tab clicked', async () => {
      const user = userEvent.setup();
      render(<NewProblemPage />);

      // Type some content first
      const descInput = screen.getByLabelText(/description/i);
      await user.type(descInput, '# Hello World\n\nThis is **bold** text.');

      // Click preview tab
      const previewTab = screen.getByRole('tab', { name: /preview/i });
      await user.click(previewTab);

      // Preview should be shown
      expect(previewTab).toHaveAttribute('aria-selected', 'true');
    });

    it('renders markdown in preview', async () => {
      const user = userEvent.setup();
      render(<NewProblemPage />);

      // Type markdown content
      const descInput = screen.getByLabelText(/description/i);
      await user.type(descInput, '# Test Heading');

      // Switch to preview
      const previewTab = screen.getByRole('tab', { name: /preview/i });
      await user.click(previewTab);

      // Should render as markdown (h1)
      await waitFor(() => {
        const previewArea = screen.getByTestId('preview-area');
        expect(previewArea).toBeInTheDocument();
      });
    });
  });

  describe('Form Submission', () => {
    it('has submit button', () => {
      render(<NewProblemPage />);
      expect(
        screen.getByRole('button', { name: /create problem|submit/i })
      ).toBeInTheDocument();
    });

    it('has cancel button', () => {
      render(<NewProblemPage />);
      expect(
        screen.getByRole('button', { name: /cancel/i })
      ).toBeInTheDocument();
    });

    it('cancel button goes back', async () => {
      const user = userEvent.setup();
      render(<NewProblemPage />);

      const cancelBtn = screen.getByRole('button', { name: /cancel/i });
      await user.click(cancelBtn);

      expect(mockRouterBack).toHaveBeenCalled();
    });

    it('submits form with correct data', async () => {
      const user = userEvent.setup();
      render(<NewProblemPage />);

      // Fill in the form
      await user.type(screen.getByLabelText(/title/i), 'Test Problem');
      await user.type(
        screen.getByLabelText(/description/i),
        'This is a test problem description that is long enough to meet the minimum requirements.'
      );
      await user.type(screen.getByLabelText(/tags/i), 'test, golang');
      await user.type(
        screen.getByLabelText(/success criteria/i),
        'All tests pass'
      );

      // Submit
      const submitBtn = screen.getByRole('button', {
        name: /create problem|submit/i,
      });
      await user.click(submitBtn);

      await waitFor(() => {
        expect(mockApiPost).toHaveBeenCalledWith(
          '/v1/problems',
          expect.objectContaining({
            type: 'problem',
            title: 'Test Problem',
            description: expect.stringContaining('test problem description'),
            tags: ['test', 'golang'],
          })
        );
      });
    });

    it('redirects to created post on success', async () => {
      const user = userEvent.setup();
      mockApiPost.mockResolvedValue({ id: 'created-123' });

      render(<NewProblemPage />);

      // Fill minimum required fields
      await user.type(screen.getByLabelText(/title/i), 'Test Problem');
      await user.type(
        screen.getByLabelText(/description/i),
        'A sufficiently long description for the problem that meets minimum length requirements.'
      );

      // Submit
      const submitBtn = screen.getByRole('button', {
        name: /create problem|submit/i,
      });
      await user.click(submitBtn);

      await waitFor(() => {
        expect(mockPush).toHaveBeenCalledWith('/posts/created-123');
      });
    });

    it('shows loading state during submission', async () => {
      const user = userEvent.setup();
      // Make API slow
      mockApiPost.mockImplementation(
        () => new Promise((resolve) => setTimeout(() => resolve({ id: '123' }), 1000))
      );

      render(<NewProblemPage />);

      // Fill form with valid values (title must be >= 10 chars)
      await user.type(screen.getByLabelText(/title/i), 'Valid Test Title For Problem');
      await user.type(
        screen.getByLabelText(/description/i),
        'A sufficiently long description for the problem that meets minimum length requirements.'
      );

      // Submit
      const submitBtn = screen.getByRole('button', {
        name: /create problem|submit/i,
      });
      await user.click(submitBtn);

      // Should show loading
      expect(screen.getByText(/creating|submitting/i)).toBeInTheDocument();
    });

    it('disables submit button during submission', async () => {
      const user = userEvent.setup();
      mockApiPost.mockImplementation(
        () => new Promise((resolve) => setTimeout(() => resolve({ id: '123' }), 1000))
      );

      render(<NewProblemPage />);

      // Fill form with valid values (title must be >= 10 chars)
      await user.type(screen.getByLabelText(/title/i), 'Valid Test Title For Problem');
      await user.type(
        screen.getByLabelText(/description/i),
        'A sufficiently long description for the problem that meets minimum length requirements.'
      );

      const submitBtn = screen.getByRole('button', {
        name: /create problem|submit/i,
      });
      await user.click(submitBtn);

      expect(submitBtn).toBeDisabled();
    });
  });

  describe('Validation', () => {
    it('shows error when title too short', async () => {
      const user = userEvent.setup();
      render(<NewProblemPage />);

      // Enter short title
      await user.type(screen.getByLabelText(/title/i), 'Hi');
      await user.type(
        screen.getByLabelText(/description/i),
        'A valid description that meets the minimum length requirements.'
      );

      const submitBtn = screen.getByRole('button', {
        name: /create problem|submit/i,
      });
      await user.click(submitBtn);

      await waitFor(() => {
        expect(screen.getByText(/at least 10 characters/i)).toBeInTheDocument();
      });
    });

    it('shows error when description too short', async () => {
      const user = userEvent.setup();
      render(<NewProblemPage />);

      await user.type(screen.getByLabelText(/title/i), 'Valid Title Here');
      await user.type(screen.getByLabelText(/description/i), 'Short');

      const submitBtn = screen.getByRole('button', {
        name: /create problem|submit/i,
      });
      await user.click(submitBtn);

      await waitFor(() => {
        expect(screen.getByText(/at least 50 characters/i)).toBeInTheDocument();
      });
    });

    it('shows error when more than 5 tags', async () => {
      const user = userEvent.setup();
      render(<NewProblemPage />);

      await user.type(screen.getByLabelText(/title/i), 'Valid Title Here');
      await user.type(
        screen.getByLabelText(/description/i),
        'A valid description that meets the minimum length requirements.'
      );
      await user.type(
        screen.getByLabelText(/tags/i),
        'one, two, three, four, five, six'
      );

      const submitBtn = screen.getByRole('button', {
        name: /create problem|submit/i,
      });
      await user.click(submitBtn);

      await waitFor(() => {
        expect(screen.getByText(/maximum 5 tags/i)).toBeInTheDocument();
      });
    });
  });

  describe('Error Handling', () => {
    it('displays API error message', async () => {
      const user = userEvent.setup();
      mockApiPost.mockRejectedValue(
        new ApiError(400, 'VALIDATION_ERROR', 'Title already exists')
      );

      render(<NewProblemPage />);

      await user.type(screen.getByLabelText(/title/i), 'Valid Title Here');
      await user.type(
        screen.getByLabelText(/description/i),
        'A valid description that meets the minimum length requirements.'
      );

      const submitBtn = screen.getByRole('button', {
        name: /create problem|submit/i,
      });
      await user.click(submitBtn);

      await waitFor(() => {
        expect(screen.getByText(/title already exists/i)).toBeInTheDocument();
      });
    });

    it('displays generic error for network failure', async () => {
      const user = userEvent.setup();
      mockApiPost.mockRejectedValue(new Error('Network error'));

      render(<NewProblemPage />);

      await user.type(screen.getByLabelText(/title/i), 'Valid Title Here');
      await user.type(
        screen.getByLabelText(/description/i),
        'A valid description that meets the minimum length requirements.'
      );

      const submitBtn = screen.getByRole('button', {
        name: /create problem|submit/i,
      });
      await user.click(submitBtn);

      await waitFor(() => {
        expect(screen.getByText(/failed to create|error/i)).toBeInTheDocument();
      });
    });

    it('can dismiss error message', async () => {
      const user = userEvent.setup();
      mockApiPost.mockRejectedValue(
        new ApiError(400, 'VALIDATION_ERROR', 'Some error')
      );

      render(<NewProblemPage />);

      await user.type(screen.getByLabelText(/title/i), 'Valid Title Here');
      await user.type(
        screen.getByLabelText(/description/i),
        'A valid description that meets the minimum length requirements.'
      );

      const submitBtn = screen.getByRole('button', {
        name: /create problem|submit/i,
      });
      await user.click(submitBtn);

      // Wait for error to appear
      await waitFor(() => {
        expect(screen.getByRole('alert')).toBeInTheDocument();
      });

      // Dismiss error
      const dismissBtn = screen.getByRole('button', {
        name: /dismiss|close|×/i,
      });
      await user.click(dismissBtn);

      expect(screen.queryByRole('alert')).not.toBeInTheDocument();
    });
  });

  describe('Accessibility', () => {
    it('has proper heading hierarchy', () => {
      render(<NewProblemPage />);
      expect(screen.getByRole('heading', { level: 1 })).toBeInTheDocument();
    });

    it('form fields have labels', () => {
      render(<NewProblemPage />);

      expect(screen.getByLabelText(/title/i)).toBeInTheDocument();
      expect(screen.getByLabelText(/description/i)).toBeInTheDocument();
      expect(screen.getByLabelText(/tags/i)).toBeInTheDocument();
    });

    it('error messages are announced', async () => {
      const user = userEvent.setup();
      mockApiPost.mockRejectedValue(
        new ApiError(400, 'VALIDATION_ERROR', 'Error occurred')
      );

      render(<NewProblemPage />);

      await user.type(screen.getByLabelText(/title/i), 'Valid Title Here');
      await user.type(
        screen.getByLabelText(/description/i),
        'A valid description that meets the minimum length requirements.'
      );

      const submitBtn = screen.getByRole('button', {
        name: /create problem|submit/i,
      });
      await user.click(submitBtn);

      await waitFor(() => {
        expect(screen.getByRole('alert')).toBeInTheDocument();
      });
    });

    it('submit button has accessible name', () => {
      render(<NewProblemPage />);

      const submitBtn = screen.getByRole('button', {
        name: /create problem|submit/i,
      });
      expect(submitBtn).toHaveAccessibleName();
    });
  });

  describe('Responsive Design', () => {
    it('has container with max-width', () => {
      render(<NewProblemPage />);

      const container = screen.getByTestId('new-problem-container');
      expect(container).toHaveClass('max-w-3xl');
    });
  });
});
