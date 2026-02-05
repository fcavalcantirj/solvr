import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { renderHook, act, waitFor } from '@testing-library/react';
import { useViewTracking } from './use-view-tracking';
import { api } from '@/lib/api';

vi.mock('@/lib/api', () => ({
  api: {
    recordView: vi.fn(),
    getViewCount: vi.fn(),
  },
}));

// Mock sessionStorage
const mockSessionStorage: Record<string, string> = {};
const sessionStorageMock = {
  getItem: vi.fn((key: string) => mockSessionStorage[key] || null),
  setItem: vi.fn((key: string, value: string) => {
    mockSessionStorage[key] = value;
  }),
  removeItem: vi.fn((key: string) => {
    delete mockSessionStorage[key];
  }),
  clear: vi.fn(() => {
    Object.keys(mockSessionStorage).forEach((key) => delete mockSessionStorage[key]);
  }),
};

Object.defineProperty(window, 'sessionStorage', {
  value: sessionStorageMock,
});

// Mock crypto.randomUUID
Object.defineProperty(window, 'crypto', {
  value: {
    randomUUID: () => 'mock-session-uuid',
  },
});

describe('useViewTracking', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    sessionStorageMock.clear();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('should initialize with initial view count', () => {
    const { result } = renderHook(() => useViewTracking('post-123', 42, { enabled: false }));

    expect(result.current.viewCount).toBe(42);
    expect(result.current.isLoading).toBe(false);
    expect(result.current.error).toBeNull();
  });

  it('should record view automatically when enabled', async () => {
    vi.mocked(api.recordView).mockResolvedValue({ data: { view_count: 100 } });

    const { result } = renderHook(() => useViewTracking('post-123', 0, { enabled: true }));

    await waitFor(() => {
      expect(api.recordView).toHaveBeenCalledWith('post-123', 'mock-session-uuid');
    });

    await waitFor(() => {
      expect(result.current.viewCount).toBe(100);
    });
  });

  it('should not record view when disabled', async () => {
    const { result } = renderHook(() => useViewTracking('post-123', 0, { enabled: false }));

    // Wait a bit to ensure nothing happens
    await new Promise((resolve) => setTimeout(resolve, 50));

    expect(api.recordView).not.toHaveBeenCalled();
    expect(result.current.viewCount).toBe(0);
  });

  it('should not record duplicate views in same session', async () => {
    vi.mocked(api.recordView).mockResolvedValue({ data: { view_count: 1 } });

    // Simulate the post was already viewed
    mockSessionStorage['solvr_viewed_posts'] = JSON.stringify(['post-123']);

    const { result } = renderHook(() => useViewTracking('post-123', 0, { enabled: true }));

    // Wait a bit
    await new Promise((resolve) => setTimeout(resolve, 50));

    expect(api.recordView).not.toHaveBeenCalled();
  });

  it('should handle API errors gracefully', async () => {
    vi.mocked(api.recordView).mockRejectedValue(new Error('Network error'));

    const { result } = renderHook(() => useViewTracking('post-123', 0, { enabled: true }));

    await waitFor(() => {
      expect(result.current.error).toBeInstanceOf(Error);
      expect(result.current.error?.message).toBe('Network error');
    });
  });

  it('should track loading state during recording', async () => {
    let resolvePromise: (value: unknown) => void;
    const promise = new Promise((resolve) => {
      resolvePromise = resolve;
    });

    vi.mocked(api.recordView).mockReturnValue(promise as ReturnType<typeof api.recordView>);

    const { result } = renderHook(() => useViewTracking('post-123', 0, { enabled: true }));

    await waitFor(() => {
      expect(result.current.isLoading).toBe(true);
    });

    await act(async () => {
      resolvePromise!({ data: { view_count: 5 } });
    });

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });
  });

  it('should mark post as viewed in session storage after recording', async () => {
    vi.mocked(api.recordView).mockResolvedValue({ data: { view_count: 1 } });

    renderHook(() => useViewTracking('post-123', 0, { enabled: true }));

    await waitFor(() => {
      expect(api.recordView).toHaveBeenCalled();
    });

    // Check that the post was marked as viewed
    expect(sessionStorageMock.setItem).toHaveBeenCalled();
    const viewedPosts = JSON.parse(mockSessionStorage['solvr_viewed_posts'] || '[]');
    expect(viewedPosts).toContain('post-123');
  });

  it('should create session ID if not present', async () => {
    vi.mocked(api.recordView).mockResolvedValue({ data: { view_count: 1 } });

    renderHook(() => useViewTracking('post-123', 0, { enabled: true }));

    await waitFor(() => {
      expect(api.recordView).toHaveBeenCalledWith('post-123', 'mock-session-uuid');
    });

    expect(sessionStorageMock.setItem).toHaveBeenCalledWith('solvr_session_id', 'mock-session-uuid');
  });

  it('should use existing session ID if present', async () => {
    mockSessionStorage['solvr_session_id'] = 'existing-session-id';
    vi.mocked(api.recordView).mockResolvedValue({ data: { view_count: 1 } });

    renderHook(() => useViewTracking('post-new', 0, { enabled: true }));

    await waitFor(() => {
      expect(api.recordView).toHaveBeenCalledWith('post-new', 'existing-session-id');
    });
  });

  it('should provide recordView function for manual recording', async () => {
    vi.mocked(api.recordView).mockResolvedValue({ data: { view_count: 10 } });

    const { result } = renderHook(() => useViewTracking('post-123', 0, { enabled: false }));

    await act(async () => {
      await result.current.recordView();
    });

    expect(api.recordView).toHaveBeenCalledWith('post-123', 'mock-session-uuid');
    expect(result.current.viewCount).toBe(10);
  });

  it('should not record view if postId is empty', async () => {
    const { result } = renderHook(() => useViewTracking('', 0, { enabled: true }));

    // Wait a bit
    await new Promise((resolve) => setTimeout(resolve, 50));

    expect(api.recordView).not.toHaveBeenCalled();
  });
});
