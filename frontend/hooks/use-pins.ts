"use client";

import { useState, useEffect, useCallback } from 'react';
import { api } from '@/lib/api';
import type { APIPinResponse, PinStatus } from '@/lib/api-types';

export interface UsePinsOptions {
  status?: PinStatus;
}

export interface StorageInfo {
  used: number;
  quota: number;
  percentage: number;
}

export interface UsePinsResult {
  pins: APIPinResponse[];
  loading: boolean;
  error: string | null;
  totalCount: number;
  storage: StorageInfo | null;
  createPin: (cid: string, name?: string) => Promise<void>;
  deletePin: (requestID: string) => Promise<void>;
  refetch: () => void;
}

export function usePins(options?: UsePinsOptions): UsePinsResult {
  const [pins, setPins] = useState<APIPinResponse[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [totalCount, setTotalCount] = useState(0);
  const [storage, setStorage] = useState<StorageInfo | null>(null);

  const optionsKey = JSON.stringify(options);

  const fetchPins = useCallback(async () => {
    try {
      setLoading(true);
      setError(null);

      const stableOptions: UsePinsOptions | undefined = optionsKey ? JSON.parse(optionsKey) : undefined;

      const [pinsResponse, storageResponse] = await Promise.allSettled([
        api.listPins({
          status: stableOptions?.status,
          limit: 100,
        }),
        api.getStorageUsage(),
      ]);

      if (pinsResponse.status === 'fulfilled') {
        const data = pinsResponse.value;
        setPins(data.results || []);
        setTotalCount(data.count || 0);
      } else {
        throw pinsResponse.reason;
      }

      if (storageResponse.status === 'fulfilled') {
        setStorage(storageResponse.value.data);
      } else {
        setStorage(null);
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to fetch pins');
      setPins([]);
      setTotalCount(0);
    } finally {
      setLoading(false);
    }
  }, [optionsKey]);

  useEffect(() => {
    fetchPins();
  }, [fetchPins]);

  const createPin = useCallback(async (cid: string, name?: string) => {
    await api.createPin(cid, name);
    fetchPins();
  }, [fetchPins]);

  const deletePin = useCallback(async (requestID: string) => {
    await api.deletePin(requestID);
    fetchPins();
  }, [fetchPins]);

  return {
    pins,
    loading,
    error,
    totalCount,
    storage,
    createPin,
    deletePin,
    refetch: fetchPins,
  };
}
