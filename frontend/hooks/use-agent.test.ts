"use client";

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { renderHook, waitFor } from '@testing-library/react';
import { useAgent } from './use-agent';
import { api } from '@/lib/api';

// Mock the API module
vi.mock('@/lib/api', () => ({
  api: {
    getAgent: vi.fn(),
  },
  formatRelativeTime: (date: string) => '5d ago',
}));

const mockAgent = {
  id: 'agent_ClaudiusThePirateEmperor',
  display_name: 'Claudius The Pirate Emperor',
  bio: 'A swashbuckling AI agent',
  status: 'active',
  karma: 1250,
  post_count: 42,
  created_at: '2025-01-01T10:00:00Z',
  has_human_backed_badge: true,
  avatar_url: 'https://example.com/avatar.png',
};

const mockStats = {
  posts_count: 42,
  answers_count: 15,
  responses_count: 8,
  karma: 1250,
};

describe('useAgent', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('returns loading true initially', () => {
    // Arrange - make the API hang
    (api.getAgent as ReturnType<typeof vi.fn>).mockImplementation(
      () => new Promise(() => {}) // Never resolves
    );

    // Act
    const { result } = renderHook(() => useAgent('agent_ClaudiusThePirateEmperor'));

    // Assert
    expect(result.current.loading).toBe(true);
    expect(result.current.agent).toBeNull();
    expect(result.current.error).toBeNull();
  });

  it('fetches agent data and transforms to camelCase', async () => {
    // Arrange
    (api.getAgent as ReturnType<typeof vi.fn>).mockResolvedValue({ data: { agent: mockAgent, stats: mockStats } });

    // Act
    const { result } = renderHook(() => useAgent('agent_ClaudiusThePirateEmperor'));

    // Wait for data to load
    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    // Assert - camelCase transformation
    expect(result.current.agent).not.toBeNull();
    expect(result.current.agent?.id).toBe('agent_ClaudiusThePirateEmperor');
    expect(result.current.agent?.displayName).toBe('Claudius The Pirate Emperor');
    expect(result.current.agent?.bio).toBe('A swashbuckling AI agent');
    expect(result.current.agent?.status).toBe('active');
    expect(result.current.agent?.karma).toBe(1250);
    expect(result.current.agent?.postCount).toBe(42);
    expect(result.current.agent?.hasHumanBackedBadge).toBe(true);
    expect(result.current.agent?.avatarUrl).toBe('https://example.com/avatar.png');
    expect(result.current.agent?.time).toBe('5d ago');
    expect(result.current.error).toBeNull();
  });

  it('handles API error gracefully', async () => {
    // Arrange
    (api.getAgent as ReturnType<typeof vi.fn>).mockRejectedValue(new Error('Network error'));

    // Act
    const { result } = renderHook(() => useAgent('agent_ClaudiusThePirateEmperor'));

    // Wait for error
    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    // Assert
    expect(result.current.error).toBe('Network error');
    expect(result.current.agent).toBeNull();
  });

  it('handles 404 not found', async () => {
    // Arrange
    (api.getAgent as ReturnType<typeof vi.fn>).mockRejectedValue(new Error('API error: 404'));

    // Act
    const { result } = renderHook(() => useAgent('nonexistent_agent'));

    // Wait for error
    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    // Assert
    expect(result.current.error).toBe('API error: 404');
    expect(result.current.agent).toBeNull();
  });

  it('does not fetch when id is empty', async () => {
    // Act
    const { result } = renderHook(() => useAgent(''));

    // Wait for state to settle
    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    // Assert - no API call made
    expect(api.getAgent).not.toHaveBeenCalled();
    expect(result.current.agent).toBeNull();
    expect(result.current.error).toBeNull();
  });

  it('refetches when refetch is called', async () => {
    // Arrange
    (api.getAgent as ReturnType<typeof vi.fn>).mockResolvedValue({ data: { agent: mockAgent, stats: mockStats } });

    // Act
    const { result } = renderHook(() => useAgent('agent_ClaudiusThePirateEmperor'));

    // Wait for initial load
    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    // Clear mocks and change the data
    vi.clearAllMocks();
    const updatedAgent = { ...mockAgent, karma: 1500 };
    const updatedStats = { ...mockStats, karma: 1500 };
    (api.getAgent as ReturnType<typeof vi.fn>).mockResolvedValue({ data: { agent: updatedAgent, stats: updatedStats } });

    // Refetch
    result.current.refetch();

    // Wait for refetch
    await waitFor(() => {
      expect(result.current.agent?.karma).toBe(1500);
    });

    // Assert
    expect(api.getAgent).toHaveBeenCalledTimes(1);
  });

  it('handles agent with null optional fields', async () => {
    // Arrange - agent without avatar_url
    const agentWithNulls = {
      id: 'agent_minimal',
      display_name: 'Minimal Agent',
      bio: '',
      status: 'pending',
      karma: 0,
      post_count: 0,
      created_at: '2025-01-01T10:00:00Z',
      has_human_backed_badge: false,
      avatar_url: null,
    };
    const minimalStats = {
      posts_count: 0,
      answers_count: 0,
      responses_count: 0,
      karma: 0,
    };

    (api.getAgent as ReturnType<typeof vi.fn>).mockResolvedValue({ data: { agent: agentWithNulls, stats: minimalStats } });

    // Act
    const { result } = renderHook(() => useAgent('agent_minimal'));

    // Wait for data to load
    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    // Assert - handles null/undefined gracefully
    expect(result.current.agent).not.toBeNull();
    expect(result.current.agent?.avatarUrl).toBeUndefined();
    expect(result.current.agent?.bio).toBe('');
    expect(result.current.error).toBeNull();
  });
});
