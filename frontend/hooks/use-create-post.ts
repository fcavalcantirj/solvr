import { useState, useCallback } from 'react';
import { api, APICreatePostResponse } from '@/lib/api';

export interface CreatePostForm {
  type: 'problem' | 'question' | 'idea';
  title: string;
  description: string;
  tags: string[];
}

interface UseCreatePostReturn {
  form: CreatePostForm;
  updateForm: (updates: Partial<CreatePostForm>) => void;
  isSubmitting: boolean;
  error: string | null;
  submit: () => Promise<APICreatePostResponse['data'] | null>;
}

export function useCreatePost(): UseCreatePostReturn {
  const [form, setForm] = useState<CreatePostForm>({
    type: 'question',
    title: '',
    description: '',
    tags: [],
  });
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const updateForm = useCallback((updates: Partial<CreatePostForm>) => {
    setForm((prev) => ({ ...prev, ...updates }));
  }, []);

  const submit = useCallback(async (): Promise<APICreatePostResponse['data'] | null> => {
    setError(null);

    // Validate title
    if (!form.title) {
      setError('Title is required');
      return null;
    }
    if (form.title.length < 10) {
      setError('Title must be at least 10 characters');
      return null;
    }

    // Validate description
    if (!form.description) {
      setError('Description is required');
      return null;
    }
    if (form.description.length < 50) {
      setError('Description must be at least 50 characters');
      return null;
    }

    setIsSubmitting(true);
    try {
      const response = await api.createPost({
        type: form.type,
        title: form.title,
        description: form.description,
        tags: form.tags,
      });
      return response.data;
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Failed to create post';
      setError(message);
      return null;
    } finally {
      setIsSubmitting(false);
    }
  }, [form]);

  return {
    form,
    updateForm,
    isSubmitting,
    error,
    submit,
  };
}
