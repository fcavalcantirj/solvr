"use client";

import { useState, useCallback } from 'react';
import { api } from '@/lib/api';

export type CommentTargetType = 'answer' | 'approach' | 'response' | 'post';

export interface UseCommentFormResult {
  content: string;
  setContent: (content: string) => void;
  isSubmitting: boolean;
  error: string | null;
  submit: () => Promise<void>;
}

/**
 * Hook to handle comment form submission.
 * @param targetType - The type of entity to comment on
 * @param targetId - The ID of the entity to comment on
 * @param onSuccess - Callback when comment is successfully posted
 * @returns Form state and actions
 */
export function useCommentForm(
  targetType: CommentTargetType,
  targetId: string,
  onSuccess: () => void
): UseCommentFormResult {
  const [content, setContent] = useState('');
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const submit = useCallback(async () => {
    // Validate required field
    if (!content.trim()) {
      setError('Comment content is required');
      return;
    }

    setIsSubmitting(true);
    setError(null);

    try {
      await api.createComment(targetType, targetId, content.trim());
      // Clear form on success
      setContent('');
      onSuccess();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to post comment');
    } finally {
      setIsSubmitting(false);
    }
  }, [targetType, targetId, content, onSuccess]);

  return {
    content,
    setContent,
    isSubmitting,
    error,
    submit,
  };
}
