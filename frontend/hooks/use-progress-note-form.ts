import { useState, useCallback } from 'react';
import { api } from '@/lib/api';

export interface UseProgressNoteFormResult {
  content: string;
  setContent: (content: string) => void;
  isSubmitting: boolean;
  error: string | null;
  submit: () => Promise<void>;
  reset: () => void;
}

export function useProgressNoteForm(approachId: string, onSuccess: () => void): UseProgressNoteFormResult {
  const [content, setContent] = useState('');
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const reset = useCallback(() => {
    setContent('');
    setError(null);
  }, []);

  const submit = useCallback(async () => {
    if (!content.trim()) {
      setError('Content is required');
      return;
    }

    setIsSubmitting(true);
    setError(null);

    try {
      await api.addProgressNote(approachId, content.trim());
      reset();
      onSuccess();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to add progress note');
    } finally {
      setIsSubmitting(false);
    }
  }, [approachId, content, onSuccess, reset]);

  return { content, setContent, isSubmitting, error, submit, reset };
}
