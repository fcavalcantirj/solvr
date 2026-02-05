import { useState, useCallback } from 'react';
import { api } from '@/lib/api';

interface UseBookmarksReturn {
  bookmarkedPosts: Set<string>;
  isLoading: boolean;
  error: string | null;
  toggleBookmark: (postId: string, isCurrentlyBookmarked?: boolean) => Promise<void>;
  checkBookmarked: (postId: string) => Promise<boolean>;
}

export function useBookmarks(): UseBookmarksReturn {
  const [bookmarkedPosts, setBookmarkedPosts] = useState<Set<string>>(new Set());
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const checkBookmarked = useCallback(async (postId: string): Promise<boolean> => {
    try {
      const response = await api.isBookmarked(postId);
      const isBookmarked = response.data.bookmarked;

      if (isBookmarked) {
        setBookmarkedPosts((prev) => new Set(prev).add(postId));
      }

      return isBookmarked;
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Failed to check bookmark';
      setError(message);
      return false;
    }
  }, []);

  const toggleBookmark = useCallback(async (postId: string, isCurrentlyBookmarked?: boolean): Promise<void> => {
    setError(null);
    setIsLoading(true);

    try {
      // If we know the current state, use it; otherwise check the local state
      const shouldRemove = isCurrentlyBookmarked ?? bookmarkedPosts.has(postId);

      if (shouldRemove) {
        await api.removeBookmark(postId);
        setBookmarkedPosts((prev) => {
          const next = new Set(prev);
          next.delete(postId);
          return next;
        });
      } else {
        await api.addBookmark(postId);
        setBookmarkedPosts((prev) => new Set(prev).add(postId));
      }
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Failed to toggle bookmark';
      setError(message);
    } finally {
      setIsLoading(false);
    }
  }, [bookmarkedPosts]);

  return {
    bookmarkedPosts,
    isLoading,
    error,
    toggleBookmark,
    checkBookmarked,
  };
}
