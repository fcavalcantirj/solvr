"use client";

import { useState, useCallback } from 'react';
import { api } from '@/lib/api';

export interface UseApproachFormResult {
  angle: string;
  setAngle: (angle: string) => void;
  method: string;
  setMethod: (method: string) => void;
  assumptions: string[];
  setAssumptions: (assumptions: string[]) => void;
  isSubmitting: boolean;
  error: string | null;
  submit: () => Promise<void>;
}

/**
 * Hook to handle approach form submission.
 * @param problemId - The problem ID to add approach to
 * @param onSuccess - Callback when approach is successfully posted
 * @returns Form state and actions
 */
export function useApproachForm(problemId: string, onSuccess: () => void): UseApproachFormResult {
  const [angle, setAngle] = useState('');
  const [method, setMethod] = useState('');
  const [assumptions, setAssumptions] = useState<string[]>([]);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const submit = useCallback(async () => {
    // Validate required field
    if (!angle.trim()) {
      setError('Approach angle is required');
      return;
    }

    setIsSubmitting(true);
    setError(null);

    try {
      await api.createApproach(problemId, {
        angle: angle.trim(),
        method: method.trim() || undefined,
        assumptions: assumptions.filter(a => a.trim()),
      });
      // Clear form on success
      setAngle('');
      setMethod('');
      setAssumptions([]);
      onSuccess();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to post approach');
    } finally {
      setIsSubmitting(false);
    }
  }, [problemId, angle, method, assumptions, onSuccess]);

  return {
    angle,
    setAngle,
    method,
    setMethod,
    assumptions,
    setAssumptions,
    isSubmitting,
    error,
    submit,
  };
}
