import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderHook, act, waitFor } from '@testing-library/react';
import { useProgressNoteForm } from './use-progress-note-form';
import { api } from '@/lib/api';

// Mock the API
vi.mock('@/lib/api', () => ({
  api: {
    addProgressNote: vi.fn(),
  },
}));

describe('useProgressNoteForm', () => {
  const mockOnSuccess = vi.fn();
  const approachId = 'approach-123';

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('initializes with empty content and no error', () => {
    const { result } = renderHook(() => useProgressNoteForm(approachId, mockOnSuccess));

    expect(result.current.content).toBe('');
    expect(result.current.error).toBeNull();
    expect(result.current.isSubmitting).toBe(false);
  });

  it('updates content when setContent is called', () => {
    const { result } = renderHook(() => useProgressNoteForm(approachId, mockOnSuccess));

    act(() => {
      result.current.setContent('Test progress note');
    });

    expect(result.current.content).toBe('Test progress note');
  });

  it('shows error when submitting empty content', async () => {
    const { result } = renderHook(() => useProgressNoteForm(approachId, mockOnSuccess));

    await act(async () => {
      await result.current.submit();
    });

    expect(result.current.error).toBe('Content is required');
    expect(api.addProgressNote).not.toHaveBeenCalled();
    expect(mockOnSuccess).not.toHaveBeenCalled();
  });

  it('shows error when submitting whitespace-only content', async () => {
    const { result } = renderHook(() => useProgressNoteForm(approachId, mockOnSuccess));

    act(() => {
      result.current.setContent('   ');
    });

    await act(async () => {
      await result.current.submit();
    });

    expect(result.current.error).toBe('Content is required');
    expect(api.addProgressNote).not.toHaveBeenCalled();
  });

  it('calls api.addProgressNote with correct params on submit', async () => {
    (api.addProgressNote as ReturnType<typeof vi.fn>).mockResolvedValue({
      data: { id: 'note-1', approach_id: approachId, content: 'Test note', created_at: '2025-01-20T12:00:00Z' },
    });

    const { result } = renderHook(() => useProgressNoteForm(approachId, mockOnSuccess));

    act(() => {
      result.current.setContent('Test progress note');
    });

    await act(async () => {
      await result.current.submit();
    });

    expect(api.addProgressNote).toHaveBeenCalledWith(approachId, 'Test progress note');
  });

  it('trims content before submitting', async () => {
    (api.addProgressNote as ReturnType<typeof vi.fn>).mockResolvedValue({
      data: { id: 'note-1', approach_id: approachId, content: 'Test note', created_at: '2025-01-20T12:00:00Z' },
    });

    const { result } = renderHook(() => useProgressNoteForm(approachId, mockOnSuccess));

    act(() => {
      result.current.setContent('  Test progress note  ');
    });

    await act(async () => {
      await result.current.submit();
    });

    expect(api.addProgressNote).toHaveBeenCalledWith(approachId, 'Test progress note');
  });

  it('sets isSubmitting true during submission', async () => {
    let resolvePromise: () => void;
    const promise = new Promise<void>((resolve) => {
      resolvePromise = resolve;
    });

    (api.addProgressNote as ReturnType<typeof vi.fn>).mockReturnValue(promise.then(() => ({
      data: { id: 'note-1', approach_id: approachId, content: 'Test', created_at: '2025-01-20T12:00:00Z' },
    })));

    const { result } = renderHook(() => useProgressNoteForm(approachId, mockOnSuccess));

    act(() => {
      result.current.setContent('Test note');
    });

    // Start submit but don't await
    act(() => {
      result.current.submit();
    });

    // Should be submitting
    expect(result.current.isSubmitting).toBe(true);

    // Resolve the promise
    await act(async () => {
      resolvePromise!();
      await promise;
    });

    // Should no longer be submitting
    await waitFor(() => {
      expect(result.current.isSubmitting).toBe(false);
    });
  });

  it('calls onSuccess and resets form after successful submission', async () => {
    (api.addProgressNote as ReturnType<typeof vi.fn>).mockResolvedValue({
      data: { id: 'note-1', approach_id: approachId, content: 'Test note', created_at: '2025-01-20T12:00:00Z' },
    });

    const { result } = renderHook(() => useProgressNoteForm(approachId, mockOnSuccess));

    act(() => {
      result.current.setContent('Test progress note');
    });

    await act(async () => {
      await result.current.submit();
    });

    expect(mockOnSuccess).toHaveBeenCalled();
    expect(result.current.content).toBe('');
    expect(result.current.error).toBeNull();
  });

  it('sets error message when API fails', async () => {
    (api.addProgressNote as ReturnType<typeof vi.fn>).mockRejectedValue(new Error('Network error'));

    const { result } = renderHook(() => useProgressNoteForm(approachId, mockOnSuccess));

    act(() => {
      result.current.setContent('Test progress note');
    });

    await act(async () => {
      await result.current.submit();
    });

    expect(result.current.error).toBe('Network error');
    expect(mockOnSuccess).not.toHaveBeenCalled();
    // Content should not be cleared on error
    expect(result.current.content).toBe('Test progress note');
  });

  it('sets generic error message when API throws non-Error', async () => {
    (api.addProgressNote as ReturnType<typeof vi.fn>).mockRejectedValue('Unknown error');

    const { result } = renderHook(() => useProgressNoteForm(approachId, mockOnSuccess));

    act(() => {
      result.current.setContent('Test progress note');
    });

    await act(async () => {
      await result.current.submit();
    });

    expect(result.current.error).toBe('Failed to add progress note');
  });

  it('resets content and error when reset is called', () => {
    const { result } = renderHook(() => useProgressNoteForm(approachId, mockOnSuccess));

    // Set some state
    act(() => {
      result.current.setContent('Test content');
    });

    // Reset
    act(() => {
      result.current.reset();
    });

    expect(result.current.content).toBe('');
    expect(result.current.error).toBeNull();
  });

  it('clears previous error on new submit attempt', async () => {
    // First, create an error
    const { result } = renderHook(() => useProgressNoteForm(approachId, mockOnSuccess));

    await act(async () => {
      await result.current.submit(); // Empty content = error
    });

    expect(result.current.error).toBe('Content is required');

    // Now set content and try again
    (api.addProgressNote as ReturnType<typeof vi.fn>).mockResolvedValue({
      data: { id: 'note-1', approach_id: approachId, content: 'Test', created_at: '2025-01-20T12:00:00Z' },
    });

    act(() => {
      result.current.setContent('Valid content');
    });

    await act(async () => {
      await result.current.submit();
    });

    expect(result.current.error).toBeNull();
  });
});
