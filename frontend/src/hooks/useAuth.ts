'use client';

/**
 * useAuth Hook
 * Per PRD requirements:
 *   - Return {user, isLoading, login, logout}
 *   - Fetch /v1/auth/me on mount
 *   - Cache user in state
 */

import { useState, useEffect, useCallback } from 'react';
import { useRouter } from 'next/navigation';

const AUTH_TOKEN_KEY = 'solvr_auth_token';

/**
 * User type matching SPEC.md Part 2.8
 */
export interface User {
  id: string;
  username: string;
  display_name: string;
  email?: string;
  avatar_url?: string;
  bio?: string;
  created_at?: string;
}

/**
 * Get API base URL from environment
 */
function getApiBaseUrl(): string {
  return process.env.NEXT_PUBLIC_API_URL || '/api';
}

/**
 * Get auth token from localStorage
 */
function getAuthToken(): string | null {
  if (typeof window === 'undefined') return null;
  return localStorage.getItem(AUTH_TOKEN_KEY);
}

/**
 * Clear auth token from localStorage
 */
function clearAuthToken(): void {
  if (typeof window === 'undefined') return;
  localStorage.removeItem(AUTH_TOKEN_KEY);
}

type OAuthProvider = 'github' | 'google';

export interface UseAuthReturn {
  user: User | null;
  isLoading: boolean;
  login: (provider: OAuthProvider) => void;
  logout: () => Promise<void>;
}

/**
 * useAuth Hook
 * Manages authentication state for the application
 */
export function useAuth(): UseAuthReturn {
  const router = useRouter();
  const [user, setUser] = useState<User | null>(null);
  const [isLoading, setIsLoading] = useState<boolean>(() => {
    // Only start loading if we have a token
    if (typeof window === 'undefined') return false;
    return !!localStorage.getItem(AUTH_TOKEN_KEY);
  });

  /**
   * Fetch current user from API
   */
  useEffect(() => {
    const token = getAuthToken();

    // No token, no user to fetch
    if (!token) {
      setUser(null);
      setIsLoading(false);
      return;
    }

    const fetchUser = async () => {
      try {
        const response = await fetch(`${getApiBaseUrl()}/v1/auth/me`, {
          headers: {
            Authorization: `Bearer ${token}`,
            'Content-Type': 'application/json',
          },
        });

        if (!response.ok) {
          // Token is invalid or expired
          if (response.status === 401) {
            clearAuthToken();
            setUser(null);
          }
          setIsLoading(false);
          return;
        }

        const data = await response.json();
        setUser(data.data);
      } catch (error) {
        console.error('Failed to fetch user:', error);
        // On network error, keep token but set user to null
        setUser(null);
      } finally {
        setIsLoading(false);
      }
    };

    fetchUser();
  }, []);

  /**
   * Redirect to OAuth provider for login
   */
  const login = useCallback((provider: OAuthProvider) => {
    const baseUrl = getApiBaseUrl();
    window.location.href = `${baseUrl}/v1/auth/${provider}`;
  }, []);

  /**
   * Logout - clear token and redirect to home
   */
  const logout = useCallback(async () => {
    const token = getAuthToken();

    // Call logout endpoint if we have a token
    if (token) {
      try {
        await fetch(`${getApiBaseUrl()}/v1/auth/logout`, {
          method: 'POST',
          headers: {
            Authorization: `Bearer ${token}`,
            'Content-Type': 'application/json',
          },
        });
      } catch (error) {
        // Ignore errors - we're logging out anyway
        console.error('Logout API call failed:', error);
      }
    }

    // Clear local state and storage
    clearAuthToken();
    setUser(null);

    // Redirect to home
    router.push('/');
  }, [router]);

  return {
    user,
    isLoading,
    login,
    logout,
  };
}

export default useAuth;
