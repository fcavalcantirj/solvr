"use client";

import { useState, useCallback, useEffect } from 'react';

// How a link left the page: through the browser's share sheet, or onto the clipboard.
export type ShareMethod = 'share_sheet' | 'clipboard';

export interface UseShareResult {
  isSharing: boolean;
  shared: boolean;
  error: string | null;
  // Resolves with how the link left the page once the share sheet or the clipboard took
  // it, and false when the visitor dismissed the sheet or the browser refused. It never
  // rejects.
  share: (title: string, url: string) => Promise<ShareMethod | false>;
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

  const share = useCallback(async (title: string, url: string): Promise<ShareMethod | false> => {
    setIsSharing(true);
    setError(null);
    setShared(false);

    try {
      // Try Web Share API first if available
      if (navigator.share) {
        await navigator.share({ title, url });
        setShared(true);
        return 'share_sheet';
      }
      if (navigator.clipboard) {
        // Fall back to clipboard
        await navigator.clipboard.writeText(url);
        setShared(true);
        return 'clipboard';
      }
      throw new Error('Sharing not supported');
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
