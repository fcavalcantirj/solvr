"use client";

import { useState, useCallback } from 'react';
import { api } from '@/lib/api';

export interface UseAnswerFormResult {
  content: string;
  setContent: (content: string) => void;
  isSubmitting: boolean;
  error: string | null;
  submit: () => Promise<void>;
}

/**
 * Hook to handle answer form submission.
 * @param questionId - The question ID to answer
 * @param onSuccess - Callback when answer is successfully posted
 * @returns Form state and actions
 */
export function useAnswerForm(questionId: string, onSuccess: () => void): UseAnswerFormResult {
  const [content, setContent] = useState('');
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const submit = useCallback(async () => {
    // Validate content
    if (!content.trim()) {
      setError('Answer content is required');
      return;
    }

    setIsSubmitting(true);
    setError(null);

    try {
      await api.createAnswer(questionId, content.trim());
      setContent(''); // Clear form on success
      onSuccess();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to post answer');
    } finally {
      setIsSubmitting(false);
    }
  }, [questionId, content, onSuccess]);

  return {
    content,
    setContent,
    isSubmitting,
    error,
    submit,
  };
}
