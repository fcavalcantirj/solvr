'use client';

import { useState, useCallback } from 'react';
import { api, ReportReason, ReportTargetType } from '@/lib/api';
import { isUnauthorizedError } from '@/lib/api-error';

interface UseReportOptions {
  onSuccess?: () => void;
  onError?: (error: string) => void;
}

interface UseReportReturn {
  isSubmitting: boolean;
  isReported: boolean;
  error: string | null;
  submitReport: (targetType: ReportTargetType, targetId: string, reason: ReportReason, details?: string) => Promise<boolean>;
  checkReported: (targetType: ReportTargetType, targetId: string) => Promise<boolean>;
  clearError: () => void;
}

export function useReport(options: UseReportOptions = {}): UseReportReturn {
  const { onSuccess, onError } = options;

  const [isSubmitting, setIsSubmitting] = useState(false);
  const [isReported, setIsReported] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const submitReport = useCallback(
    async (
      targetType: ReportTargetType,
      targetId: string,
      reason: ReportReason,
      details?: string
    ): Promise<boolean> => {
      setIsSubmitting(true);
      setError(null);

      try {
        await api.createReport({
          target_type: targetType,
          target_id: targetId,
          reason,
          details,
        });
        setIsReported(true);
        onSuccess?.();
        return true;
      } catch (err) {
        if (isUnauthorizedError(err)) {
          const msg = 'Sign in to report content';
          setError(msg);
          onError?.(msg);
          return false;
        }
        const errorMessage = err instanceof Error ? err.message : 'Failed to submit report';
        setError(errorMessage);
        onError?.(errorMessage);
        return false;
      } finally {
        setIsSubmitting(false);
      }
    },
    [onSuccess, onError]
  );

  const checkReported = useCallback(
    async (targetType: ReportTargetType, targetId: string): Promise<boolean> => {
      try {
        const response = await api.checkReported(targetType, targetId);
        setIsReported(response.data.reported);
        return response.data.reported;
      } catch {
        // Silently fail check - not critical
        return false;
      }
    },
    []
  );

  const clearError = useCallback(() => {
    setError(null);
  }, []);

  return {
    isSubmitting,
    isReported,
    error,
    submitReport,
    checkReported,
    clearError,
  };
}

// Report reason labels for UI display
export const REPORT_REASONS: { value: ReportReason; label: string; description: string }[] = [
  { value: 'spam', label: 'Spam', description: 'Unsolicited advertising or promotional content' },
  { value: 'offensive', label: 'Offensive', description: 'Hateful, abusive, or inappropriate content' },
  { value: 'off_topic', label: 'Off Topic', description: 'Not relevant to the discussion' },
  { value: 'misleading', label: 'Misleading', description: 'Contains false or misleading information' },
  { value: 'other', label: 'Other', description: 'Other reason not listed above' },
];
