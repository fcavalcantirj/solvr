/**
 * Tests for Human-Backed Badge display on Agent Profile Page
 * Per PRD requirement: Human-Backed badge display on agent profile
 *
 * Requirements:
 * - Show badge on agent profile page
 * - Optionally show human's handle if human opts in
 *
 * TDD approach: RED -> GREEN -> REFACTOR
 */

import { render, screen, waitFor } from '@testing-library/react';
import { act } from 'react';

// Mock next/navigation
const mockParams = { id: 'verified-agent' };
jest.mock('next/navigation', () => ({
  useRouter: () => ({ push: jest.fn(), replace: jest.fn(), back: jest.fn() }),
  useParams: () => mockParams,
  notFound: jest.fn(),
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
jest.mock('@/lib/api', () => ({
  api: {
    get: (...args: unknown[]) => mockApiGet(...args),
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

// Import the ApiError to use in tests
import { ApiError } from '@/lib/api';

// Mock useAuth hook
jest.mock('@/hooks/useAuth', () => ({
  useAuth: () => ({
    user: null,
    isLoading: false,
    login: jest.fn(),
    logout: jest.fn(),
  }),
  __esModule: true,
}));

// Import component after mocks
import AgentProfilePage from '../app/agents/[id]/page';

// Test data - Agent with Human-Backed badge
const mockAgentWithHumanBadge = {
  id: 'verified-agent',
  display_name: 'Verified Agent',
  human_id: 'owner-123',
  bio: 'A verified AI assistant.',
  specialties: ['debugging'],
  avatar_url: 'https://example.com/avatar.jpg',
  moltbook_verified: false,
  created_at: '2026-01-01T10:00:00Z',
  has_human_backed_badge: true,
  human_username: 'john_owner',
  karma: 150,
  owner: {
    id: 'owner-123',
    username: 'john_owner',
    display_name: 'John Owner',
  },
  stats: {
    problems_solved: 10,
    problems_contributed: 20,
    questions_asked: 5,
    questions_answered: 30,
    answers_accepted: 15,
    ideas_posted: 8,
    responses_given: 25,
    upvotes_received: 100,
    reputation: 1500,
  },
};

// Agent without Human-Backed badge
const mockAgentWithoutBadge = {
  ...mockAgentWithHumanBadge,
  id: 'unverified-agent',
  display_name: 'Unverified Agent',
  has_human_backed_badge: false,
  human_username: undefined,
  human_id: undefined,
  owner: undefined,
};

// Agent with badge but no username shown
const mockAgentWithBadgeNoUsername = {
  ...mockAgentWithHumanBadge,
  id: 'badge-no-username',
  display_name: 'Badge Agent',
  human_username: undefined, // Human hasn't opted in to show username
};

describe('AgentProfilePage - Human-Backed Badge', () => {
  beforeEach(() => {
    jest.clearAllMocks();
    mockParams.id = 'verified-agent';
  });

  describe('Badge Display', () => {
    it('shows Human-Backed badge when agent has_human_backed_badge is true', async () => {
      mockApiGet.mockImplementation((path: string) => {
        if (path === '/v1/agents/verified-agent') {
          return Promise.resolve(mockAgentWithHumanBadge);
        }
        if (path.includes('/activity')) {
          return Promise.resolve([]);
        }
        return Promise.reject(new ApiError(404, 'NOT_FOUND', 'Not found'));
      });

      await act(async () => {
        render(<AgentProfilePage />);
      });

      await waitFor(() => {
        expect(screen.getByTestId('human-backed-badge')).toBeInTheDocument();
      });
    });

    it('does not show Human-Backed badge when agent has_human_backed_badge is false', async () => {
      mockParams.id = 'unverified-agent';
      mockApiGet.mockImplementation((path: string) => {
        if (path === '/v1/agents/unverified-agent') {
          return Promise.resolve(mockAgentWithoutBadge);
        }
        if (path.includes('/activity')) {
          return Promise.resolve([]);
        }
        return Promise.reject(new ApiError(404, 'NOT_FOUND', 'Not found'));
      });

      await act(async () => {
        render(<AgentProfilePage />);
      });

      await waitFor(() => {
        expect(screen.getByText('Unverified Agent')).toBeInTheDocument();
      });

      expect(screen.queryByTestId('human-backed-badge')).not.toBeInTheDocument();
    });

    it('shows Human-Backed badge text', async () => {
      mockApiGet.mockImplementation((path: string) => {
        if (path === '/v1/agents/verified-agent') {
          return Promise.resolve(mockAgentWithHumanBadge);
        }
        if (path.includes('/activity')) {
          return Promise.resolve([]);
        }
        return Promise.reject(new ApiError(404, 'NOT_FOUND', 'Not found'));
      });

      await act(async () => {
        render(<AgentProfilePage />);
      });

      await waitFor(() => {
        expect(screen.getByText(/Human-Backed/i)).toBeInTheDocument();
      });
    });
  });

  describe('Human Handle Display', () => {
    it('shows human username when agent has human_username', async () => {
      mockApiGet.mockImplementation((path: string) => {
        if (path === '/v1/agents/verified-agent') {
          return Promise.resolve(mockAgentWithHumanBadge);
        }
        if (path.includes('/activity')) {
          return Promise.resolve([]);
        }
        return Promise.reject(new ApiError(404, 'NOT_FOUND', 'Not found'));
      });

      await act(async () => {
        render(<AgentProfilePage />);
      });

      await waitFor(() => {
        // The owner link should be visible
        expect(screen.getByText('john_owner')).toBeInTheDocument();
      });
    });

    it('shows badge without human username when not provided', async () => {
      mockParams.id = 'badge-no-username';
      mockApiGet.mockImplementation((path: string) => {
        if (path === '/v1/agents/badge-no-username') {
          return Promise.resolve(mockAgentWithBadgeNoUsername);
        }
        if (path.includes('/activity')) {
          return Promise.resolve([]);
        }
        return Promise.reject(new ApiError(404, 'NOT_FOUND', 'Not found'));
      });

      await act(async () => {
        render(<AgentProfilePage />);
      });

      await waitFor(() => {
        expect(screen.getByTestId('human-backed-badge')).toBeInTheDocument();
      });

      // Badge should still be visible even without username
      expect(screen.getByText(/Human-Backed/i)).toBeInTheDocument();
    });
  });

  describe('Badge Position', () => {
    it('displays Human-Backed badge near agent name in header', async () => {
      mockApiGet.mockImplementation((path: string) => {
        if (path === '/v1/agents/verified-agent') {
          return Promise.resolve(mockAgentWithHumanBadge);
        }
        if (path.includes('/activity')) {
          return Promise.resolve([]);
        }
        return Promise.reject(new ApiError(404, 'NOT_FOUND', 'Not found'));
      });

      await act(async () => {
        render(<AgentProfilePage />);
      });

      await waitFor(() => {
        // Badge should be in the same container as the agent name
        const header = screen.getByRole('heading', { level: 1 });
        const badgeContainer = header.parentElement;
        expect(badgeContainer).toContainElement(
          screen.getByTestId('human-backed-badge')
        );
      });
    });
  });

  describe('Multiple Badges', () => {
    it('shows both Moltbook Verified and Human-Backed badges when both are true', async () => {
      mockApiGet.mockImplementation((path: string) => {
        if (path === '/v1/agents/verified-agent') {
          return Promise.resolve({
            ...mockAgentWithHumanBadge,
            moltbook_verified: true,
          });
        }
        if (path.includes('/activity')) {
          return Promise.resolve([]);
        }
        return Promise.reject(new ApiError(404, 'NOT_FOUND', 'Not found'));
      });

      await act(async () => {
        render(<AgentProfilePage />);
      });

      await waitFor(() => {
        expect(screen.getByText(/Moltbook Verified/i)).toBeInTheDocument();
        expect(screen.getByTestId('human-backed-badge')).toBeInTheDocument();
      });
    });

    it('displays AI Agent badge alongside Human-Backed badge', async () => {
      mockApiGet.mockImplementation((path: string) => {
        if (path === '/v1/agents/verified-agent') {
          return Promise.resolve(mockAgentWithHumanBadge);
        }
        if (path.includes('/activity')) {
          return Promise.resolve([]);
        }
        return Promise.reject(new ApiError(404, 'NOT_FOUND', 'Not found'));
      });

      await act(async () => {
        render(<AgentProfilePage />);
      });

      await waitFor(() => {
        expect(screen.getByText(/AI Agent/i)).toBeInTheDocument();
        expect(screen.getByTestId('human-backed-badge')).toBeInTheDocument();
      });
    });
  });

  describe('Accessibility', () => {
    it('badge has appropriate aria-label', async () => {
      mockApiGet.mockImplementation((path: string) => {
        if (path === '/v1/agents/verified-agent') {
          return Promise.resolve(mockAgentWithHumanBadge);
        }
        if (path.includes('/activity')) {
          return Promise.resolve([]);
        }
        return Promise.reject(new ApiError(404, 'NOT_FOUND', 'Not found'));
      });

      await act(async () => {
        render(<AgentProfilePage />);
      });

      await waitFor(() => {
        const badge = screen.getByTestId('human-backed-badge');
        expect(badge).toHaveAttribute(
          'aria-label',
          'Human-Backed verified agent'
        );
      });
    });
  });
});
