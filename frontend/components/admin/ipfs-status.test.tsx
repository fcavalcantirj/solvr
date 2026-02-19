import { render, screen } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { IPFSStatusIndicator } from './ipfs-status';

// Mock the hook
vi.mock('@/hooks/use-ipfs-health', () => ({
  useIPFSHealth: vi.fn(),
}));

import { useIPFSHealth } from '@/hooks/use-ipfs-health';

describe('IPFSStatusIndicator', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('shows loading state', () => {
    vi.mocked(useIPFSHealth).mockReturnValue({
      data: null,
      loading: true,
      error: null,
      refetch: vi.fn(),
    });

    render(<IPFSStatusIndicator />);
    expect(screen.getByText('IPFS NODE')).toBeInTheDocument();
    expect(screen.getByText('CHECKING...')).toBeInTheDocument();
  });

  it('shows connected state with green indicator', () => {
    vi.mocked(useIPFSHealth).mockReturnValue({
      data: {
        connected: true,
        peer_id: '12D3KooWJG6rZ1KWTQy1fPeaZuxhfukik3RmYTjyf76Yn6CwUP3A',
        version: 'kubo/0.39.0/',
      },
      loading: false,
      error: null,
      refetch: vi.fn(),
    });

    render(<IPFSStatusIndicator />);
    expect(screen.getByText('IPFS NODE')).toBeInTheDocument();
    expect(screen.getByText('CONNECTED')).toBeInTheDocument();
    // Peer ID should be truncated
    expect(screen.getByText('12D3Ko...UP3A')).toBeInTheDocument();
    expect(screen.getByText('kubo/0.39.0/')).toBeInTheDocument();
  });

  it('shows disconnected state with red indicator', () => {
    vi.mocked(useIPFSHealth).mockReturnValue({
      data: {
        connected: false,
        peer_id: '',
        version: '',
        error: 'timeout',
      },
      loading: false,
      error: null,
      refetch: vi.fn(),
    });

    render(<IPFSStatusIndicator />);
    expect(screen.getByText('DISCONNECTED')).toBeInTheDocument();
    expect(screen.getByText('timeout')).toBeInTheDocument();
  });

  it('shows error state when API call fails', () => {
    vi.mocked(useIPFSHealth).mockReturnValue({
      data: null,
      loading: false,
      error: 'Failed to fetch',
      refetch: vi.fn(),
    });

    render(<IPFSStatusIndicator />);
    expect(screen.getByText('ERROR')).toBeInTheDocument();
    expect(screen.getByText('Failed to fetch')).toBeInTheDocument();
  });

  it('shows status dot with correct color for connected', () => {
    vi.mocked(useIPFSHealth).mockReturnValue({
      data: {
        connected: true,
        peer_id: '12D3KooWTest',
        version: 'kubo/0.39.0',
      },
      loading: false,
      error: null,
      refetch: vi.fn(),
    });

    render(<IPFSStatusIndicator />);
    const statusDot = screen.getByTestId('ipfs-status-dot');
    expect(statusDot.className).toContain('bg-emerald-500');
  });

  it('shows status dot with correct color for disconnected', () => {
    vi.mocked(useIPFSHealth).mockReturnValue({
      data: {
        connected: false,
        peer_id: '',
        version: '',
        error: 'timeout',
      },
      loading: false,
      error: null,
      refetch: vi.fn(),
    });

    render(<IPFSStatusIndicator />);
    const statusDot = screen.getByTestId('ipfs-status-dot');
    expect(statusDot.className).toContain('bg-red-500');
  });

  it('shows status dot with yellow color for loading', () => {
    vi.mocked(useIPFSHealth).mockReturnValue({
      data: null,
      loading: true,
      error: null,
      refetch: vi.fn(),
    });

    render(<IPFSStatusIndicator />);
    const statusDot = screen.getByTestId('ipfs-status-dot');
    expect(statusDot.className).toContain('bg-yellow-500');
  });

  it('shows status dot with red color for error', () => {
    vi.mocked(useIPFSHealth).mockReturnValue({
      data: null,
      loading: false,
      error: 'Network error',
      refetch: vi.fn(),
    });

    render(<IPFSStatusIndicator />);
    const statusDot = screen.getByTestId('ipfs-status-dot');
    expect(statusDot.className).toContain('bg-red-500');
  });

  it('truncates long peer IDs', () => {
    vi.mocked(useIPFSHealth).mockReturnValue({
      data: {
        connected: true,
        peer_id: '12D3KooWJG6rZ1KWTQy1fPeaZuxhfukik3RmYTjyf76Yn6CwUP3A',
        version: 'kubo/0.39.0',
      },
      loading: false,
      error: null,
      refetch: vi.fn(),
    });

    render(<IPFSStatusIndicator />);
    // Should show first 6 chars + ... + last 4 chars
    expect(screen.getByText('12D3Ko...UP3A')).toBeInTheDocument();
    // Full peer ID should NOT be shown
    expect(screen.queryByText('12D3KooWJG6rZ1KWTQy1fPeaZuxhfukik3RmYTjyf76Yn6CwUP3A')).not.toBeInTheDocument();
  });

  it('does not truncate short peer IDs', () => {
    vi.mocked(useIPFSHealth).mockReturnValue({
      data: {
        connected: true,
        peer_id: 'ShortPeerID',
        version: 'kubo/0.39.0',
      },
      loading: false,
      error: null,
      refetch: vi.fn(),
    });

    render(<IPFSStatusIndicator />);
    expect(screen.getByText('ShortPeerID')).toBeInTheDocument();
  });

  it('uses 30s default poll interval', () => {
    vi.mocked(useIPFSHealth).mockReturnValue({
      data: null,
      loading: true,
      error: null,
      refetch: vi.fn(),
    });

    render(<IPFSStatusIndicator />);
    expect(useIPFSHealth).toHaveBeenCalledWith({ pollIntervalMs: 30000 });
  });

  it('accepts custom poll interval', () => {
    vi.mocked(useIPFSHealth).mockReturnValue({
      data: null,
      loading: true,
      error: null,
      refetch: vi.fn(),
    });

    render(<IPFSStatusIndicator pollIntervalMs={10000} />);
    expect(useIPFSHealth).toHaveBeenCalledWith({ pollIntervalMs: 10000 });
  });
});
