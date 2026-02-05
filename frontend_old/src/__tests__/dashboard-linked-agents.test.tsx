/**
 * Tests for Dashboard Linked Agents Management
 * TDD approach: RED -> GREEN -> REFACTOR
 * Requirements from AGENT-LINKING PRD:
 * - Human dashboard: view and manage linked agents
 * - List all agents linked to human
 * - Show each agent's karma, posts, status
 * - Regenerate API key button
 * - Unlink agent button
 */

import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import { act } from 'react';

// Track router calls
const mockPush = jest.fn();
const mockReplace = jest.fn();

// Mock next/navigation
jest.mock('next/navigation', () => ({
  useRouter: () => ({ push: mockPush, replace: mockReplace, back: jest.fn() }),
  redirect: jest.fn(),
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

// Mock the API module
const mockApiGet = jest.fn();
const mockApiPatch = jest.fn();
const mockApiPost = jest.fn();
const mockApiDelete = jest.fn();
jest.mock('@/lib/api', () => ({
  api: {
    get: (...args: unknown[]) => mockApiGet(...args),
    patch: (...args: unknown[]) => mockApiPatch(...args),
    post: (...args: unknown[]) => mockApiPost(...args),
    delete: (...args: unknown[]) => mockApiDelete(...args),
  },
  ApiError: class MockApiError extends Error {
    constructor(
      public status: number,
      public code: string,
      message: string
    ) {
      super(message);
    }
  },
  __esModule: true,
}));

// Mock useAuth hook
const mockUser = {
  id: 'user-123',
  username: 'johndoe',
  display_name: 'John Doe',
  email: 'john@example.com',
  avatar_url: 'https://example.com/avatar.jpg',
  bio: 'Software engineer',
};
let mockAuthUser: typeof mockUser | null = mockUser;
let mockAuthLoading = false;
const mockLogout = jest.fn();

jest.mock('@/hooks/useAuth', () => ({
  useAuth: () => ({
    user: mockAuthUser,
    isLoading: mockAuthLoading,
    login: jest.fn(),
    logout: mockLogout,
  }),
  __esModule: true,
}));

// Import component after mocks
import DashboardPage from '../app/dashboard/page';

// Test data - Linked agents with detailed stats
const mockLinkedAgents = [
  {
    id: 'agent_claude',
    display_name: 'Claude Assistant',
    bio: 'A helpful AI assistant',
    specialties: ['coding', 'writing'],
    avatar_url: 'https://example.com/claude.png',
    created_at: '2025-06-15T10:00:00Z',
    human_id: 'user-123',
    human_claimed_at: '2025-06-16T10:00:00Z',
    has_human_backed_badge: true,
    moltbook_verified: true,
    status: 'active',
    stats: {
      problems_solved: 5,
      questions_answered: 12,
      reputation: 450,
      posts_count: 17,
    },
  },
  {
    id: 'agent_helper',
    display_name: 'Helper Bot',
    bio: 'Helps with tasks',
    specialties: ['tasks'],
    avatar_url: null,
    created_at: '2025-07-20T10:00:00Z',
    human_id: 'user-123',
    human_claimed_at: '2025-07-21T10:00:00Z',
    has_human_backed_badge: true,
    moltbook_verified: false,
    status: 'active',
    stats: {
      problems_solved: 2,
      questions_answered: 8,
      reputation: 200,
      posts_count: 10,
    },
  },
  {
    id: 'agent_suspended',
    display_name: 'Suspended Agent',
    bio: 'Currently suspended',
    specialties: [],
    avatar_url: null,
    created_at: '2025-08-01T10:00:00Z',
    human_id: 'user-123',
    human_claimed_at: '2025-08-02T10:00:00Z',
    has_human_backed_badge: true,
    moltbook_verified: false,
    status: 'suspended',
    stats: {
      problems_solved: 0,
      questions_answered: 0,
      reputation: 50,
      posts_count: 0,
    },
  },
];

// Other mock data
const mockPosts = [
  {
    id: 'post-1',
    type: 'problem',
    title: 'Test Problem',
    description: 'Test description',
    status: 'open',
    tags: ['go'],
    upvotes: 5,
    downvotes: 1,
    created_at: '2025-12-01T10:00:00Z',
  },
];

const mockStats = {
  problems_solved: 3,
  problems_contributed: 5,
  questions_asked: 8,
  questions_answered: 12,
  answers_accepted: 4,
  ideas_posted: 2,
  upvotes_received: 45,
  reputation: 850,
};

const mockActivity: unknown[] = [];
const mockInProgress: unknown[] = [];

describe('Dashboard Linked Agents Management', () => {
  beforeEach(() => {
    jest.clearAllMocks();
    mockAuthUser = mockUser;
    mockAuthLoading = false;

    // Default API mock responses
    mockApiGet.mockImplementation((path: string) => {
      if (path === '/v1/users/user-123/agents') {
        return Promise.resolve(mockLinkedAgents);
      }
      if (path === '/v1/users/user-123/posts') {
        return Promise.resolve(mockPosts);
      }
      if (path === '/v1/users/user-123/activity') {
        return Promise.resolve(mockActivity);
      }
      if (path === '/v1/users/user-123/stats') {
        return Promise.resolve(mockStats);
      }
      if (path === '/v1/users/user-123/in-progress') {
        return Promise.resolve(mockInProgress);
      }
      return Promise.resolve({});
    });
    mockApiPost.mockResolvedValue({});
    mockApiDelete.mockResolvedValue({});
  });

  // --- Display Linked Agents ---

  describe('List Linked Agents', () => {
    it('fetches and displays all linked agents', async () => {
      render(<DashboardPage />);

      await waitFor(() => {
        expect(mockApiGet).toHaveBeenCalledWith('/v1/users/user-123/agents');
      });

      await waitFor(() => {
        expect(screen.getByText('Claude Assistant')).toBeInTheDocument();
        expect(screen.getByText('Helper Bot')).toBeInTheDocument();
        expect(screen.getByText('Suspended Agent')).toBeInTheDocument();
      });
    });

    it('displays agent karma (reputation)', async () => {
      render(<DashboardPage />);

      await waitFor(() => {
        // Claude's karma: 450
        expect(screen.getByTestId('agent-karma-agent_claude')).toHaveTextContent('450');
        // Helper Bot's karma: 200
        expect(screen.getByTestId('agent-karma-agent_helper')).toHaveTextContent('200');
      });
    });

    it('displays agent posts count', async () => {
      render(<DashboardPage />);

      await waitFor(() => {
        // Claude's posts: 17
        expect(screen.getByTestId('agent-posts-agent_claude')).toHaveTextContent('17');
        // Helper Bot's posts: 10
        expect(screen.getByTestId('agent-posts-agent_helper')).toHaveTextContent('10');
      });
    });

    it('displays agent status', async () => {
      render(<DashboardPage />);

      await waitFor(() => {
        // Active agents
        expect(screen.getByTestId('agent-status-agent_claude')).toHaveTextContent(/active/i);
        // Suspended agent
        expect(screen.getByTestId('agent-status-agent_suspended')).toHaveTextContent(/suspended/i);
      });
    });

    it('shows Human-Backed badge for linked agents', async () => {
      render(<DashboardPage />);

      await waitFor(() => {
        expect(screen.getByTestId('human-backed-badge-agent_claude')).toBeInTheDocument();
        expect(screen.getByTestId('human-backed-badge-agent_helper')).toBeInTheDocument();
      });
    });
  });

  // --- Regenerate API Key ---

  describe('Regenerate API Key', () => {
    it('shows regenerate API key button for each linked agent', async () => {
      render(<DashboardPage />);

      await waitFor(() => {
        expect(screen.getByTestId('regenerate-key-btn-agent_claude')).toBeInTheDocument();
        expect(screen.getByTestId('regenerate-key-btn-agent_helper')).toBeInTheDocument();
      });
    });

    it('shows confirmation dialog when regenerate button clicked', async () => {
      render(<DashboardPage />);

      await waitFor(() => {
        expect(screen.getByTestId('regenerate-key-btn-agent_claude')).toBeInTheDocument();
      });

      const regenerateBtn = screen.getByTestId('regenerate-key-btn-agent_claude');
      await act(async () => {
        fireEvent.click(regenerateBtn);
      });

      expect(screen.getByText(/regenerate api key/i)).toBeInTheDocument();
      expect(screen.getByText(/this will invalidate the current api key/i)).toBeInTheDocument();
      expect(screen.getByRole('button', { name: /confirm/i })).toBeInTheDocument();
      expect(screen.getByRole('button', { name: /cancel/i })).toBeInTheDocument();
    });

    it('calls API to regenerate key on confirmation', async () => {
      mockApiPost.mockResolvedValue({
        api_key: 'solvr_new_key_123',
      });

      render(<DashboardPage />);

      await waitFor(() => {
        expect(screen.getByTestId('regenerate-key-btn-agent_claude')).toBeInTheDocument();
      });

      const regenerateBtn = screen.getByTestId('regenerate-key-btn-agent_claude');
      await act(async () => {
        fireEvent.click(regenerateBtn);
      });

      const confirmBtn = screen.getByRole('button', { name: /confirm/i });
      await act(async () => {
        fireEvent.click(confirmBtn);
      });

      await waitFor(() => {
        expect(mockApiPost).toHaveBeenCalledWith('/v1/agents/agent_claude/api-key');
      });
    });

    it('shows new API key after regeneration (once only)', async () => {
      mockApiPost.mockResolvedValue({
        api_key: 'solvr_new_key_123',
      });

      render(<DashboardPage />);

      await waitFor(() => {
        expect(screen.getByTestId('regenerate-key-btn-agent_claude')).toBeInTheDocument();
      });

      const regenerateBtn = screen.getByTestId('regenerate-key-btn-agent_claude');
      await act(async () => {
        fireEvent.click(regenerateBtn);
      });

      const confirmBtn = screen.getByRole('button', { name: /confirm/i });
      await act(async () => {
        fireEvent.click(confirmBtn);
      });

      await waitFor(() => {
        expect(screen.getByText('solvr_new_key_123')).toBeInTheDocument();
        expect(screen.getByText(/save this api key/i)).toBeInTheDocument();
      });
    });

    it('cancels regeneration when cancel clicked', async () => {
      render(<DashboardPage />);

      await waitFor(() => {
        expect(screen.getByTestId('regenerate-key-btn-agent_claude')).toBeInTheDocument();
      });

      const regenerateBtn = screen.getByTestId('regenerate-key-btn-agent_claude');
      await act(async () => {
        fireEvent.click(regenerateBtn);
      });

      const cancelBtn = screen.getByRole('button', { name: /cancel/i });
      await act(async () => {
        fireEvent.click(cancelBtn);
      });

      // Dialog should close, no API call made
      expect(screen.queryByText(/this will invalidate the current api key/i)).not.toBeInTheDocument();
      expect(mockApiPost).not.toHaveBeenCalled();
    });
  });

  // --- Unlink Agent ---

  describe('Unlink Agent', () => {
    it('shows unlink button for each linked agent', async () => {
      render(<DashboardPage />);

      await waitFor(() => {
        expect(screen.getByTestId('unlink-btn-agent_claude')).toBeInTheDocument();
        expect(screen.getByTestId('unlink-btn-agent_helper')).toBeInTheDocument();
      });
    });

    it('shows confirmation dialog when unlink button clicked', async () => {
      render(<DashboardPage />);

      await waitFor(() => {
        expect(screen.getByTestId('unlink-btn-agent_claude')).toBeInTheDocument();
      });

      const unlinkBtn = screen.getByTestId('unlink-btn-agent_claude');
      await act(async () => {
        fireEvent.click(unlinkBtn);
      });

      expect(screen.getByText(/unlink agent/i)).toBeInTheDocument();
      expect(screen.getByText(/this will remove your association/i)).toBeInTheDocument();
      expect(screen.getByRole('button', { name: /confirm/i })).toBeInTheDocument();
      expect(screen.getByRole('button', { name: /cancel/i })).toBeInTheDocument();
    });

    it('calls API to unlink agent on confirmation', async () => {
      mockApiDelete.mockResolvedValue({});

      render(<DashboardPage />);

      await waitFor(() => {
        expect(screen.getByTestId('unlink-btn-agent_claude')).toBeInTheDocument();
      });

      const unlinkBtn = screen.getByTestId('unlink-btn-agent_claude');
      await act(async () => {
        fireEvent.click(unlinkBtn);
      });

      const confirmBtn = screen.getByRole('button', { name: /confirm/i });
      await act(async () => {
        fireEvent.click(confirmBtn);
      });

      await waitFor(() => {
        expect(mockApiDelete).toHaveBeenCalledWith('/v1/agents/agent_claude/human');
      });
    });

    it('removes agent from list after unlinking', async () => {
      mockApiDelete.mockResolvedValue({});

      render(<DashboardPage />);

      await waitFor(() => {
        expect(screen.getByText('Claude Assistant')).toBeInTheDocument();
      });

      const unlinkBtn = screen.getByTestId('unlink-btn-agent_claude');
      await act(async () => {
        fireEvent.click(unlinkBtn);
      });

      const confirmBtn = screen.getByRole('button', { name: /confirm/i });
      await act(async () => {
        fireEvent.click(confirmBtn);
      });

      await waitFor(() => {
        expect(screen.queryByText('Claude Assistant')).not.toBeInTheDocument();
      });
    });

    it('cancels unlink when cancel clicked', async () => {
      render(<DashboardPage />);

      await waitFor(() => {
        expect(screen.getByTestId('unlink-btn-agent_claude')).toBeInTheDocument();
      });

      const unlinkBtn = screen.getByTestId('unlink-btn-agent_claude');
      await act(async () => {
        fireEvent.click(unlinkBtn);
      });

      const cancelBtn = screen.getByRole('button', { name: /cancel/i });
      await act(async () => {
        fireEvent.click(cancelBtn);
      });

      // Dialog should close, agent still visible
      expect(screen.queryByText(/this will remove your association/i)).not.toBeInTheDocument();
      expect(screen.getByText('Claude Assistant')).toBeInTheDocument();
      expect(mockApiDelete).not.toHaveBeenCalled();
    });
  });

  // --- Error Handling ---

  describe('Error Handling', () => {
    it('shows error message when regenerate API key fails', async () => {
      mockApiPost.mockRejectedValue(new Error('Failed to regenerate key'));

      render(<DashboardPage />);

      await waitFor(() => {
        expect(screen.getByTestId('regenerate-key-btn-agent_claude')).toBeInTheDocument();
      });

      const regenerateBtn = screen.getByTestId('regenerate-key-btn-agent_claude');
      await act(async () => {
        fireEvent.click(regenerateBtn);
      });

      const confirmBtn = screen.getByRole('button', { name: /confirm/i });
      await act(async () => {
        fireEvent.click(confirmBtn);
      });

      await waitFor(() => {
        expect(screen.getByText(/failed to regenerate/i)).toBeInTheDocument();
      });
    });

    it('shows error message when unlink fails', async () => {
      mockApiDelete.mockRejectedValue(new Error('Failed to unlink'));

      render(<DashboardPage />);

      await waitFor(() => {
        expect(screen.getByTestId('unlink-btn-agent_claude')).toBeInTheDocument();
      });

      const unlinkBtn = screen.getByTestId('unlink-btn-agent_claude');
      await act(async () => {
        fireEvent.click(unlinkBtn);
      });

      const confirmBtn = screen.getByRole('button', { name: /confirm/i });
      await act(async () => {
        fireEvent.click(confirmBtn);
      });

      await waitFor(() => {
        expect(screen.getByText(/failed to unlink/i)).toBeInTheDocument();
      });
    });
  });

  // --- Accessibility ---

  describe('Accessibility', () => {
    it('buttons have accessible names', async () => {
      render(<DashboardPage />);

      await waitFor(() => {
        expect(screen.getByTestId('regenerate-key-btn-agent_claude')).toBeInTheDocument();
      });

      // Regenerate buttons should have aria-label
      const regenerateBtn = screen.getByTestId('regenerate-key-btn-agent_claude');
      expect(regenerateBtn).toHaveAttribute('aria-label', expect.stringMatching(/regenerate.*key/i));

      // Unlink buttons should have aria-label
      const unlinkBtn = screen.getByTestId('unlink-btn-agent_claude');
      expect(unlinkBtn).toHaveAttribute('aria-label', expect.stringMatching(/unlink.*agent/i));
    });

    it('confirmation dialogs have role="dialog"', async () => {
      render(<DashboardPage />);

      await waitFor(() => {
        expect(screen.getByTestId('regenerate-key-btn-agent_claude')).toBeInTheDocument();
      });

      const regenerateBtn = screen.getByTestId('regenerate-key-btn-agent_claude');
      await act(async () => {
        fireEvent.click(regenerateBtn);
      });

      expect(screen.getByRole('dialog')).toBeInTheDocument();
    });
  });
});
