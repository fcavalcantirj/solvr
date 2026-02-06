"use client";

import { useState, useEffect, useCallback } from 'react';
import { api, APIKey, APIKeyCreateResponse } from '@/lib/api';

export interface UseAPIKeysResult {
  keys: APIKey[];
  loading: boolean;
  error: string | null;
  total: number;
  createKey: (name: string) => Promise<APIKeyCreateResponse>;
  revokeKey: (id: string) => Promise<void>;
  regenerateKey: (id: string) => Promise<APIKeyCreateResponse>;
  refetch: () => void;
}

export function useAPIKeys(): UseAPIKeysResult {
  const [keys, setKeys] = useState<APIKey[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [total, setTotal] = useState(0);

  const fetchKeys = useCallback(async () => {
    try {
      setLoading(true);
      setError(null);
      const response = await api.listAPIKeys();
      setKeys(response.data ?? []);
      setTotal(response.meta?.total ?? 0);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to fetch API keys');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchKeys();
  }, [fetchKeys]);

  const createKey = useCallback(async (name: string): Promise<APIKeyCreateResponse> => {
    const response = await api.createAPIKey(name);
    // Refetch to update the list
    await fetchKeys();
    return response;
  }, [fetchKeys]);

  const revokeKey = useCallback(async (id: string): Promise<void> => {
    await api.revokeAPIKey(id);
    // Refetch to update the list
    await fetchKeys();
  }, [fetchKeys]);

  const regenerateKey = useCallback(async (id: string): Promise<APIKeyCreateResponse> => {
    const response = await api.regenerateAPIKey(id);
    // Refetch to update the list
    await fetchKeys();
    return response;
  }, [fetchKeys]);

  const refetch = useCallback(() => {
    fetchKeys();
  }, [fetchKeys]);

  return {
    keys,
    loading,
    error,
    total,
    createKey,
    revokeKey,
    regenerateKey,
    refetch,
  };
}
