"use client";

import { useState, useEffect, useCallback } from 'react';
import { api, APIAuthMethodResponse } from '@/lib/api';

export interface UseAuthMethodsResult {
  authMethods: APIAuthMethodResponse[];
  loading: boolean;
  error: string | null;
  refetch: () => void;
}

export function useAuthMethods(): UseAuthMethodsResult {
  const [authMethods, setAuthMethods] = useState<APIAuthMethodResponse[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const fetchAuthMethods = useCallback(async () => {
    try {
      setLoading(true);
      setError(null);
      const response = await api.getMyAuthMethods();
      setAuthMethods(response.data.auth_methods ?? []);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to fetch authentication methods');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchAuthMethods();
  }, [fetchAuthMethods]);

  const refetch = useCallback(() => {
    fetchAuthMethods();
  }, [fetchAuthMethods]);

  return {
    authMethods,
    loading,
    error,
    refetch,
  };
}
