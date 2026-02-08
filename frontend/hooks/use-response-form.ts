"use client";

import { useState, useCallback } from 'react';
import { api } from '@/lib/api';
import type { IdeaResponseType } from '@/lib/api-types';

export interface UseResponseFormResult {
  content: string;
  setContent: (content: string) => void;
  responseType: IdeaResponseType;
  setResponseType: (type: IdeaResponseType) => void;
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
  const [responseType, setResponseType] = useState<IdeaResponseType>('support');
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
      await api.createIdeaResponse(ideaId, content.trim(), responseType);
      // Clear form on success
      setContent('');
      setResponseType('support');
      onSuccess();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to post response');
    } finally {
      setIsSubmitting(false);
    }
  }, [ideaId, content, responseType, onSuccess]);

  return {
    content,
    setContent,
    responseType,
    setResponseType,
    isSubmitting,
    error,
    submit,
  };
}
