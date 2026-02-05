"use client";

import { useState, useCallback } from 'react';
import { api } from '@/lib/api';

export interface UseResponseFormResult {
  content: string;
  setContent: (content: string) => void;
  isSubmitting: boolean;
  error: string | null;
  submit: () => Promise<void>;
}

/**
 * Hook to handle response form submission for ideas.
 * @param ideaId - The idea ID to add response to
 * @param onSuccess - Callback when response is successfully posted
 * @returns Form state and actions
 */
export function useResponseForm(ideaId: string, onSuccess: () => void): UseResponseFormResult {
  const [content, setContent] = useState('');
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const submit = useCallback(async () => {
    // Validate required field
    if (!content.trim()) {
      setError('Response content is required');
      return;
    }

    setIsSubmitting(true);
    setError(null);

    try {
      await api.createResponse(ideaId, content.trim());
      // Clear form on success
      setContent('');
      onSuccess();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to post response');
    } finally {
      setIsSubmitting(false);
    }
  }, [ideaId, content, onSuccess]);

  return {
    content,
    setContent,
    isSubmitting,
    error,
    submit,
  };
}
