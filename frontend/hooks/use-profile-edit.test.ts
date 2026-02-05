import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { renderHook, act } from '@testing-library/react';
import { useProfileEdit } from './use-profile-edit';
import { api } from '@/lib/api';

vi.mock('@/lib/api', () => ({
  api: {
    updateProfile: vi.fn(),
  },
}));

describe('useProfileEdit', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('should initialize with default state', () => {
    const { result } = renderHook(() => useProfileEdit());

    expect(result.current.saving).toBe(false);
    expect(result.current.error).toBeNull();
    expect(result.current.success).toBe(false);
  });

  it('should update profile successfully', async () => {
    vi.mocked(api.updateProfile).mockResolvedValue({
      data: {
        id: 'user-123',
        type: 'human',
        display_name: 'New Name',
        email: 'test@example.com',
      },
    });

    const { result } = renderHook(() => useProfileEdit());

    let updateResult: boolean | undefined;
    await act(async () => {
      updateResult = await result.current.updateProfile({ display_name: 'New Name' });
    });

    expect(updateResult).toBe(true);
    expect(result.current.success).toBe(true);
    expect(result.current.error).toBeNull();
    expect(result.current.saving).toBe(false);
    expect(api.updateProfile).toHaveBeenCalledWith({ display_name: 'New Name' });
  });

  it('should update profile with bio', async () => {
    vi.mocked(api.updateProfile).mockResolvedValue({
      data: {
        id: 'user-123',
        type: 'human',
        display_name: 'Test User',
        email: 'test@example.com',
      },
    });

    const { result } = renderHook(() => useProfileEdit());

    await act(async () => {
      await result.current.updateProfile({ bio: 'New bio text' });
    });

    expect(api.updateProfile).toHaveBeenCalledWith({ bio: 'New bio text' });
    expect(result.current.success).toBe(true);
  });

  it('should update both display_name and bio', async () => {
    vi.mocked(api.updateProfile).mockResolvedValue({
      data: {
        id: 'user-123',
        type: 'human',
        display_name: 'Updated Name',
        email: 'test@example.com',
      },
    });

    const { result } = renderHook(() => useProfileEdit());

    await act(async () => {
      await result.current.updateProfile({
        display_name: 'Updated Name',
        bio: 'Updated bio'
      });
    });

    expect(api.updateProfile).toHaveBeenCalledWith({
      display_name: 'Updated Name',
      bio: 'Updated bio'
    });
    expect(result.current.success).toBe(true);
  });

  it('should handle API error with Error instance', async () => {
    vi.mocked(api.updateProfile).mockRejectedValue(new Error('Network error'));

    const { result } = renderHook(() => useProfileEdit());

    let updateResult: boolean | undefined;
    await act(async () => {
      updateResult = await result.current.updateProfile({ display_name: 'Name' });
    });

    expect(updateResult).toBe(false);
    expect(result.current.error).toBe('Network error');
    expect(result.current.success).toBe(false);
    expect(result.current.saving).toBe(false);
  });

  it('should handle non-Error API failure', async () => {
    vi.mocked(api.updateProfile).mockRejectedValue('Unknown error');

    const { result } = renderHook(() => useProfileEdit());

    await act(async () => {
      await result.current.updateProfile({ display_name: 'Name' });
    });

    expect(result.current.error).toBe('Failed to update profile');
    expect(result.current.success).toBe(false);
  });

  it('should set saving to true during update', async () => {
    let resolvePromise: (value: unknown) => void;
    const pendingPromise = new Promise((resolve) => {
      resolvePromise = resolve;
    });
    vi.mocked(api.updateProfile).mockReturnValue(pendingPromise as never);

    const { result } = renderHook(() => useProfileEdit());

    act(() => {
      result.current.updateProfile({ display_name: 'Name' });
    });

    expect(result.current.saving).toBe(true);

    await act(async () => {
      resolvePromise!({
        data: {
          id: 'user-123',
          type: 'human',
          display_name: 'Name',
          email: 'test@example.com',
        },
      });
    });

    expect(result.current.saving).toBe(false);
  });

  it('should clear status with clearStatus', async () => {
    vi.mocked(api.updateProfile).mockResolvedValue({
      data: {
        id: 'user-123',
        type: 'human',
        display_name: 'Name',
        email: 'test@example.com',
      },
    });

    const { result } = renderHook(() => useProfileEdit());

    // First, create success state
    await act(async () => {
      await result.current.updateProfile({ display_name: 'Name' });
    });

    expect(result.current.success).toBe(true);

    // Then clear it
    act(() => {
      result.current.clearStatus();
    });

    expect(result.current.success).toBe(false);
    expect(result.current.error).toBeNull();
  });

  it('should clear error with clearStatus', async () => {
    vi.mocked(api.updateProfile).mockRejectedValue(new Error('Some error'));

    const { result } = renderHook(() => useProfileEdit());

    // First, create error state
    await act(async () => {
      await result.current.updateProfile({ display_name: 'Name' });
    });

    expect(result.current.error).toBe('Some error');

    // Then clear it
    act(() => {
      result.current.clearStatus();
    });

    expect(result.current.error).toBeNull();
    expect(result.current.success).toBe(false);
  });

  it('should reset error and success on new update attempt', async () => {
    // First call fails
    vi.mocked(api.updateProfile).mockRejectedValueOnce(new Error('First error'));

    const { result } = renderHook(() => useProfileEdit());

    await act(async () => {
      await result.current.updateProfile({ display_name: 'Name' });
    });

    expect(result.current.error).toBe('First error');

    // Second call succeeds
    vi.mocked(api.updateProfile).mockResolvedValueOnce({
      data: {
        id: 'user-123',
        type: 'human',
        display_name: 'Name',
        email: 'test@example.com',
      },
    });

    await act(async () => {
      await result.current.updateProfile({ display_name: 'Name' });
    });

    expect(result.current.error).toBeNull();
    expect(result.current.success).toBe(true);
  });
});
