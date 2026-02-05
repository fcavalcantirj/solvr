"use client";

import { useState, useCallback } from 'react';
import { api } from '@/lib/api';

export interface ProfileUpdateData {
  display_name?: string;
  bio?: string;
}

export interface UseProfileEditResult {
  saving: boolean;
  error: string | null;
  success: boolean;
  updateProfile: (data: ProfileUpdateData) => Promise<boolean>;
  clearStatus: () => void;
}

export function useProfileEdit(): UseProfileEditResult {
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState(false);

  const updateProfile = useCallback(async (data: ProfileUpdateData): Promise<boolean> => {
    try {
      setSaving(true);
      setError(null);
      setSuccess(false);

      await api.updateProfile(data);

      setSuccess(true);
      return true;
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to update profile');
      return false;
    } finally {
      setSaving(false);
    }
  }, []);

  const clearStatus = useCallback(() => {
    setError(null);
    setSuccess(false);
  }, []);

  return {
    saving,
    error,
    success,
    updateProfile,
    clearStatus,
  };
}
