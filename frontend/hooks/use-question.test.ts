"use client";

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { renderHook, waitFor } from '@testing-library/react';
import { useQuestion } from './use-question';
import { api } from '@/lib/api';

// Mock the API module
vi.mock('@/lib/api', () => ({
  api: {
    getPost: vi.fn(),
    getQuestionAnswers: vi.fn(),
  },
  formatRelativeTime: (date: string) => 'just now',
  mapStatus: (status: string) => status.toUpperCase(),
}));

const mockQuestion = {
  id: 'question-123',
  type: 'question' as const,
  title: 'How do I handle async errors in Go?',
  description: 'I am trying to handle async errors in Go...',
  status: 'open',
  upvotes: 10,
  downvotes: 2,
  vote_score: 8,
  author: {
    id: 'user-1',
    type: 'human' as const,
    display_name: 'John Doe',
  },
  tags: ['go', 'async', 'errors'],
  created_at: '2025-01-15T10:00:00Z',
  updated_at: '2025-01-15T12:00:00Z',
  answers_count: 3,
};

const mockAnswers = [
  {
    id: 'answer-1',
    question_id: 'question-123',
    author_type: 'human' as const,
    author_id: 'user-2',
    content: 'You should use goroutines with channels...',
    is_accepted: true,
    upvotes: 15,
    downvotes: 1,
    vote_score: 14,
    created_at: '2025-01-15T11:00:00Z',
    author: {
      type: 'human' as const,
      id: 'user-2',
      display_name: 'Jane Smith',
    },
  },
  {
    id: 'answer-2',
    question_id: 'question-123',
    author_type: 'agent' as const,
    author_id: 'claude-1',
    content: 'Another approach is to use errgroup...',
    is_accepted: false,
    upvotes: 5,
    downvotes: 0,
    vote_score: 5,
    created_at: '2025-01-15T13:00:00Z',
    author: {
      type: 'agent' as const,
      id: 'claude-1',
      display_name: 'Claude Assistant',
    },
  },
];

describe('useQuestion', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('should fetch question data and answers', async () => {
    // Arrange
    (api.getPost as ReturnType<typeof vi.fn>).mockResolvedValue({ data: mockQuestion });
    (api.getQuestionAnswers as ReturnType<typeof vi.fn>).mockResolvedValue({
      data: mockAnswers,
      meta: { total: 2, page: 1, per_page: 20, has_more: false },
    });

    // Act
    const { result } = renderHook(() => useQuestion('question-123'));

    // Assert - initial loading state
    expect(result.current.loading).toBe(true);
    expect(result.current.question).toBeNull();
    expect(result.current.answers).toEqual([]);

    // Wait for data to load
    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    // Assert - question loaded
    expect(result.current.question).not.toBeNull();
    expect(result.current.question?.id).toBe('question-123');
    expect(result.current.question?.title).toBe('How do I handle async errors in Go?');
    expect(result.current.question?.voteScore).toBe(8);
    expect(result.current.question?.tags).toEqual(['go', 'async', 'errors']);

    // Assert - answers loaded
    expect(result.current.answers).toHaveLength(2);
    expect(result.current.answers[0].id).toBe('answer-1');
    expect(result.current.answers[0].isAccepted).toBe(true);
    expect(result.current.answers[1].id).toBe('answer-2');

    // Assert - no error
    expect(result.current.error).toBeNull();
  });

  it('should handle API errors gracefully', async () => {
    // Arrange
    (api.getPost as ReturnType<typeof vi.fn>).mockRejectedValue(new Error('Network error'));

    // Act
    const { result } = renderHook(() => useQuestion('question-123'));

    // Wait for error
    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    // Assert
    expect(result.current.error).toBe('Network error');
    expect(result.current.question).toBeNull();
  });

  it('should handle question not found', async () => {
    // Arrange
    (api.getPost as ReturnType<typeof vi.fn>).mockRejectedValue(new Error('API error: 404'));

    // Act
    const { result } = renderHook(() => useQuestion('nonexistent-id'));

    // Wait for error
    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    // Assert
    expect(result.current.error).toBe('API error: 404');
    expect(result.current.question).toBeNull();
  });

  it('should refetch data when refetch is called', async () => {
    // Arrange
    (api.getPost as ReturnType<typeof vi.fn>).mockResolvedValue({ data: mockQuestion });
    (api.getQuestionAnswers as ReturnType<typeof vi.fn>).mockResolvedValue({
      data: mockAnswers,
      meta: { total: 2, page: 1, per_page: 20, has_more: false },
    });

    // Act
    const { result } = renderHook(() => useQuestion('question-123'));

    // Wait for initial load
    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    // Clear mocks and change the data
    vi.clearAllMocks();
    const updatedQuestion = { ...mockQuestion, upvotes: 15, vote_score: 13 };
    (api.getPost as ReturnType<typeof vi.fn>).mockResolvedValue({ data: updatedQuestion });
    (api.getQuestionAnswers as ReturnType<typeof vi.fn>).mockResolvedValue({
      data: mockAnswers,
      meta: { total: 2, page: 1, per_page: 20, has_more: false },
    });

    // Refetch
    result.current.refetch();

    // Wait for refetch
    await waitFor(() => {
      expect(result.current.question?.voteScore).toBe(13);
    });

    // Assert
    expect(api.getPost).toHaveBeenCalledTimes(1);
    expect(api.getQuestionAnswers).toHaveBeenCalledTimes(1);
  });

  it('should not fetch when id is empty', async () => {
    // Act
    const { result } = renderHook(() => useQuestion(''));

    // Assert - should not be loading
    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    // No API calls made
    expect(api.getPost).not.toHaveBeenCalled();
    expect(api.getQuestionAnswers).not.toHaveBeenCalled();
  });

  // FE-021: Edge cases for defensive handling

  it('should handle question with null tags and missing optional fields', async () => {
    // Arrange - API returns null for tags and missing optional fields
    const questionWithNulls = {
      id: 'question-123',
      type: 'question' as const,
      title: 'Test question',
      description: 'Description',
      status: 'open',
      upvotes: 5,
      downvotes: 1,
      vote_score: 4,
      author: {
        id: 'user-1',
        type: 'human' as const,
        display_name: 'Test User',
      },
      tags: null, // null instead of array
      created_at: '2025-01-10T10:00:00Z',
      updated_at: '2025-01-10T10:00:00Z',
      // Missing: answers_count, view_count
    };

    (api.getPost as ReturnType<typeof vi.fn>).mockResolvedValue({ data: questionWithNulls });
    (api.getQuestionAnswers as ReturnType<typeof vi.fn>).mockResolvedValue({
      data: [],
      meta: { total: 0, page: 1, per_page: 20, has_more: false },
    });

    // Act
    const { result } = renderHook(() => useQuestion('question-123'));

    // Wait for data to load
    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    // Assert - should handle nulls gracefully
    expect(result.current.question).not.toBeNull();
    expect(result.current.question?.tags).toEqual([]);
    expect(result.current.question?.answersCount).toBe(0);
    expect(result.current.question?.views).toBe(0);
    expect(result.current.error).toBeNull();
  });

  it('should handle answers API returning undefined data', async () => {
    // Arrange - Malformed API response without data property
    (api.getPost as ReturnType<typeof vi.fn>).mockResolvedValue({ data: mockQuestion });
    (api.getQuestionAnswers as ReturnType<typeof vi.fn>).mockResolvedValue({
      // Missing data property
      meta: { total: 0, page: 1, per_page: 20, has_more: false },
    });

    // Act
    const { result } = renderHook(() => useQuestion('question-123'));

    // Wait for data to load
    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    // Assert - should return empty array when data is undefined
    expect(result.current.answers).toEqual([]);
    expect(result.current.error).toBeNull();
  });
});
