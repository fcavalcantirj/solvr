import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { renderHook, act, waitFor } from '@testing-library/react';
import { useReport, REPORT_REASONS } from './use-report';
import { api } from '@/lib/api';

vi.mock('@/lib/api', () => ({
  api: {
    createReport: vi.fn(),
    checkReported: vi.fn(),
  },
}));

describe('useReport', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('should initialize with default state', () => {
    const { result } = renderHook(() => useReport());

    expect(result.current.isSubmitting).toBe(false);
    expect(result.current.isReported).toBe(false);
    expect(result.current.error).toBeNull();
  });

  it('should submit report successfully', async () => {
    vi.mocked(api.createReport).mockResolvedValue({
      data: {
        id: 'report-123',
        target_type: 'answer',
        target_id: 'answer-456',
        reason: 'spam',
        status: 'pending',
        created_at: '2026-02-05T00:00:00Z',
      },
    });

    const onSuccess = vi.fn();
    const { result } = renderHook(() => useReport({ onSuccess }));

    let success: boolean;
    await act(async () => {
      success = await result.current.submitReport('answer', 'answer-456', 'spam', 'This is spam content');
    });

    expect(success!).toBe(true);
    expect(api.createReport).toHaveBeenCalledWith({
      target_type: 'answer',
      target_id: 'answer-456',
      reason: 'spam',
      details: 'This is spam content',
    });
    expect(result.current.isReported).toBe(true);
    expect(result.current.error).toBeNull();
    expect(onSuccess).toHaveBeenCalled();
  });

  it('should handle submit report error', async () => {
    vi.mocked(api.createReport).mockRejectedValue(new Error('Already reported'));

    const onError = vi.fn();
    const { result } = renderHook(() => useReport({ onError }));

    let success: boolean;
    await act(async () => {
      success = await result.current.submitReport('answer', 'answer-456', 'spam');
    });

    expect(success!).toBe(false);
    expect(result.current.error).toBe('Already reported');
    expect(result.current.isReported).toBe(false);
    expect(onError).toHaveBeenCalledWith('Already reported');
  });

  it('should track loading state during submission', async () => {
    let resolvePromise: (value: unknown) => void;
    const promise = new Promise((resolve) => {
      resolvePromise = resolve;
    });

    vi.mocked(api.createReport).mockReturnValue(promise as ReturnType<typeof api.createReport>);

    const { result } = renderHook(() => useReport());

    act(() => {
      result.current.submitReport('post', 'post-123', 'offensive');
    });

    await waitFor(() => {
      expect(result.current.isSubmitting).toBe(true);
    });

    await act(async () => {
      resolvePromise!({ data: { id: 'report-1' } });
    });

    await waitFor(() => {
      expect(result.current.isSubmitting).toBe(false);
    });
  });

  it('should check if content is already reported', async () => {
    vi.mocked(api.checkReported).mockResolvedValue({ data: { reported: true } });

    const { result } = renderHook(() => useReport());

    let isReported: boolean;
    await act(async () => {
      isReported = await result.current.checkReported('answer', 'answer-456');
    });

    expect(isReported!).toBe(true);
    expect(result.current.isReported).toBe(true);
    expect(api.checkReported).toHaveBeenCalledWith('answer', 'answer-456');
  });

  it('should handle check reported error gracefully', async () => {
    vi.mocked(api.checkReported).mockRejectedValue(new Error('Network error'));

    const { result } = renderHook(() => useReport());

    let isReported: boolean;
    await act(async () => {
      isReported = await result.current.checkReported('answer', 'answer-456');
    });

    expect(isReported!).toBe(false);
    // Should not set error for check failures (non-critical)
    expect(result.current.error).toBeNull();
  });

  it('should clear error', async () => {
    vi.mocked(api.createReport).mockRejectedValue(new Error('Failed'));

    const { result } = renderHook(() => useReport());

    await act(async () => {
      await result.current.submitReport('post', 'post-123', 'spam');
    });

    expect(result.current.error).toBe('Failed');

    act(() => {
      result.current.clearError();
    });

    expect(result.current.error).toBeNull();
  });

  it('should submit report without details', async () => {
    vi.mocked(api.createReport).mockResolvedValue({
      data: {
        id: 'report-123',
        target_type: 'comment',
        target_id: 'comment-789',
        reason: 'off_topic',
        status: 'pending',
        created_at: '2026-02-05T00:00:00Z',
      },
    });

    const { result } = renderHook(() => useReport());

    await act(async () => {
      await result.current.submitReport('comment', 'comment-789', 'off_topic');
    });

    expect(api.createReport).toHaveBeenCalledWith({
      target_type: 'comment',
      target_id: 'comment-789',
      reason: 'off_topic',
      details: undefined,
    });
  });
});

describe('REPORT_REASONS', () => {
  it('should have all required reasons', () => {
    const reasons = REPORT_REASONS.map((r) => r.value);
    expect(reasons).toContain('spam');
    expect(reasons).toContain('offensive');
    expect(reasons).toContain('off_topic');
    expect(reasons).toContain('misleading');
    expect(reasons).toContain('other');
  });

  it('should have labels and descriptions for all reasons', () => {
    REPORT_REASONS.forEach((reason) => {
      expect(reason.label).toBeTruthy();
      expect(reason.description).toBeTruthy();
    });
  });
});
