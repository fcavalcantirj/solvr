import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { renderHook, act, waitFor } from '@testing-library/react';
import { useAPIKeys } from './use-api-keys';
import { api } from '@/lib/api';

vi.mock('@/lib/api', () => ({
  api: {
    listAPIKeys: vi.fn(),
    createAPIKey: vi.fn(),
    revokeAPIKey: vi.fn(),
    regenerateAPIKey: vi.fn(),
  },
}));

describe('useAPIKeys', () => {
  const mockKeys = [
    {
      id: 'key_1',
      name: 'Production',
      key_preview: 'solvr_sk_****7f2a',
      last_used_at: '2026-02-05T10:00:00Z',
      created_at: '2026-02-01T10:00:00Z',
    },
    {
      id: 'key_2',
      name: 'Development',
      key_preview: 'solvr_sk_****3b1c',
      last_used_at: null,
      created_at: '2026-02-03T10:00:00Z',
    },
  ];

  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('should initialize with loading state', () => {
    vi.mocked(api.listAPIKeys).mockImplementation(() => new Promise(() => {})); // Never resolves

    const { result } = renderHook(() => useAPIKeys());

    expect(result.current.loading).toBe(true);
    expect(result.current.keys).toEqual([]);
    expect(result.current.error).toBeNull();
  });

  it('should load API keys on mount', async () => {
    vi.mocked(api.listAPIKeys).mockResolvedValue({
      data: mockKeys,
      meta: { total: 2 },
    });

    const { result } = renderHook(() => useAPIKeys());

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    expect(result.current.keys).toEqual(mockKeys);
    expect(result.current.total).toBe(2);
    expect(result.current.error).toBeNull();
  });

  it('should handle fetch error', async () => {
    vi.mocked(api.listAPIKeys).mockRejectedValue(new Error('Network error'));

    const { result } = renderHook(() => useAPIKeys());

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    expect(result.current.keys).toEqual([]);
    expect(result.current.error).toBe('Network error');
  });

  it('should handle non-Error rejection', async () => {
    vi.mocked(api.listAPIKeys).mockRejectedValue('Unknown error');

    const { result } = renderHook(() => useAPIKeys());

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    expect(result.current.error).toBe('Failed to fetch API keys');
  });

  it('should create a new API key', async () => {
    vi.mocked(api.listAPIKeys).mockResolvedValue({
      data: mockKeys,
      meta: { total: 2 },
    });

    const createResponse = {
      data: {
        id: 'key_3',
        name: 'New Key',
        key: 'solvr_sk_full_secret_key_here',
        created_at: '2026-02-05T12:00:00Z',
      },
    };

    vi.mocked(api.createAPIKey).mockResolvedValue(createResponse);

    const { result } = renderHook(() => useAPIKeys());

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    let response;
    await act(async () => {
      response = await result.current.createKey('New Key');
    });

    expect(response).toEqual(createResponse);
    expect(api.createAPIKey).toHaveBeenCalledWith('New Key');
    // Should refetch after creating
    expect(api.listAPIKeys).toHaveBeenCalledTimes(2);
  });

  it('should revoke an API key', async () => {
    vi.mocked(api.listAPIKeys).mockResolvedValue({
      data: mockKeys,
      meta: { total: 2 },
    });

    vi.mocked(api.revokeAPIKey).mockResolvedValue(undefined);

    const { result } = renderHook(() => useAPIKeys());

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    await act(async () => {
      await result.current.revokeKey('key_1');
    });

    expect(api.revokeAPIKey).toHaveBeenCalledWith('key_1');
    // Should refetch after revoking
    expect(api.listAPIKeys).toHaveBeenCalledTimes(2);
  });

  it('should regenerate an API key', async () => {
    vi.mocked(api.listAPIKeys).mockResolvedValue({
      data: mockKeys,
      meta: { total: 2 },
    });

    const regenerateResponse = {
      data: {
        id: 'key_1',
        name: 'Production',
        key: 'solvr_sk_new_regenerated_key',
        created_at: '2026-02-05T12:00:00Z',
      },
    };

    vi.mocked(api.regenerateAPIKey).mockResolvedValue(regenerateResponse);

    const { result } = renderHook(() => useAPIKeys());

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    let response;
    await act(async () => {
      response = await result.current.regenerateKey('key_1');
    });

    expect(response).toEqual(regenerateResponse);
    expect(api.regenerateAPIKey).toHaveBeenCalledWith('key_1');
    // Should refetch after regenerating
    expect(api.listAPIKeys).toHaveBeenCalledTimes(2);
  });

  it('should refetch keys on demand', async () => {
    vi.mocked(api.listAPIKeys).mockResolvedValue({
      data: mockKeys,
      meta: { total: 2 },
    });

    const { result } = renderHook(() => useAPIKeys());

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    expect(api.listAPIKeys).toHaveBeenCalledTimes(1);

    act(() => {
      result.current.refetch();
    });

    await waitFor(() => {
      expect(api.listAPIKeys).toHaveBeenCalledTimes(2);
    });
  });

  it('should handle response without meta field', async () => {
    // API might return response without meta field
    vi.mocked(api.listAPIKeys).mockResolvedValue({
      data: mockKeys,
      // No meta field
    } as never);

    const { result } = renderHook(() => useAPIKeys());

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    // Should not crash, keys should be loaded
    expect(result.current.keys).toEqual(mockKeys);
    expect(result.current.total).toBe(0); // Default to 0
    expect(result.current.error).toBeNull();
  });

  it('should handle response with empty data array', async () => {
    vi.mocked(api.listAPIKeys).mockResolvedValue({
      data: [],
      meta: { total: 0 },
    });

    const { result } = renderHook(() => useAPIKeys());

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    expect(result.current.keys).toEqual([]);
    expect(result.current.total).toBe(0);
    expect(result.current.error).toBeNull();
  });
});
