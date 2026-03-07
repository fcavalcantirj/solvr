"use client";

import { useState, useCallback } from 'react';
import { api } from '@/lib/api';

export interface UseVerifyApproachResult {
  isVerifying: boolean;
  error: string | null;
  verify: (approachId: string) => Promise<void>;
}

export function useVerifyApproach(onSuccess: () => void): UseVerifyApproachResult {
  const [isVerifying, setIsVerifying] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const verify = useCallback(async (approachId: string) => {
    setIsVerifying(true);
    setError(null);

    try {
      await api.verifyApproach(approachId);
      onSuccess();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to verify approach');
    } finally {
      setIsVerifying(false);
    }
  }, [onSuccess]);

  return { isVerifying, error, verify };
}
