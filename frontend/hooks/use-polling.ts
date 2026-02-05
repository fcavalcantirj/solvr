import { useState, useEffect, useRef, useCallback } from 'react';

interface UsePollingOptions {
  enabled?: boolean;
}

interface UsePollingReturn {
  isPolling: boolean;
  error: string | null;
}

export function usePolling(
  callback: () => Promise<void>,
  intervalMs: number,
  options: UsePollingOptions = {}
): UsePollingReturn {
  const { enabled = true } = options;
  const [isPolling, setIsPolling] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const callbackRef = useRef(callback);

  // Update callback ref when callback changes
  useEffect(() => {
    callbackRef.current = callback;
  }, [callback]);

  const executeCallback = useCallback(async () => {
    setIsPolling(true);
    setError(null);
    try {
      await callbackRef.current();
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Polling error';
      setError(message);
    } finally {
      setIsPolling(false);
    }
  }, []);

  useEffect(() => {
    if (!enabled) return;

    // Call immediately on mount
    executeCallback();

    // Skip interval if intervalMs is 0 or negative
    if (intervalMs <= 0) return;

    const intervalId = setInterval(() => {
      executeCallback();
    }, intervalMs);

    return () => {
      clearInterval(intervalId);
    };
  }, [enabled, intervalMs, executeCallback]);

  return {
    isPolling,
    error,
  };
}
