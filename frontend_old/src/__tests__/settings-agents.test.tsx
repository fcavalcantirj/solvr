/**
 * Tests for Settings Page - Agents Tab
 * TDD approach: RED -> GREEN -> REFACTOR
 * Requirements from PRD line 493: Settings: agents list
 *
 * Split from settings.test.tsx to keep files under 800 lines
 */

import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import { act } from 'react';

// Track router push calls
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

// Import the ApiError to use in tests
import { ApiError } from '@/lib/api';

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
import SettingsPage from '../app/settings/page';

// Test data - User's agents
const mockAgents = [
  {
    id: 'agent_claude',
    display_name: 'Claude Assistant',
    bio: 'A helpful AI assistant',
    specialties: ['coding', 'writing'],
    avatar_url: 'https://example.com/claude.png',
    created_at: '2025-06-15T10:00:00Z',
    human_id: 'user-123',
    moltbook_verified: true,
  },
  {
    id: 'agent_helper',
    display_name: 'Helper Bot',
    bio: 'Helps with tasks',
    specialties: ['tasks'],
    avatar_url: null,
    created_at: '2025-07-20T10:00:00Z',
    human_id: 'user-123',
    moltbook_verified: false,
  },
];

// Test data - Notification settings
const mockNotificationSettings = {
  email_answers: true,
  email_comments: true,
  email_mentions: false,
  email_digest: 'weekly',
};

describe('Settings Page - Agents Tab', () => {
  beforeEach(() => {
    jest.clearAllMocks();
    mockAuthUser = mockUser;
    mockAuthLoading = false;

    // Default API mock responses
    mockApiGet.mockImplementation((path: string) => {
      if (path === '/v1/users/user-123/agents') {
        return Promise.resolve(mockAgents);
      }
      if (path === '/v1/users/user-123/notifications') {
        return Promise.resolve(mockNotificationSettings);
      }
      return Promise.resolve({});
    });
    mockApiPatch.mockResolvedValue({});
    mockApiPost.mockResolvedValue({});
  });

  it('switches to agents tab when clicked', async () => {
    render(<SettingsPage />);

    await waitFor(() => {
      expect(screen.getByRole('tab', { name: /agents/i })).toBeInTheDocument();
    });

    const agentsTab = screen.getByRole('tab', { name: /agents/i });
    await act(async () => {
      fireEvent.click(agentsTab);
    });

    expect(agentsTab).toHaveAttribute('aria-selected', 'true');
  });

  it('fetches and displays user agents list', async () => {
    render(<SettingsPage />);

    await waitFor(() => {
      expect(screen.getByRole('tab', { name: /agents/i })).toBeInTheDocument();
    });

    const agentsTab = screen.getByRole('tab', { name: /agents/i });
    await act(async () => {
      fireEvent.click(agentsTab);
    });

    await waitFor(() => {
      expect(screen.getByText('Claude Assistant')).toBeInTheDocument();
      expect(screen.getByText('Helper Bot')).toBeInTheDocument();
    });
  });

  it('displays agent ID for each agent', async () => {
    render(<SettingsPage />);

    await waitFor(() => {
      expect(screen.getByRole('tab', { name: /agents/i })).toBeInTheDocument();
    });

    const agentsTab = screen.getByRole('tab', { name: /agents/i });
    await act(async () => {
      fireEvent.click(agentsTab);
    });

    await waitFor(() => {
      expect(screen.getByText('@agent_claude')).toBeInTheDocument();
      expect(screen.getByText('@agent_helper')).toBeInTheDocument();
    });
  });

  it('displays Moltbook verified badge for verified agents', async () => {
    render(<SettingsPage />);

    await waitFor(() => {
      expect(screen.getByRole('tab', { name: /agents/i })).toBeInTheDocument();
    });

    const agentsTab = screen.getByRole('tab', { name: /agents/i });
    await act(async () => {
      fireEvent.click(agentsTab);
    });

    await waitFor(() => {
      expect(screen.getByText(/moltbook verified/i)).toBeInTheDocument();
    });
  });

  it('shows create new agent button', async () => {
    render(<SettingsPage />);

    await waitFor(() => {
      expect(screen.getByRole('tab', { name: /agents/i })).toBeInTheDocument();
    });

    const agentsTab = screen.getByRole('tab', { name: /agents/i });
    await act(async () => {
      fireEvent.click(agentsTab);
    });

    await waitFor(() => {
      expect(
        screen.getByRole('button', { name: /create.*agent|new.*agent/i })
      ).toBeInTheDocument();
    });
  });

  it('shows create agent form when button clicked', async () => {
    render(<SettingsPage />);

    await waitFor(() => {
      expect(screen.getByRole('tab', { name: /agents/i })).toBeInTheDocument();
    });

    const agentsTab = screen.getByRole('tab', { name: /agents/i });
    await act(async () => {
      fireEvent.click(agentsTab);
    });

    await waitFor(() => {
      expect(
        screen.getByRole('button', { name: /create.*agent|new.*agent/i })
      ).toBeInTheDocument();
    });

    const createButton = screen.getByRole('button', { name: /create.*agent|new.*agent/i });
    await act(async () => {
      fireEvent.click(createButton);
    });

    expect(screen.getByLabelText(/agent id/i)).toBeInTheDocument();
    expect(screen.getByLabelText(/display name/i)).toBeInTheDocument();
  });

  it('validates agent ID format (alphanumeric + underscore)', async () => {
    mockApiPost.mockResolvedValue({
      agent: { id: 'test_agent', display_name: 'Test Agent' },
      api_key: 'solvr_abc123',
    });

    render(<SettingsPage />);

    await waitFor(() => {
      expect(screen.getByRole('tab', { name: /agents/i })).toBeInTheDocument();
    });

    const agentsTab = screen.getByRole('tab', { name: /agents/i });
    await act(async () => {
      fireEvent.click(agentsTab);
    });

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /create.*agent|new.*agent/i })).toBeInTheDocument();
    });

    const createButton = screen.getByRole('button', { name: /create.*agent|new.*agent/i });
    await act(async () => {
      fireEvent.click(createButton);
    });

    const idInput = screen.getByLabelText(/agent id/i);
    await act(async () => {
      fireEvent.change(idInput, { target: { value: 'invalid-id!' } });
    });

    const submitButton = screen.getByRole('button', { name: /create$/i });
    await act(async () => {
      fireEvent.click(submitButton);
    });

    expect(
      screen.getByText(/agent id must contain only letters, numbers, and underscores/i)
    ).toBeInTheDocument();
  });

  it('creates agent with valid data', async () => {
    mockApiPost.mockResolvedValue({
      agent: { id: 'new_agent', display_name: 'New Agent' },
      api_key: 'solvr_newkey123',
    });

    render(<SettingsPage />);

    await waitFor(() => {
      expect(screen.getByRole('tab', { name: /agents/i })).toBeInTheDocument();
    });

    const agentsTab = screen.getByRole('tab', { name: /agents/i });
    await act(async () => {
      fireEvent.click(agentsTab);
    });

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /create.*agent|new.*agent/i })).toBeInTheDocument();
    });

    const createButton = screen.getByRole('button', { name: /create.*agent|new.*agent/i });
    await act(async () => {
      fireEvent.click(createButton);
    });

    const idInput = screen.getByLabelText(/agent id/i);
    // Get the display name input by its specific ID since there are multiple
    const displayNameInput = document.getElementById('agent-display-name') as HTMLInputElement;
    await act(async () => {
      fireEvent.change(idInput, { target: { value: 'new_agent' } });
      fireEvent.change(displayNameInput, { target: { value: 'New Agent' } });
    });

    const submitButton = screen.getByRole('button', { name: /create$/i });
    await act(async () => {
      fireEvent.click(submitButton);
    });

    await waitFor(() => {
      expect(mockApiPost).toHaveBeenCalledWith(
        '/v1/agents',
        expect.objectContaining({
          id: 'new_agent',
          display_name: 'New Agent',
        })
      );
    });
  });

  it('shows API key after agent creation (only once)', async () => {
    mockApiPost.mockResolvedValue({
      agent: { id: 'new_agent', display_name: 'New Agent' },
      api_key: 'solvr_newkey123',
    });

    render(<SettingsPage />);

    await waitFor(() => {
      expect(screen.getByRole('tab', { name: /agents/i })).toBeInTheDocument();
    });

    const agentsTab = screen.getByRole('tab', { name: /agents/i });
    await act(async () => {
      fireEvent.click(agentsTab);
    });

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /create.*agent|new.*agent/i })).toBeInTheDocument();
    });

    const createButton = screen.getByRole('button', { name: /create.*agent|new.*agent/i });
    await act(async () => {
      fireEvent.click(createButton);
    });

    const idInput = screen.getByLabelText(/agent id/i);
    // Get the display name input by its specific ID since there are multiple
    const displayNameInput = document.getElementById('agent-display-name') as HTMLInputElement;
    await act(async () => {
      fireEvent.change(idInput, { target: { value: 'new_agent' } });
      fireEvent.change(displayNameInput, { target: { value: 'New Agent' } });
    });

    const submitButton = screen.getByRole('button', { name: /create$/i });
    await act(async () => {
      fireEvent.click(submitButton);
    });

    await waitFor(() => {
      expect(screen.getByText('solvr_newkey123')).toBeInTheDocument();
      expect(screen.getByText(/save this api key/i)).toBeInTheDocument();
    });
  });

  it('links to agent manage page', async () => {
    render(<SettingsPage />);

    await waitFor(() => {
      expect(screen.getByRole('tab', { name: /agents/i })).toBeInTheDocument();
    });

    const agentsTab = screen.getByRole('tab', { name: /agents/i });
    await act(async () => {
      fireEvent.click(agentsTab);
    });

    await waitFor(() => {
      expect(screen.getByText('Claude Assistant')).toBeInTheDocument();
    });

    const manageLink = screen.getAllByRole('link', { name: /manage|edit/i })[0];
    expect(manageLink).toHaveAttribute('href', '/settings/agents/agent_claude');
  });

  it('shows empty state when no agents', async () => {
    mockApiGet.mockImplementation((path: string) => {
      if (path === '/v1/users/user-123/agents') {
        return Promise.resolve([]);
      }
      return Promise.resolve({});
    });

    render(<SettingsPage />);

    await waitFor(() => {
      expect(screen.getByRole('tab', { name: /agents/i })).toBeInTheDocument();
    });

    const agentsTab = screen.getByRole('tab', { name: /agents/i });
    await act(async () => {
      fireEvent.click(agentsTab);
    });

    await waitFor(() => {
      expect(screen.getByText(/no agents registered/i)).toBeInTheDocument();
    });
  });

  it('shows error when agents fetch fails', async () => {
    mockApiGet.mockImplementation((path: string) => {
      if (path === '/v1/users/user-123/agents') {
        return Promise.reject(new ApiError(500, 'INTERNAL_ERROR', 'Server error'));
      }
      return Promise.resolve({});
    });

    render(<SettingsPage />);

    await waitFor(() => {
      expect(screen.getByRole('tab', { name: /agents/i })).toBeInTheDocument();
    });

    const agentsTab = screen.getByRole('tab', { name: /agents/i });
    await act(async () => {
      fireEvent.click(agentsTab);
    });

    await waitFor(() => {
      expect(screen.getByText(/failed to load agents/i)).toBeInTheDocument();
    });
  });

  it('shows retry button on error', async () => {
    mockApiGet.mockImplementation((path: string) => {
      if (path === '/v1/users/user-123/agents') {
        return Promise.reject(new ApiError(500, 'INTERNAL_ERROR', 'Server error'));
      }
      return Promise.resolve({});
    });

    render(<SettingsPage />);

    await waitFor(() => {
      expect(screen.getByRole('tab', { name: /agents/i })).toBeInTheDocument();
    });

    const agentsTab = screen.getByRole('tab', { name: /agents/i });
    await act(async () => {
      fireEvent.click(agentsTab);
    });

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /retry/i })).toBeInTheDocument();
    });
  });
});
