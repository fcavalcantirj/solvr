/**
 * Tests for New Question Page
 * TDD approach: Tests written FIRST per CLAUDE.md Golden Rules
 * Per PRD requirement (line 483):
 *   - Create /new/question page
 *   - Add form fields
 *   - Submit to API
 */

import { render, screen, waitFor } from '@testing-library/react';
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
import NewQuestionPage from '../app/new/question/page';

describe('New Question Page', () => {
  beforeEach(() => {
    jest.clearAllMocks();
    mockIsAuthenticated = true;
    mockIsLoading = false;
    mockApiPost.mockResolvedValue({
      id: 'new-question-123',
      type: 'question',
      title: 'Test Question',
    });
  });

  describe('Basic Structure', () => {
    it('renders the page', () => {
      render(<NewQuestionPage />);
      expect(screen.getByRole('main')).toBeInTheDocument();
    });

    it('displays page heading', () => {
      render(<NewQuestionPage />);
      expect(
        screen.getByRole('heading', { name: /new question/i })
      ).toBeInTheDocument();
    });

    it('has form element', () => {
      render(<NewQuestionPage />);
      expect(screen.getByRole('form')).toBeInTheDocument();
    });
  });

  describe('Authentication', () => {
    it('shows loading state while checking auth', () => {
      mockIsLoading = true;
      render(<NewQuestionPage />);
      expect(screen.getByText(/loading/i)).toBeInTheDocument();
    });

    it('redirects to login when not authenticated', async () => {
      mockIsAuthenticated = false;
      mockIsLoading = false;
      render(<NewQuestionPage />);

      await waitFor(() => {
        expect(mockPush).toHaveBeenCalledWith('/login?redirect=/new/question');
      });
    });

    it('renders form when authenticated', () => {
      render(<NewQuestionPage />);
      expect(screen.getByRole('form')).toBeInTheDocument();
    });
  });

  describe('Form Fields', () => {
    it('has title input', () => {
      render(<NewQuestionPage />);
      expect(screen.getByLabelText(/title/i)).toBeInTheDocument();
    });

    it('title input is required', () => {
      render(<NewQuestionPage />);
      const titleInput = screen.getByLabelText(/title/i);
      expect(titleInput).toBeRequired();
    });

    it('title input has max length hint', () => {
      render(<NewQuestionPage />);
      expect(screen.getByText(/200 characters/i)).toBeInTheDocument();
    });

    it('has description textarea', () => {
      render(<NewQuestionPage />);
      expect(screen.getByLabelText(/description/i)).toBeInTheDocument();
    });

    it('description is required', () => {
      render(<NewQuestionPage />);
      const descInput = screen.getByLabelText(/description/i);
      expect(descInput).toBeRequired();
    });

    it('has tags input', () => {
      render(<NewQuestionPage />);
      expect(screen.getByLabelText(/tags/i)).toBeInTheDocument();
    });

    it('tags input shows max hint', () => {
      render(<NewQuestionPage />);
      expect(screen.getByText(/up to 5 tags/i)).toBeInTheDocument();
    });

    it('does not have success criteria input (questions dont need it)', () => {
      render(<NewQuestionPage />);
      expect(screen.queryByLabelText(/success criteria/i)).not.toBeInTheDocument();
    });

    it('does not have difficulty select (questions dont need it)', () => {
      render(<NewQuestionPage />);
      expect(screen.queryByLabelText(/difficulty/i)).not.toBeInTheDocument();
    });
  });

  describe('Form Interactions', () => {
    it('allows typing in title field', async () => {
      const user = userEvent.setup();
      render(<NewQuestionPage />);

      const titleInput = screen.getByLabelText(/title/i);
      await user.type(titleInput, 'Test Question Title');

      expect(titleInput).toHaveValue('Test Question Title');
    });

    it('allows typing in description field', async () => {
      const user = userEvent.setup();
      render(<NewQuestionPage />);

      const descInput = screen.getByLabelText(/description/i);
      await user.type(descInput, 'This is a test description');

      expect(descInput).toHaveValue('This is a test description');
    });

    it('allows entering tags', async () => {
      const user = userEvent.setup();
      render(<NewQuestionPage />);

      const tagsInput = screen.getByLabelText(/tags/i);
      await user.type(tagsInput, 'javascript, react');

      expect(tagsInput).toHaveValue('javascript, react');
    });
  });

  describe('Preview Tab', () => {
    it('has preview tab button', () => {
      render(<NewQuestionPage />);
      expect(
        screen.getByRole('tab', { name: /preview/i })
      ).toBeInTheDocument();
    });

    it('has write/edit tab button', () => {
      render(<NewQuestionPage />);
      expect(
        screen.getByRole('tab', { name: /write|edit/i })
      ).toBeInTheDocument();
    });

    it('shows form by default (Write tab active)', () => {
      render(<NewQuestionPage />);
      const writeTab = screen.getByRole('tab', { name: /write|edit/i });
      expect(writeTab).toHaveAttribute('aria-selected', 'true');
    });

    it('switches to preview when Preview tab clicked', async () => {
      const user = userEvent.setup();
      render(<NewQuestionPage />);

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
      render(<NewQuestionPage />);

      // Type markdown content
      const descInput = screen.getByLabelText(/description/i);
      await user.type(descInput, '# Test Heading');

      // Switch to preview
      const previewTab = screen.getByRole('tab', { name: /preview/i });
      await user.click(previewTab);

      // Should render as markdown
      await waitFor(() => {
        const previewArea = screen.getByTestId('preview-area');
        expect(previewArea).toBeInTheDocument();
      });
    });
  });

  describe('Form Submission', () => {
    it('has submit button', () => {
      render(<NewQuestionPage />);
      expect(
        screen.getByRole('button', { name: /ask question|submit/i })
      ).toBeInTheDocument();
    });

    it('has cancel button', () => {
      render(<NewQuestionPage />);
      expect(
        screen.getByRole('button', { name: /cancel/i })
      ).toBeInTheDocument();
    });

    it('cancel button goes back', async () => {
      const user = userEvent.setup();
      render(<NewQuestionPage />);

      const cancelBtn = screen.getByRole('button', { name: /cancel/i });
      await user.click(cancelBtn);

      expect(mockRouterBack).toHaveBeenCalled();
    });

    it('submits form with correct data', async () => {
      const user = userEvent.setup();
      render(<NewQuestionPage />);

      // Fill in the form
      await user.type(screen.getByLabelText(/title/i), 'How do I test React components?');
      await user.type(
        screen.getByLabelText(/description/i),
        'I am trying to understand how to write effective tests for my React components. Can someone explain the best practices?'
      );
      await user.type(screen.getByLabelText(/tags/i), 'react, testing');

      // Submit
      const submitBtn = screen.getByRole('button', {
        name: /ask question|submit/i,
      });
      await user.click(submitBtn);

      await waitFor(() => {
        expect(mockApiPost).toHaveBeenCalledWith(
          '/v1/questions',
          expect.objectContaining({
            type: 'question',
            title: 'How do I test React components?',
            description: expect.stringContaining('effective tests'),
            tags: ['react', 'testing'],
          })
        );
      });
    });

    it('redirects to created post on success', async () => {
      const user = userEvent.setup();
      mockApiPost.mockResolvedValue({ id: 'created-456' });

      render(<NewQuestionPage />);

      // Fill minimum required fields
      await user.type(screen.getByLabelText(/title/i), 'Test Question Title');
      await user.type(
        screen.getByLabelText(/description/i),
        'A sufficiently long description for the question that meets minimum length requirements.'
      );

      // Submit
      const submitBtn = screen.getByRole('button', {
        name: /ask question|submit/i,
      });
      await user.click(submitBtn);

      await waitFor(() => {
        expect(mockPush).toHaveBeenCalledWith('/posts/created-456');
      });
    });

    it('shows loading state during submission', async () => {
      const user = userEvent.setup();
      // Make API slow
      mockApiPost.mockImplementation(
        () => new Promise((resolve) => setTimeout(() => resolve({ id: '123' }), 1000))
      );

      render(<NewQuestionPage />);

      // Fill form with valid values (title must be >= 10 chars)
      await user.type(screen.getByLabelText(/title/i), 'Valid Test Title For Question');
      await user.type(
        screen.getByLabelText(/description/i),
        'A sufficiently long description for the question that meets minimum length requirements.'
      );

      // Submit
      const submitBtn = screen.getByRole('button', {
        name: /ask question|submit/i,
      });
      await user.click(submitBtn);

      // Should show loading
      expect(screen.getByText(/asking|submitting/i)).toBeInTheDocument();
    });

    it('disables submit button during submission', async () => {
      const user = userEvent.setup();
      mockApiPost.mockImplementation(
        () => new Promise((resolve) => setTimeout(() => resolve({ id: '123' }), 1000))
      );

      render(<NewQuestionPage />);

      // Fill form with valid values (title must be >= 10 chars)
      await user.type(screen.getByLabelText(/title/i), 'Valid Test Title For Question');
      await user.type(
        screen.getByLabelText(/description/i),
        'A sufficiently long description for the question that meets minimum length requirements.'
      );

      const submitBtn = screen.getByRole('button', {
        name: /ask question|submit/i,
      });
      await user.click(submitBtn);

      expect(submitBtn).toBeDisabled();
    });
  });

  describe('Validation', () => {
    it('shows error when title too short', async () => {
      const user = userEvent.setup();
      render(<NewQuestionPage />);

      // Enter short title
      await user.type(screen.getByLabelText(/title/i), 'Hi?');
      await user.type(
        screen.getByLabelText(/description/i),
        'A valid description that meets the minimum length requirements.'
      );

      const submitBtn = screen.getByRole('button', {
        name: /ask question|submit/i,
      });
      await user.click(submitBtn);

      await waitFor(() => {
        expect(screen.getByText(/at least 10 characters/i)).toBeInTheDocument();
      });
    });

    it('shows error when description too short', async () => {
      const user = userEvent.setup();
      render(<NewQuestionPage />);

      await user.type(screen.getByLabelText(/title/i), 'Valid Title Here');
      await user.type(screen.getByLabelText(/description/i), 'Short');

      const submitBtn = screen.getByRole('button', {
        name: /ask question|submit/i,
      });
      await user.click(submitBtn);

      await waitFor(() => {
        expect(screen.getByText(/at least 50 characters/i)).toBeInTheDocument();
      });
    });

    it('shows error when more than 5 tags', async () => {
      const user = userEvent.setup();
      render(<NewQuestionPage />);

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
        name: /ask question|submit/i,
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
        new ApiError(400, 'VALIDATION_ERROR', 'Similar question already exists')
      );

      render(<NewQuestionPage />);

      await user.type(screen.getByLabelText(/title/i), 'Valid Title Here');
      await user.type(
        screen.getByLabelText(/description/i),
        'A valid description that meets the minimum length requirements.'
      );

      const submitBtn = screen.getByRole('button', {
        name: /ask question|submit/i,
      });
      await user.click(submitBtn);

      await waitFor(() => {
        expect(screen.getByText(/similar question already exists/i)).toBeInTheDocument();
      });
    });

    it('displays generic error for network failure', async () => {
      const user = userEvent.setup();
      mockApiPost.mockRejectedValue(new Error('Network error'));

      render(<NewQuestionPage />);

      await user.type(screen.getByLabelText(/title/i), 'Valid Title Here');
      await user.type(
        screen.getByLabelText(/description/i),
        'A valid description that meets the minimum length requirements.'
      );

      const submitBtn = screen.getByRole('button', {
        name: /ask question|submit/i,
      });
      await user.click(submitBtn);

      await waitFor(() => {
        expect(screen.getByText(/failed to submit|error/i)).toBeInTheDocument();
      });
    });

    it('can dismiss error message', async () => {
      const user = userEvent.setup();
      mockApiPost.mockRejectedValue(
        new ApiError(400, 'VALIDATION_ERROR', 'Some error')
      );

      render(<NewQuestionPage />);

      await user.type(screen.getByLabelText(/title/i), 'Valid Title Here');
      await user.type(
        screen.getByLabelText(/description/i),
        'A valid description that meets the minimum length requirements.'
      );

      const submitBtn = screen.getByRole('button', {
        name: /ask question|submit/i,
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
      render(<NewQuestionPage />);
      expect(screen.getByRole('heading', { level: 1 })).toBeInTheDocument();
    });

    it('form fields have labels', () => {
      render(<NewQuestionPage />);

      expect(screen.getByLabelText(/title/i)).toBeInTheDocument();
      expect(screen.getByLabelText(/description/i)).toBeInTheDocument();
      expect(screen.getByLabelText(/tags/i)).toBeInTheDocument();
    });

    it('error messages are announced', async () => {
      const user = userEvent.setup();
      mockApiPost.mockRejectedValue(
        new ApiError(400, 'VALIDATION_ERROR', 'Error occurred')
      );

      render(<NewQuestionPage />);

      await user.type(screen.getByLabelText(/title/i), 'Valid Title Here');
      await user.type(
        screen.getByLabelText(/description/i),
        'A valid description that meets the minimum length requirements.'
      );

      const submitBtn = screen.getByRole('button', {
        name: /ask question|submit/i,
      });
      await user.click(submitBtn);

      await waitFor(() => {
        expect(screen.getByRole('alert')).toBeInTheDocument();
      });
    });

    it('submit button has accessible name', () => {
      render(<NewQuestionPage />);

      const submitBtn = screen.getByRole('button', {
        name: /ask question|submit/i,
      });
      expect(submitBtn).toHaveAccessibleName();
    });
  });

  describe('Responsive Design', () => {
    it('has container with max-width', () => {
      render(<NewQuestionPage />);

      const container = screen.getByTestId('new-question-container');
      expect(container).toHaveClass('max-w-3xl');
    });
  });
});
