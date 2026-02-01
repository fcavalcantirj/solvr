/**
 * Tests for Settings Page component
 * TDD approach: RED -> GREEN -> REFACTOR
 * Requirements from PRD lines 491-494:
 * - Create /settings page
 * - Settings: profile form (edit display_name, bio, avatar)
 * - Settings: agents list (list registered agents, create new agent form)
 * - Settings: notifications preferences
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

describe('Settings Page', () => {
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

  // --- Basic Structure Tests ---

  describe('Basic Structure', () => {
    it('renders the settings page with main container', async () => {
      render(<SettingsPage />);

      await waitFor(() => {
        expect(screen.getByRole('main')).toBeInTheDocument();
      });
    });

    it('renders the page heading', async () => {
      render(<SettingsPage />);

      await waitFor(() => {
        expect(
          screen.getByRole('heading', { name: /settings/i })
        ).toBeInTheDocument();
      });
    });

    it('renders navigation tabs for Profile, Agents, Notifications', async () => {
      render(<SettingsPage />);

      await waitFor(() => {
        expect(screen.getByRole('tab', { name: /profile/i })).toBeInTheDocument();
        expect(screen.getByRole('tab', { name: /agents/i })).toBeInTheDocument();
        expect(screen.getByRole('tab', { name: /notifications/i })).toBeInTheDocument();
      });
    });
  });

  // --- Authentication Required Tests ---

  describe('Authentication Required', () => {
    it('redirects to login when not authenticated', async () => {
      mockAuthUser = null;

      render(<SettingsPage />);

      await waitFor(() => {
        expect(mockReplace).toHaveBeenCalledWith('/login');
      });
    });

    it('shows loading state while auth is loading', async () => {
      mockAuthLoading = true;

      render(<SettingsPage />);

      expect(screen.getByTestId('settings-skeleton')).toBeInTheDocument();
    });
  });

  // --- Profile Tab Tests ---

  describe('Profile Tab', () => {
    it('shows profile form with current user data', async () => {
      render(<SettingsPage />);

      await waitFor(() => {
        const displayNameInput = screen.getByLabelText(/display name/i);
        expect(displayNameInput).toHaveValue('John Doe');
      });

      expect(screen.getByLabelText(/bio/i)).toHaveValue('Software engineer');
    });

    it('shows avatar preview with current avatar', async () => {
      render(<SettingsPage />);

      await waitFor(() => {
        const avatar = screen.getByAltText(/profile avatar/i);
        expect(avatar).toHaveAttribute('src', 'https://example.com/avatar.jpg');
      });
    });

    it('shows avatar URL input field', async () => {
      render(<SettingsPage />);

      await waitFor(() => {
        const avatarInput = screen.getByLabelText(/avatar url/i);
        expect(avatarInput).toHaveValue('https://example.com/avatar.jpg');
      });
    });

    it('allows editing display name', async () => {
      render(<SettingsPage />);

      await waitFor(() => {
        expect(screen.getByLabelText(/display name/i)).toBeInTheDocument();
      });

      const input = screen.getByLabelText(/display name/i);
      await act(async () => {
        fireEvent.change(input, { target: { value: 'Jane Doe' } });
      });

      expect(input).toHaveValue('Jane Doe');
    });

    it('allows editing bio', async () => {
      render(<SettingsPage />);

      await waitFor(() => {
        expect(screen.getByLabelText(/bio/i)).toBeInTheDocument();
      });

      const textarea = screen.getByLabelText(/bio/i);
      await act(async () => {
        fireEvent.change(textarea, { target: { value: 'New bio text' } });
      });

      expect(textarea).toHaveValue('New bio text');
    });

    it('shows character limit for bio (500 chars)', async () => {
      render(<SettingsPage />);

      await waitFor(() => {
        expect(screen.getByText(/0\s*\/\s*500|17\s*\/\s*500/i)).toBeInTheDocument();
      });
    });

    it('submits profile update on save button click', async () => {
      render(<SettingsPage />);

      await waitFor(() => {
        expect(screen.getByLabelText(/display name/i)).toBeInTheDocument();
      });

      const input = screen.getByLabelText(/display name/i);
      await act(async () => {
        fireEvent.change(input, { target: { value: 'Updated Name' } });
      });

      const saveButton = screen.getByRole('button', { name: /save profile/i });
      await act(async () => {
        fireEvent.click(saveButton);
      });

      await waitFor(() => {
        expect(mockApiPatch).toHaveBeenCalledWith(
          '/v1/users/user-123',
          expect.objectContaining({
            display_name: 'Updated Name',
          })
        );
      });
    });

    it('shows success message after profile update', async () => {
      mockApiPatch.mockResolvedValue({ ...mockUser, display_name: 'Updated Name' });

      render(<SettingsPage />);

      await waitFor(() => {
        expect(screen.getByLabelText(/display name/i)).toBeInTheDocument();
      });

      const input = screen.getByLabelText(/display name/i);
      await act(async () => {
        fireEvent.change(input, { target: { value: 'Updated Name' } });
      });

      const saveButton = screen.getByRole('button', { name: /save profile/i });
      await act(async () => {
        fireEvent.click(saveButton);
      });

      await waitFor(() => {
        expect(screen.getByText(/profile updated/i)).toBeInTheDocument();
      });
    });

    it('shows error message on profile update failure', async () => {
      mockApiPatch.mockRejectedValue(new ApiError(400, 'VALIDATION_ERROR', 'Invalid data'));

      render(<SettingsPage />);

      await waitFor(() => {
        expect(screen.getByLabelText(/display name/i)).toBeInTheDocument();
      });

      const saveButton = screen.getByRole('button', { name: /save profile/i });
      await act(async () => {
        fireEvent.click(saveButton);
      });

      await waitFor(() => {
        expect(screen.getByText(/invalid data/i)).toBeInTheDocument();
      });
    });

    it('validates display name is not empty', async () => {
      render(<SettingsPage />);

      await waitFor(() => {
        expect(screen.getByLabelText(/display name/i)).toBeInTheDocument();
      });

      const input = screen.getByLabelText(/display name/i);
      await act(async () => {
        fireEvent.change(input, { target: { value: '' } });
      });

      const saveButton = screen.getByRole('button', { name: /save profile/i });
      await act(async () => {
        fireEvent.click(saveButton);
      });

      expect(screen.getByText(/display name is required/i)).toBeInTheDocument();
      expect(mockApiPatch).not.toHaveBeenCalled();
    });

    it('validates display name max length (50 chars)', async () => {
      render(<SettingsPage />);

      await waitFor(() => {
        expect(screen.getByLabelText(/display name/i)).toBeInTheDocument();
      });

      const input = screen.getByLabelText(/display name/i);
      const longName = 'a'.repeat(51);
      await act(async () => {
        fireEvent.change(input, { target: { value: longName } });
      });

      const saveButton = screen.getByRole('button', { name: /save profile/i });
      await act(async () => {
        fireEvent.click(saveButton);
      });

      expect(screen.getByText(/display name must be 50 characters or less/i)).toBeInTheDocument();
    });
  });

  // Note: Agents Tab tests are in settings-agents.test.tsx

  // --- Notifications Tab Tests ---

  describe('Notifications Tab', () => {
    it('switches to notifications tab when clicked', async () => {
      render(<SettingsPage />);

      await waitFor(() => {
        expect(screen.getByRole('tab', { name: /notifications/i })).toBeInTheDocument();
      });

      const notificationsTab = screen.getByRole('tab', { name: /notifications/i });
      await act(async () => {
        fireEvent.click(notificationsTab);
      });

      expect(notificationsTab).toHaveAttribute('aria-selected', 'true');
    });

    it('displays notification preference toggles', async () => {
      render(<SettingsPage />);

      await waitFor(() => {
        expect(screen.getByRole('tab', { name: /notifications/i })).toBeInTheDocument();
      });

      const notificationsTab = screen.getByRole('tab', { name: /notifications/i });
      await act(async () => {
        fireEvent.click(notificationsTab);
      });

      await waitFor(() => {
        expect(screen.getByText(/email.*answers/i)).toBeInTheDocument();
        expect(screen.getByText(/email.*comments/i)).toBeInTheDocument();
        expect(screen.getByText(/email.*mentions/i)).toBeInTheDocument();
      });
    });

    it('shows current notification settings', async () => {
      render(<SettingsPage />);

      await waitFor(() => {
        expect(screen.getByRole('tab', { name: /notifications/i })).toBeInTheDocument();
      });

      const notificationsTab = screen.getByRole('tab', { name: /notifications/i });
      await act(async () => {
        fireEvent.click(notificationsTab);
      });

      await waitFor(() => {
        const answersToggle = screen.getByRole('checkbox', { name: /answers/i });
        expect(answersToggle).toBeChecked();
      });
    });

    it('allows toggling notification preferences', async () => {
      render(<SettingsPage />);

      await waitFor(() => {
        expect(screen.getByRole('tab', { name: /notifications/i })).toBeInTheDocument();
      });

      const notificationsTab = screen.getByRole('tab', { name: /notifications/i });
      await act(async () => {
        fireEvent.click(notificationsTab);
      });

      await waitFor(() => {
        expect(screen.getByRole('checkbox', { name: /mentions/i })).toBeInTheDocument();
      });

      const mentionsToggle = screen.getByRole('checkbox', { name: /mentions/i });
      await act(async () => {
        fireEvent.click(mentionsToggle);
      });

      expect(mentionsToggle).toBeChecked();
    });

    it('shows email digest frequency selector', async () => {
      render(<SettingsPage />);

      await waitFor(() => {
        expect(screen.getByRole('tab', { name: /notifications/i })).toBeInTheDocument();
      });

      const notificationsTab = screen.getByRole('tab', { name: /notifications/i });
      await act(async () => {
        fireEvent.click(notificationsTab);
      });

      await waitFor(() => {
        expect(screen.getByLabelText(/digest frequency/i)).toBeInTheDocument();
      });

      const digestSelect = screen.getByLabelText(/digest frequency/i);
      expect(digestSelect).toHaveValue('weekly');
    });

    it('saves notification settings on button click', async () => {
      render(<SettingsPage />);

      await waitFor(() => {
        expect(screen.getByRole('tab', { name: /notifications/i })).toBeInTheDocument();
      });

      const notificationsTab = screen.getByRole('tab', { name: /notifications/i });
      await act(async () => {
        fireEvent.click(notificationsTab);
      });

      await waitFor(() => {
        expect(screen.getByRole('button', { name: /save.*notification|save.*preferences/i })).toBeInTheDocument();
      });

      const saveButton = screen.getByRole('button', { name: /save.*notification|save.*preferences/i });
      await act(async () => {
        fireEvent.click(saveButton);
      });

      await waitFor(() => {
        expect(mockApiPatch).toHaveBeenCalledWith(
          '/v1/users/user-123/notifications',
          expect.any(Object)
        );
      });
    });
  });

  // --- Accessibility Tests ---

  describe('Accessibility', () => {
    it('has proper heading hierarchy', async () => {
      render(<SettingsPage />);

      await waitFor(() => {
        const h1 = screen.getByRole('heading', { level: 1 });
        expect(h1).toHaveTextContent(/settings/i);
      });
    });

    it('has proper ARIA labels on tabs', async () => {
      render(<SettingsPage />);

      await waitFor(() => {
        const tablist = screen.getByRole('tablist');
        expect(tablist).toBeInTheDocument();
      });
    });

    it('has accessible form labels', async () => {
      render(<SettingsPage />);

      await waitFor(() => {
        expect(screen.getByLabelText(/display name/i)).toBeInTheDocument();
        expect(screen.getByLabelText(/bio/i)).toBeInTheDocument();
      });
    });

    it('shows loading/saving states for screen readers', async () => {
      render(<SettingsPage />);

      await waitFor(() => {
        expect(screen.getByLabelText(/display name/i)).toBeInTheDocument();
      });

      const saveButton = screen.getByRole('button', { name: /save profile/i });

      // Simulate slow API
      mockApiPatch.mockImplementation(() => new Promise((resolve) => setTimeout(resolve, 100)));

      await act(async () => {
        fireEvent.click(saveButton);
      });

      expect(saveButton).toHaveAttribute('aria-busy', 'true');
    });
  });

  // Note: Error Handling tests for Agents tab are in settings-agents.test.tsx
});
