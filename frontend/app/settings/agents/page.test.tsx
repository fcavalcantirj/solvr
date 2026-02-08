import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import MyAgentsPage from './page';

// Mock Next.js navigation
vi.mock('next/navigation', () => ({
  useRouter: () => ({
    push: vi.fn(),
  }),
  usePathname: () => '/settings/agents',
  redirect: vi.fn(),
}));

// Mock useAuth hook
const mockUser = { id: 'user-1', type: 'human', displayName: 'Test User' };
vi.mock('@/hooks/use-auth', () => ({
  useAuth: () => ({
    isAuthenticated: true,
    isLoading: false,
    user: mockUser,
    loginWithGitHub: vi.fn(),
    loginWithGoogle: vi.fn(),
    logout: vi.fn(),
  }),
}));

// Mock API
const mockGetUserAgents = vi.fn();
const mockUpdateAgent = vi.fn();
const mockConfirmClaim = vi.fn();

vi.mock('@/lib/api', () => ({
  api: {
    getUserAgents: () => mockGetUserAgents(),
    updateAgent: (id: string, data: unknown) => mockUpdateAgent(id, data),
    confirmClaim: (token: string) => mockConfirmClaim(token),
  },
  formatRelativeTime: () => '2 days ago',
  truncateText: (text: string, len: number) => text?.substring(0, len) || '',
}));

// Sample agent data
const mockAgent = {
  id: 'agent-1',
  display_name: 'Test Agent',
  bio: 'A test agent',
  karma: 100,
  status: 'active',
  has_human_backed_badge: true,
  model: 'claude-opus-4',
  created_at: '2026-01-15T10:00:00Z',
};

describe('MyAgentsPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockGetUserAgents.mockResolvedValue({ data: [mockAgent] });
  });

  describe('Edit agent functionality', () => {
    it('shows Edit button on each agent card', async () => {
      render(<MyAgentsPage />);

      await waitFor(() => {
        expect(screen.getByText('Test Agent')).toBeInTheDocument();
      });

      expect(screen.getByRole('button', { name: /edit/i })).toBeInTheDocument();
    });

    it('opens modal when Edit button is clicked', async () => {
      render(<MyAgentsPage />);

      await waitFor(() => {
        expect(screen.getByText('Test Agent')).toBeInTheDocument();
      });

      const editButton = screen.getByRole('button', { name: /edit/i });
      fireEvent.click(editButton);

      expect(screen.getByText('Edit Agent')).toBeInTheDocument();
      expect(screen.getByLabelText(/model/i)).toBeInTheDocument();
    });

    it('displays current model value in modal', async () => {
      render(<MyAgentsPage />);

      await waitFor(() => {
        expect(screen.getByText('Test Agent')).toBeInTheDocument();
      });

      const editButton = screen.getByRole('button', { name: /edit/i });
      fireEvent.click(editButton);

      const modelInput = screen.getByLabelText(/model/i) as HTMLInputElement;
      expect(modelInput.value).toBe('claude-opus-4');
    });

    it('calls updateAgent API on save', async () => {
      mockUpdateAgent.mockResolvedValue({ data: { ...mockAgent, model: 'gpt-4o' } });

      render(<MyAgentsPage />);

      await waitFor(() => {
        expect(screen.getByText('Test Agent')).toBeInTheDocument();
      });

      const editButton = screen.getByRole('button', { name: /edit/i });
      fireEvent.click(editButton);

      const modelInput = screen.getByLabelText(/model/i);
      fireEvent.change(modelInput, { target: { value: 'gpt-4o' } });

      const saveButton = screen.getByRole('button', { name: /save/i });
      fireEvent.click(saveButton);

      await waitFor(() => {
        expect(mockUpdateAgent).toHaveBeenCalledWith('agent-1', { model: 'gpt-4o' });
      });
    });

    it('closes modal and refreshes list on successful save', async () => {
      mockUpdateAgent.mockResolvedValue({ data: { ...mockAgent, model: 'gpt-4o' } });

      render(<MyAgentsPage />);

      await waitFor(() => {
        expect(screen.getByText('Test Agent')).toBeInTheDocument();
      });

      const editButton = screen.getByRole('button', { name: /edit/i });
      fireEvent.click(editButton);

      const modelInput = screen.getByLabelText(/model/i);
      fireEvent.change(modelInput, { target: { value: 'gpt-4o' } });

      const saveButton = screen.getByRole('button', { name: /save/i });
      fireEvent.click(saveButton);

      await waitFor(() => {
        // Modal should be closed (no "Edit Agent" title visible)
        expect(screen.queryByText('Edit Agent')).not.toBeInTheDocument();
      });

      // Should have fetched agents again
      expect(mockGetUserAgents).toHaveBeenCalledTimes(2);
    });

    it('shows error message when save fails', async () => {
      mockUpdateAgent.mockRejectedValue(new Error('Update failed'));

      render(<MyAgentsPage />);

      await waitFor(() => {
        expect(screen.getByText('Test Agent')).toBeInTheDocument();
      });

      const editButton = screen.getByRole('button', { name: /edit/i });
      fireEvent.click(editButton);

      const saveButton = screen.getByRole('button', { name: /save/i });
      fireEvent.click(saveButton);

      await waitFor(() => {
        expect(screen.getByText(/update failed/i)).toBeInTheDocument();
      });
    });
  });

  describe('Loading state', () => {
    it('shows loading indicator while fetching agents', () => {
      mockGetUserAgents.mockImplementation(() => new Promise(() => {})); // Never resolves
      render(<MyAgentsPage />);

      // Check for loader (by finding the spinning icon)
      expect(document.querySelector('.animate-spin')).toBeInTheDocument();
    });
  });

  describe('Empty state', () => {
    it('shows empty message when no agents', async () => {
      mockGetUserAgents.mockResolvedValue({ data: [] });
      render(<MyAgentsPage />);

      await waitFor(() => {
        expect(screen.getByText('No agents yet')).toBeInTheDocument();
      });
    });
  });

  describe('Claim functionality', () => {
    it('shows claim section', async () => {
      render(<MyAgentsPage />);

      await waitFor(() => {
        expect(screen.getByText('CLAIM AN AGENT')).toBeInTheDocument();
      });
    });
  });
});
