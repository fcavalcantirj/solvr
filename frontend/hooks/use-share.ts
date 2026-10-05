"use client";

import { useState, useCallback, useEffect } from 'react';

export interface UseShareResult {
  isSharing: boolean;
  shared: boolean;
  error: string | null;
  // Resolves true once the link was handed to the share sheet or the clipboard, and
  // false when the visitor dismissed the sheet or the browser refused. It never rejects.
  share: (title: string, url: string) => Promise<boolean>;
}

/**
 * Hook to handle sharing a URL via Web Share API or clipboard.
 * @returns Share state and share function
 */
export function useShare(): UseShareResult {
  const [isSharing, setIsSharing] = useState(false);
  const [shared, setShared] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Reset shared state after 2 seconds
  useEffect(() => {
    if (shared) {
      const timer = setTimeout(() => {
        setShared(false);
      }, 2000);
      return () => clearTimeout(timer);
    }
  }, [shared]);

  const share = useCallback(async (title: string, url: string): Promise<boolean> => {
    setIsSharing(true);
    setError(null);
    setShared(false);

    try {
      // Try Web Share API first if available
      if (navigator.share) {
        await navigator.share({ title, url });
        setShared(true);
      } else if (navigator.clipboard) {
        // Fall back to clipboard
        await navigator.clipboard.writeText(url);
        setShared(true);
      } else {
        throw new Error('Sharing not supported');
      }
      return true;
    } catch (err) {
      // Don't treat user cancellation as an error
      if (err instanceof Error && err.name === 'AbortError') {
        return false;
      }
      setError(err instanceof Error ? err.message : 'Failed to share');
      return false;
    } finally {
      setIsSharing(false);
    }
  }, []);

  return {
    isSharing,
    shared,
    error,
    share,
  };
}
