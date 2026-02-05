'use client';

import { useEffect, useState, useCallback, useRef } from 'react';
import { api } from '@/lib/api';

interface UseViewTrackingOptions {
  enabled?: boolean;
}

interface UseViewTrackingReturn {
  viewCount: number;
  isLoading: boolean;
  error: Error | null;
  recordView: () => Promise<void>;
}

// Session storage key prefix for tracking views
const VIEW_STORAGE_KEY = 'solvr_viewed_posts';

// Get or create a session ID for anonymous view tracking
function getSessionId(): string {
  if (typeof window === 'undefined') return '';

  let sessionId = sessionStorage.getItem('solvr_session_id');
  if (!sessionId) {
    sessionId = crypto.randomUUID();
    sessionStorage.setItem('solvr_session_id', sessionId);
  }
  return sessionId;
}

// Check if a post has been viewed this session
function hasViewedPost(postId: string): boolean {
  if (typeof window === 'undefined') return false;

  const viewedPosts = sessionStorage.getItem(VIEW_STORAGE_KEY);
  if (!viewedPosts) return false;

  try {
    const viewed = JSON.parse(viewedPosts) as string[];
    return viewed.includes(postId);
  } catch {
    return false;
  }
}

// Mark a post as viewed this session
function markPostAsViewed(postId: string): void {
  if (typeof window === 'undefined') return;

  const viewedPosts = sessionStorage.getItem(VIEW_STORAGE_KEY);
  let viewed: string[] = [];

  try {
    viewed = viewedPosts ? JSON.parse(viewedPosts) : [];
  } catch {
    viewed = [];
  }

  if (!viewed.includes(postId)) {
    viewed.push(postId);
    sessionStorage.setItem(VIEW_STORAGE_KEY, JSON.stringify(viewed));
  }
}

export function useViewTracking(
  postId: string,
  initialViewCount: number = 0,
  options: UseViewTrackingOptions = {}
): UseViewTrackingReturn {
  const { enabled = true } = options;

  const [viewCount, setViewCount] = useState(initialViewCount);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<Error | null>(null);
  const hasRecordedRef = useRef(false);

  const recordView = useCallback(async () => {
    if (!postId || hasRecordedRef.current) return;

    // Check if already viewed this session (client-side dedup)
    if (hasViewedPost(postId)) {
      hasRecordedRef.current = true;
      return;
    }

    setIsLoading(true);
    setError(null);

    try {
      const sessionId = getSessionId();
      const response = await api.recordView(postId, sessionId);
      setViewCount(response.data.view_count);
      markPostAsViewed(postId);
      hasRecordedRef.current = true;
    } catch (err) {
      // Don't show errors for view tracking - it's non-critical
      setError(err instanceof Error ? err : new Error('Failed to record view'));
    } finally {
      setIsLoading(false);
    }
  }, [postId]);

  // Auto-record view on mount if enabled
  useEffect(() => {
    if (enabled && postId && !hasRecordedRef.current) {
      recordView();
    }
  }, [enabled, postId, recordView]);

  return {
    viewCount,
    isLoading,
    error,
    recordView,
  };
}
